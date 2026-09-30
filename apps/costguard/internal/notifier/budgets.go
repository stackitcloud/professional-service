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

// Budgets composes the message of a budgets run: every budget at or above
// one of its thresholds, and the budgets that could not be checked. ok is
// false when there is nothing to say: no cost of the month is in yet, or no
// budget reached a threshold and all could be checked.
func (c Composer) Budgets(chk *report.BudgetCheck) (m Message, ok bool) {
	reached := chk.Reached()
	if chk.NothingIn() || (len(reached) == 0 && len(chk.Problems) == 0) {
		return Message{}, false
	}
	month := chk.Month.Format("January")
	m = Message{
		Subtitle: fmt.Sprintf("Costs up to and including %s (UTC) · %s", chk.Checked.Format("2 January 2006"), c.clock(chk.GeneratedAt)),
		Footer:   "costguard " + c.Version,
	}
	switch len(reached) {
	case 0:
		m.Title = "costguard: budgets could not all be checked"
	case 1:
		m.Title = fmt.Sprintf("costguard: budget %s passed %d%%", reached[0].Name, reached[0].Reached)
	default:
		m.Title = fmt.Sprintf("costguard: %s passed a threshold", report.Count(len(reached), "budget"))
	}
	if len(reached) > 0 {
		m.Intro = fmt.Sprintf("Every budgets run lists the budgets at or above a threshold, until %s ends.", month)
	}
	for _, b := range reached {
		sec := Section{Title: fmt.Sprintf("%s (%s): passed %d%%", b.Name, b.Target, b.Reached)}
		sec.Lines = append(sec.Lines,
			Line{Text: fmt.Sprintf("%s of %s in %s (%s)", eur(b.MonthEUR), eur(b.LimitEUR), month, percent(b.Percent())), Link: c.projectLink(b.ProjectID)},
			Line{Text: fmt.Sprintf("%s: %s", chk.Checked.Format("2 January"), eur(b.DayEUR))},
		)
		if b.ForecastEUR > 0 {
			sec.Lines = append(sec.Lines, Line{Text: fmt.Sprintf("Forecast for %s: about %s (%s)",
				month, eur(b.ForecastEUR), percent(b.ForecastEUR/b.LimitEUR*100))})
		}
		if next := b.NextThreshold(); next > 0 {
			sec.Lines = append(sec.Lines, Line{Text: fmt.Sprintf("Next threshold: %d%%, %s", next, eur(report.ThresholdEUR(b.LimitEUR, next)))})
		} else {
			sec.Lines = append(sec.Lines, Line{Text: "Every threshold of this budget is passed"})
		}
		for _, p := range b.Top {
			sec.Lines = append(sec.Lines, Line{Text: fmt.Sprintf("Top project %s: %s in %s", p.Name, eur(p.EUR), month), Link: c.projectLink(p.ID)})
		}
		m.Sections = append(m.Sections, sec)
	}
	addBudgetProblems(&m, chk.Problems)
	return m, true
}

// addBudgets adds where every budget stands to the boot message.
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
			Link: c.projectLink(b.ProjectID),
		})
	}
	m.Sections = append(m.Sections, sec)
	addBudgetProblems(m, chk.Problems)
}

// costsAsOf says which days the amounts cover and when STACKIT last
// updated them.
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

// addBudgetProblems lists the budgets that could not be checked, like
// scan errors: one alert, the details at the end.
func addBudgetProblems(m *Message, problems []string) {
	if len(problems) == 0 {
		return
	}
	m.Alerts = append(m.Alerts, report.Count(len(problems), "budget")+" could not be checked; details at the end.")
	sec := Section{Title: fmt.Sprintf("Budgets not checked: %d", len(problems))}
	for _, p := range problems {
		sec.Lines = append(sec.Lines, Line{Text: p})
	}
	m.Sections = append(m.Sections, sec)
}

// projectLink is the portal page of a project, or "".
func (c Composer) projectLink(projectID string) string {
	return PortalLink(c.PortalURL, report.Item{Kind: report.KindProject, ProjectID: projectID})
}

// clock renders a time in the configured zone, e.g. "2026-10-01 14:03
// (Europe/Berlin)".
func (c Composer) clock(t time.Time) string {
	if c.Location == nil || c.Location == time.UTC {
		return t.UTC().Format("2006-01-02 15:04 UTC")
	}
	return t.In(c.Location).Format("2006-01-02 15:04") + " (" + c.Location.String() + ")"
}

func percent(v float64) string {
	return fmt.Sprintf("%.0f%%", v)
}
