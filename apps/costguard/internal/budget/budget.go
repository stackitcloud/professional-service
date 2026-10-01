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

package budget

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const dateLayout = "2006-01-02"

const forecastFromDay = 7

const topProjects = 5

type Checker struct {
	Clients *stackit.Set
	Config  config.Config
	Logger  *slog.Logger
	Pacer   scanner.Pacer
	Now     func() time.Time
}

func New(clients *stackit.Set, cfg config.Config, logger *slog.Logger) *Checker {
	return &Checker{
		Clients: clients,
		Config:  cfg,
		Logger:  logger,
		Pacer:   &scanner.FixedPacer{Interval: scanner.DefaultCallPacing},
		Now:     time.Now,
	}
}

func (c *Checker) Check(ctx context.Context) (*report.BudgetCheck, error) {
	if c.Config.Budgets == nil {
		return nil, fmt.Errorf("no budgets are configured")
	}
	now := c.Now()
	today := utcDay(now)
	res := &report.BudgetCheck{
		Month:       time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC),
		Checked:     today.AddDate(0, 0, -1),
		GeneratedAt: now.UTC(),
	}

	targets, problems, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	res.Problems = problems

	costs := &stackit.DailyCosts{}
	if !res.NothingIn() {
		if costs, err = c.dailyCosts(ctx, res.Month, res.Checked); err != nil {
			return nil, err
		}
		res.LastModified = costs.LastModified
		switch {
		case costs.LastModified.IsZero():
			c.Logger.Warn("the Cost API did not say when it last updated the costs; using them as they are")
		case costs.LastModified.Before(today):
			res.Checked = utcDay(costs.LastModified).AddDate(0, 0, -1)
		}
	}
	for _, t := range targets {
		res.Budgets = append(res.Budgets, evaluate(t, costs, res.Month, res.Checked))
	}
	return res, nil
}

func (c *Checker) dailyCosts(ctx context.Context, from, to time.Time) (*stackit.DailyCosts, error) {
	if err := c.Pacer.Pace(ctx); err != nil {
		return nil, err
	}
	costs, err := c.Clients.Cost.DailyCosts(ctx, c.Config.OrganizationID, from, to)
	if err != nil {
		msg := fmt.Sprintf("costguard cannot read the costs of the organization %s.", c.Config.OrganizationID)
		if code, ok := stackit.StatusCode(err); ok && (code == 401 || code == 403) {
			msg += " Check that its service account has the costguard.reader role on this organization (Terraform assigns it; run terraform apply)."
		}
		return nil, fmt.Errorf("%s\n  - %s", msg, stackit.Describe(err))
	}
	return costs, nil
}

func evaluate(t target, costs *stackit.DailyCosts, month, checked time.Time) report.BudgetStatus {
	b := t.budget
	st := report.BudgetStatus{
		Name: b.Name, Target: t.describe, Organization: t.wholeOrg, FolderID: t.folderID, ProjectID: t.projectID,
		LimitEUR: b.MonthlyEUR, Thresholds: append([]int(nil), b.Thresholds...),
	}
	if checked.Before(month) {
		return st
	}
	firstKey, dayKey := month.Format(dateLayout), checked.Format(dateLayout)

	ids := make([]string, 0, len(costs.Projects))
	for id := range costs.Projects {
		if t.covers(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var spends []report.ProjectSpend
	for _, id := range ids {
		p := costs.Projects[id]
		days := make([]string, 0, len(p.EUR))
		for d := range p.EUR {
			if d >= firstKey && d <= dayKey {
				days = append(days, d)
			}
		}
		sort.Strings(days)
		var project float64
		for _, d := range days {
			project += p.EUR[d]
		}
		st.MonthEUR += project
		st.DayEUR += p.EUR[dayKey]
		spends = append(spends, report.ProjectSpend{ID: id, Name: p.Name, EUR: cents(project)})
	}
	st.MonthEUR, st.DayEUR = cents(st.MonthEUR), cents(st.DayEUR)

	for _, th := range b.Thresholds {
		if st.MonthEUR >= report.ThresholdEUR(b.MonthlyEUR, th) {
			st.Reached = th
		}
	}
	if checked.Day() >= forecastFromDay {
		daysInMonth := month.AddDate(0, 1, -1).Day()
		st.ForecastEUR = cents(st.MonthEUR / float64(checked.Day()) * float64(daysInMonth))
	}
	if t.projectID == "" {
		sort.SliceStable(spends, func(i, j int) bool { return spends[i].EUR > spends[j].EUR })
		for _, s := range spends {
			if len(st.Top) == topProjects || s.EUR <= 0 {
				break
			}
			st.Top = append(st.Top, s)
		}
	}
	return st
}

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func cents(eur float64) float64 {
	return math.Round(eur*100) / 100
}
