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

package scanner

import (
	"context"
	"errors"
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

const DefaultCallPacing = 100 * time.Millisecond

const DefaultWorkers = 6

type Pacer interface {
	Pace(ctx context.Context) error
}

type FixedPacer struct {
	Interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

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

type NopPacer struct{}

func (NopPacer) Pace(ctx context.Context) error { return ctx.Err() }

type Scanner struct {
	Clients   *stackit.Set
	Config    config.Config
	Logger    *slog.Logger
	Pacer     Pacer
	Now       func() time.Time
	Workers   int
	DeleteRun bool
}

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

type Result struct {
	Report     *report.Report
	Flag       []stackit.Resource
	Unflag     []stackit.Resource
	ClearStale []stackit.Resource
	Delete     []stackit.Resource
	Context    DeleteContext
}

func (r *Result) Blocked() bool {
	return len(r.Report.Blocked) > 0
}

type DeleteContext struct {
	LoadBalancerAddresses map[string]map[string]string
	NICServers            map[string]string
	ProjectNames          map[string]string
}

type ScopeError struct {
	Problems []string
}

func (e *ScopeError) Error() string {
	return "the scope cannot be resolved:\n  - " + strings.Join(e.Problems, "\n  - ")
}

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

const maxPlaces = 3

func newErrorLog(logger *slog.Logger) *errorLog {
	return &errorLog{groups: map[string]*errorGroup{}, logger: logger}
}

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

func (s *Scanner) Scan(ctx context.Context) (*Result, error) {
	now := s.Now().UTC()
	errs := newErrorLog(s.Logger)
	prices := s.startPrices(ctx)

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
	priceList, err := prices()
	if err != nil {
		s.Logger.Warn("prices unavailable", "error", err)
		rep.PricesProblem = "STACKIT's price list could not be read"
	}
	res := classify(invs, rep, errs, priceList)
	if !s.DeleteRun && s.Config.WarnEmptyAfterDays > 0 {
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

func (s *Scanner) startPrices(ctx context.Context) func() (*stackit.Prices, error) {
	if s.Clients.PriceList == nil {
		return func() (*stackit.Prices, error) { return nil, errors.New("no price list") }
	}
	type result struct {
		prices *stackit.Prices
		err    error
	}
	done := make(chan result, 1)
	go func() {
		p, err := s.Clients.PriceList.Prices(ctx, s.Config.Regions)
		done <- result{p, err}
	}()
	return func() (*stackit.Prices, error) {
		r := <-done
		return r.prices, r.err
	}
}

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
