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

package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

func (c Composer) Budgets(chk *report.BudgetCheck) (m Message, ok bool) {
	reached := chk.Reached()
	if chk.NothingIn() || (len(reached) == 0 && len(chk.Problems) == 0) {
		return Message{}, false
	}
	month := chk.Month.Format("January")
	m = Message{
		Title:    c.title("budget"),
		Subtitle: fmt.Sprintf("Costs up to and including %s (UTC) · %s", chk.Checked.Format("2 January 2006"), c.clock(chk.GeneratedAt)),
		Footer:   "costguard " + c.Version,
	}
	if len(reached) > 0 {
		m.Intro = fmt.Sprintf("Every budgets run lists the budgets at or above a threshold, until %s ends.", month)
	}
	for _, b := range reached {
		sec := Section{Lines: []Line{{
			Text: fmt.Sprintf("%s: %s of %s in %s (%s)", b.Name, eur(b.MonthEUR), eur(b.LimitEUR), month, percent(b.Percent())),
			Link: c.budgetLink(b),
		}}}
		if b.ForecastEUR > 0 {
			sec.Lines = append(sec.Lines, Line{Text: fmt.Sprintf("Forecast for %s: about %s (%s)",
				month, eur(b.ForecastEUR), percent(b.ForecastEUR/b.LimitEUR*100))})
		}
		if len(b.Top) > 0 {
			sec.Lines = append(sec.Lines, Line{Text: "Top projects in " + month + ":"})
		}
		for i, p := range b.Top {
			sec.Lines = append(sec.Lines, Line{Text: fmt.Sprintf("%s: %s", p.Name, eur(p.EUR)), Link: dashboard(c.PortalURL, "project", p.ID), Number: i + 1})
		}
		m.Sections = append(m.Sections, sec)
	}
	addBudgetProblems(&m, chk.Problems)
	return m, true
}

func (c Composer) addBudgets(m *Message, chk *report.BudgetCheck) {
	sec := Section{Title: fmt.Sprintf("Budgets for %s (UTC days): %d", chk.Month.Format("January 2006"), len(chk.Budgets))}
	sec.Lines = append(sec.Lines, Line{Text: c.costsAsOf(chk)})
	for _, b := range chk.Budgets {
		thresholds := make([]string, len(b.Thresholds))
		for i, t := range b.Thresholds {
			thresholds[i] = fmt.Sprintf("%d%%", t)
		}
		sec.Lines = append(sec.Lines, Line{
			Text: fmt.Sprintf("%s · %s · %s of %s so far (%s) · alerts at %s",
				b.Name, b.Target, eur(b.MonthEUR), eur(b.LimitEUR), percent(b.Percent()), strings.Join(thresholds, ", ")),
			Link: c.budgetLink(b),
		})
	}
	m.Sections = append(m.Sections, sec)
	addBudgetProblems(m, chk.Problems)
}

func (c Composer) costsAsOf(chk *report.BudgetCheck) string {
	text := "Costs from the 1st up to and including " + chk.Checked.Format("2 January")
	if chk.NothingIn() {
		text = "No costs of " + chk.Month.Format("January") + " are in yet"
	}
	if !chk.LastModified.IsZero() {
		text += "; STACKIT last updated them " + c.clock(chk.LastModified)
	}
	return text + "."
}

func addBudgetProblems(m *Message, problems []string) {
	if len(problems) == 0 {
		return
	}
	m.Alerts = append(m.Alerts, report.Count(len(problems), "budget")+" could not be checked; details at the end.")
	sec := Section{Title: fmt.Sprintf("Budgets not checked: %d", len(problems)), Folded: true}
	for _, p := range problems {
		sec.Lines = append(sec.Lines, Line{Text: p})
	}
	m.Sections = append(m.Sections, sec)
}

func (c Composer) budgetLink(b report.BudgetStatus) string {
	switch {
	case b.ProjectID != "":
		return dashboard(c.PortalURL, "project", b.ProjectID)
	case b.FolderID != "":
		return dashboard(c.PortalURL, "folder", b.FolderID)
	case b.Organization:
		return dashboard(c.PortalURL, "organization", c.OrganizationID)
	}
	return ""
}

func (c Composer) clock(t time.Time) string {
	if c.Location == nil || c.Location == time.UTC {
		return t.UTC().Format("2006-01-02 15:04 UTC")
	}
	return t.In(c.Location).Format("2006-01-02 15:04") + " (" + c.Location.String() + ")"
}

func percent(v float64) string {
	return fmt.Sprintf("%.0f%%", v)
}
