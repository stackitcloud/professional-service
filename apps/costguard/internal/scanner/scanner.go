// Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package scanner implements the read-only scan phase. Scanners
// never call delete functions — deletion is a separate step orchestrated
// by internal/deleter. Non-fatal scanner
// errors are logged, counted and surfaced in the report; they never abort
// the run.
package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// DefaultListCallPacing is the minimum gap between list API calls to stay
// well under rate limits.
const DefaultListCallPacing = 200 * time.Millisecond

// DefaultWorkers bounds the parallel read pool; deletions stay sequential elsewhere.
const DefaultWorkers = 6

// Pacer paces list API calls. It is an interface so tests can run without
// sleeping.
type Pacer interface {
	Pace(ctx context.Context) error
}

// FixedPacer enforces a minimum gap between consecutive paced calls across
// the whole run, even when calls run in parallel.
type FixedPacer struct {
	Interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

// Pace blocks until the minimum interval has elapsed since the last paced
// call.
func (p *FixedPacer) Pace(ctx context.Context) error {
	if p == nil || p.Interval <= 0 {
		return nil
	}
	p.mu.Lock()
	wait := p.Interval - time.Since(p.last)
	p.last = time.Now()
	p.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}
	return nil
}

// NopPacer never waits (tests).
type NopPacer struct{}

// Pace is a no-op.
func (NopPacer) Pace(context.Context) error { return nil }

// containerRef is one node of the org-tree walk.
type containerRef struct {
	ID   string
	Name string
	// folder is non-nil for folder containers; it carries the labels used
	// for the folder's own protection check.
	folder *stackit.Folder
}

// scannedProject is a walk result with the protection state computed on
// the container it was listed from.
type scannedProject struct {
	stackit.Project
	ParentFolderName string
	// Invisible projects (whitelisted project, or inside a whitelisted or
	// safe-labeled folder) appear nowhere in the report and contribute no
	// IaaS candidates (3.3, 3.4).
	Invisible bool
}

// Scanner runs all resource scanners for one run.
type Scanner struct {
	Clients   *stackit.Set
	Whitelist *whitelist.Whitelist
	Config    config.Config
	Logger    *slog.Logger

	Pacer   Pacer
	Now     func() time.Time
	Workers int
}

// New builds a Scanner with production defaults (200 ms pacing, 6 workers,
// wall clock).
func New(clients *stackit.Set, wl *whitelist.Whitelist, cfg config.Config, logger *slog.Logger) *Scanner {
	return &Scanner{
		Clients:   clients,
		Whitelist: wl,
		Config:    cfg,
		Logger:    logger,
		Pacer:     &FixedPacer{Interval: DefaultListCallPacing},
		Now:       time.Now,
		Workers:   DefaultWorkers,
	}
}

// Scan performs the full scan (org walk, hygiene, deletion candidates with
// mark maintenance, billing) and returns the shared report.
// Individual scanner failures are collected in Report.ScanErrors and never
// abort the run.
func (s *Scanner) Scan(ctx context.Context) (*report.Report, error) {
	rep := &report.Report{
		GeneratedAt: s.Now().UTC(),
		DryRun:      s.Config.DryRun,
		Scope:       s.Config.Scope,
		OrgID:       s.Config.OrgID,
		Regions:     s.Config.Regions,
	}
	addError := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		rep.ScanErrors = append(rep.ScanErrors, msg)
		s.Logger.Warn("scanner error (run continues)", "error", msg)
	}

	projects, err := s.walk(ctx)
	if err != nil {
		addError("org/folder walk: %v", err)
	}
	active := make([]scannedProject, 0, len(projects))
	for _, p := range projects {
		if p.Invisible {
			continue
		}
		active = append(active, p)
	}

	rep.StaleProjects = s.staleProjects(ctx, active, addError)
	rep.EmptySNAs = s.emptySNAs(ctx, addError)

	var mu sync.Mutex
	var idleIPs []report.IdlePublicIP
	var volumes []report.DetachedVolume
	jobs := make([]func(context.Context), 0, len(active)*len(s.Config.Regions))
	for _, p := range active {
		if s.Whitelist.SkipPublicIPScan(p.ID) && s.Whitelist.SkipVolumeScan(p.ID) {
			continue
		}
		for _, region := range s.Config.Regions {
			p, region := p, region
			jobs = append(jobs, func(ctx context.Context) {
				var ips []report.IdlePublicIP
				if !s.Whitelist.SkipPublicIPScan(p.ID) {
					ips = s.idlePublicIPs(ctx, p, region, addError)
				}
				var vols []report.DetachedVolume
				if !s.Whitelist.SkipVolumeScan(p.ID) {
					vols = s.detachedVolumes(ctx, p, region, addError)
				}
				mu.Lock()
				idleIPs = append(idleIPs, ips...)
				volumes = append(volumes, vols...)
				mu.Unlock()
			})
		}
	}
	runPooled(ctx, s.Workers, jobs)
	rep.IdlePublicIPs = idleIPs
	rep.DetachedVolumes = volumes

	s.fillBilling(ctx, rep, addError)
	rep.FillSavings(s.Config.PublicIPCostEurPerMonth)
	return rep, nil
}

// runPooled runs all jobs with at most workers in flight.
func runPooled(ctx context.Context, workers int, jobs []func(context.Context)) {
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, job := range jobs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(j func(context.Context)) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			j(ctx)
		}(job)
	}
	wg.Wait()
}
