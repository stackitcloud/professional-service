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

// Package scanner reads the scope and decides what happens to every
// resource. It never writes: the flag and delete runs act on its Result.
// Anything that cannot be read is reported as a scan error and left alone.
package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// DefaultCallPacing is the minimum gap between two API calls of a scan. ❓
// The STACKIT rate limits are not documented; verify on a large org.
const DefaultCallPacing = 100 * time.Millisecond

// DefaultWorkers bounds the parallel project/region jobs.
const DefaultWorkers = 6

// Pacer spaces out API calls.
type Pacer interface {
	Pace(ctx context.Context) error
}

// FixedPacer enforces a minimum gap between consecutive calls across the
// whole run, also when calls run in parallel.
type FixedPacer struct {
	Interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

// Pace blocks until Interval has passed since the previous call.
func (p *FixedPacer) Pace(ctx context.Context) error {
	if p == nil || p.Interval <= 0 || ctx.Err() != nil {
		return ctx.Err()
	}
	p.mu.Lock()
	now := time.Now()
	next := p.last.Add(p.Interval)
	if next.Before(now) {
		next = now
	}
	p.last = next
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Until(next)):
		return nil
	}
}

// NopPacer never waits (tests).
type NopPacer struct{}

// Pace only checks the context.
func (NopPacer) Pace(ctx context.Context) error { return ctx.Err() }

// Scanner scans the configured scope.
type Scanner struct {
	Clients *stackit.Set
	Config  config.Config
	Logger  *slog.Logger
	Pacer   Pacer
	Now     func() time.Time
	Workers int
	// SkipWarnings leaves out the empty-project and empty-network-area
	// checks. The delete run sets it: it never shows them, and their calls
	// and scan errors would only cost time and add noise.
	SkipWarnings bool
}

// New builds a Scanner with production defaults.
func New(clients *stackit.Set, cfg config.Config, logger *slog.Logger) *Scanner {
	return &Scanner{
		Clients: clients,
		Config:  cfg,
		Logger:  logger,
		Pacer:   &FixedPacer{Interval: DefaultCallPacing},
		Now:     time.Now,
		Workers: DefaultWorkers,
	}
}

// Result is what a run needs after the scan.
type Result struct {
	Report *report.Report
	// Flag lists new cleanup candidates that get delete=true.
	Flag []stackit.Resource
	// Unflag lists volumes and public IPs whose delete label is removed
	// because they are in use again.
	Unflag []stackit.Resource
	// ClearStale lists volumes and public IPs that carry do-not-delete and
	// a leftover delete=true; the flag run removes the delete label.
	ClearStale []stackit.Resource
	// Delete lists what the delete run deletes, in deletion order.
	Delete []stackit.Resource
	// Context is what the delete run needs to re-check volumes and IPs.
	Context DeleteContext
}

// Blocked reports whether a skip entry matched nothing. A blocked run
// neither flags nor deletes.
func (r *Result) Blocked() bool {
	return len(r.Report.Blocked) > 0
}

// DeleteContext carries scan facts the deleter uses when it re-checks a
// volume or public IP right before deleting it.
type DeleteContext struct {
	// LoadBalancerAddresses maps "project/region" to the load balancer
	// addresses there.
	LoadBalancerAddresses map[string]map[string]string
	// NICServers maps a NIC ID to the server it is attached to.
	NICServers map[string]string
	// ProjectNames maps project IDs to names.
	ProjectNames map[string]string
}

// ScopeError means a scope entry did not resolve to exactly one
// container. The run must stop before any write.
type ScopeError struct {
	Problems []string
}

func (e *ScopeError) Error() string {
	return "the scope cannot be resolved:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// errorLog collects scan errors from parallel jobs and groups identical
// causes, so a missing permission shows up once with the places it hit,
// not once per project. The raw error goes to the log.
type errorLog struct {
	mu     sync.Mutex
	order  []string
	groups map[string]*errorGroup
	logger *slog.Logger
}

type errorGroup struct {
	what, cause string
	where       []string
}

// maxPlaces is how many places a grouped scan error names.
const maxPlaces = 3

func newErrorLog(logger *slog.Logger) *errorLog {
	return &errorLog{groups: map[string]*errorGroup{}, logger: logger}
}

// add records that `what` failed at `where` (may be empty) because of err.
func (l *errorLog) add(what, where string, err error) {
	cause := stackit.Describe(err)
	key := what + "\x00" + cause
	l.mu.Lock()
	g, ok := l.groups[key]
	if !ok {
		g = &errorGroup{what: what, cause: cause}
		l.groups[key] = g
		l.order = append(l.order, key)
	}
	if where != "" {
		g.where = append(g.where, where)
	}
	l.mu.Unlock()
	l.logger.Warn("scan error (left alone, run continues)", "what", what, "where", where, "error", err.Error())
}

// lines renders one line per cause, e.g. "listing images: HTTP 403: …
// (in 300 places, e.g. shop (eu01), web (eu01), api (eu01))". Causes and
// places are sorted: the parallel jobs finish in random order, and the
// message should not change from run to run because of that.
func (l *errorLog) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	keys := append([]string(nil), l.order...)
	sort.Strings(keys)
	var out []string
	for _, key := range keys {
		g := l.groups[key]
		sort.Strings(g.where)
		line := g.what + ": " + g.cause
		switch n := len(g.where); {
		case n == 1:
			line += " (" + g.where[0] + ")"
		case n > 1 && n <= maxPlaces:
			line += fmt.Sprintf(" (in %d places: %s)", n, strings.Join(g.where, ", "))
		case n > maxPlaces:
			line += fmt.Sprintf(" (in %d places, e.g. %s)", n, strings.Join(g.where[:maxPlaces], ", "))
		}
		out = append(out, line)
	}
	return out
}

// Scan reads the scope and returns the report plus what to write or
// delete. It returns an error only when the scope itself cannot be
// resolved (ScopeError) or the context ends.
func (s *Scanner) Scan(ctx context.Context) (*Result, error) {
	now := s.Now().UTC()
	errs := newErrorLog(s.Logger)

	tree, err := s.resolve(ctx, errs)
	if err != nil {
		return nil, err
	}
	rep := &report.Report{
		GeneratedAt:     now,
		OrganizationID:  s.Config.OrganizationID,
		Scope:           tree.describe,
		Regions:         s.Config.Regions,
		Blocked:         tree.dangling,
		SkippedFolders:  tree.skippedFolders,
		SkippedProjects: tree.skippedProjects,
	}

	projects := tree.active()
	invs := s.inventory(ctx, projects, errs)
	res := classify(invs, rep, errs)
	if !s.SkipWarnings && s.Config.WarnEmptyAfterDays > 0 {
		s.emptyProjects(ctx, projects, invs, rep, errs, now)
		if tree.wholeOrg {
			s.emptyNetworkAreas(ctx, rep, errs, now)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res.Context.ProjectNames = make(map[string]string, len(projects))
	for _, p := range projects {
		res.Context.ProjectNames[p.ID] = p.Name
	}
	rep.ScanErrors = errs.lines()
	res.Report = rep
	return res, nil
}

// runPooled runs jobs with at most workers in flight.
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
		sem <- struct{}{}
		go func(j func(context.Context)) {
			defer wg.Done()
			defer func() { <-sem }()
			j(ctx)
		}(job)
	}
	wg.Wait()
}
