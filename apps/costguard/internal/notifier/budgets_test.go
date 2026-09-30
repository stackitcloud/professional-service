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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

var (
	september  = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	checkedDay = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	checkedAt  = time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	updatedAt  = time.Date(2026, 9, 29, 7, 41, 0, 0, time.UTC)
)

func berlinComposer(t *testing.T) Composer {
	t.Helper()
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	c := composer()
	c.Location = berlin
	return c
}

func teamA() report.BudgetStatus {
	return report.BudgetStatus{
		Name: "Team A", Target: "folder team-a", LimitEUR: 2500, Thresholds: []int{80, 100},
		MonthEUR: 2034.12, DayEUR: 123.45, Reached: 80, ForecastEUR: 2179.41,
		Top: []report.ProjectSpend{{ID: "p1", Name: "shop", EUR: 1200}, {ID: "p2", Name: "api", EUR: 834.12}},
	}
}

func sandbox() report.BudgetStatus {
	return report.BudgetStatus{
		Name: "Sandbox", Target: "project sandbox", ProjectID: "p3", LimitEUR: 50, Thresholds: []int{100},
		MonthEUR: 51, DayEUR: 2, Reached: 100,
	}
}

func quiet() report.BudgetStatus {
	return report.BudgetStatus{Name: "Quiet", Target: "organization", LimitEUR: 1000, Thresholds: []int{80}, MonthEUR: 10}
}

func september28(budgets ...report.BudgetStatus) *report.BudgetCheck {
	return &report.BudgetCheck{Month: september, Checked: checkedDay, GeneratedAt: checkedAt, LastModified: updatedAt, Budgets: budgets}
}

func lines(sec Section) string {
	var out []string
	for _, l := range sec.Lines {
		out = append(out, l.Text+" <"+l.Link+">")
	}
	return strings.Join(out, "\n")
}

func TestComposeOneBudgetReached(t *testing.T) {
	m, ok := berlinComposer(t).Budgets(september28(teamA(), quiet()))
	if !ok {
		t.Fatal("want a message")
	}
	if m.Title != "costguard: budget Team A passed 80%" || m.Subtitle != "Costs up to and including 28 September 2026 (UTC) · 2026-09-29 10:00 (Europe/Berlin)" {
		t.Errorf("title/subtitle = %q / %q", m.Title, m.Subtitle)
	}
	if m.Intro != "Every budgets run lists the budgets at or above a threshold, until September ends." {
		t.Errorf("intro = %q", m.Intro)
	}
	if len(m.Sections) != 1 || m.Sections[0].Title != "Team A (folder team-a): passed 80%" {
		t.Fatalf("sections = %s", titles(m))
	}
	want := strings.Join([]string{
		"€2034.12 of €2500.00 in September (81%) <>",
		"Forecast for September: about €2179.41 (87%) <>",
		"Top project shop: €1200.00 in September <https://portal.example/projects/p1>",
		"Top project api: €834.12 in September <https://portal.example/projects/p2>",
	}, "\n")
	if got := lines(m.Sections[0]); got != want {
		t.Errorf("lines =\n%s\nwant\n%s", got, want)
	}
	if len(m.Alerts) != 0 || m.Footer != "costguard v0.1.0" {
		t.Errorf("alerts %v, footer %q", m.Alerts, m.Footer)
	}
}

func TestComposeSeveralBudgetsAndProblems(t *testing.T) {
	chk := september28(teamA(), sandbox())
	chk.Problems = []string{`budget "Gone": folder "team-b" matches nothing (or it is not readable)`}
	m, ok := composer().Budgets(chk)
	if !ok || m.Title != "costguard: 2 budgets passed a threshold" {
		t.Fatalf("title = %q", m.Title)
	}
	if got := titles(m); got != "Team A (folder team-a): passed 80% | Sandbox (project sandbox): passed 100% | Budgets not checked: 1" {
		t.Errorf("sections = %s", got)
	}
	want := "€51.00 of €50.00 in September (102%) <https://portal.example/projects/p3>"
	if got := lines(m.Sections[1]); got != want {
		t.Errorf("sandbox lines =\n%s\nwant\n%s", got, want)
	}
	if len(m.Alerts) != 1 || m.Alerts[0] != "1 budget could not be checked; details at the end." {
		t.Errorf("alerts = %v", m.Alerts)
	}
	if m.Subtitle != "Costs up to and including 28 September 2026 (UTC) · 2026-09-29 08:00 UTC" {
		t.Errorf("subtitle = %q", m.Subtitle)
	}
}

func TestComposeBudgetProblemsOnly(t *testing.T) {
	chk := september28(quiet())
	chk.Problems = []string{"a", "b"}
	m, ok := composer().Budgets(chk)
	if !ok || m.Title != "costguard: budgets could not all be checked" || m.Intro != "" || m.Alerts[0] != "2 budgets could not be checked; details at the end." {
		t.Errorf("message = %+v", m)
	}
}

func TestComposeNoBudgetNews(t *testing.T) {
	if _, ok := composer().Budgets(september28(quiet())); ok {
		t.Error("no budget reached a threshold and no problem: no message")
	}
	nothing := &report.BudgetCheck{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Checked: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
		Budgets: []report.BudgetStatus{sandbox()}, Problems: []string{"x"}}
	if _, ok := composer().Budgets(nothing); ok {
		t.Error("nothing in yet: no message")
	}
}

func TestComposeBootWithBudgets(t *testing.T) {
	c := berlinComposer(t)
	c.ReportRunAt, c.DeleteRunAt, c.BudgetsRunAt = "Monday 08:00 (Europe/Berlin)", "", "Monday to Friday 10:00 (Europe/Berlin)"
	chk := september28(teamA(), sandbox())
	chk.Problems = []string{`budget "Gone": folder "team-b" matches nothing`}
	m := c.Boot(fullReport(), chk, nil, checkedAt)
	if m.Title != "costguard v0.1.0 is running" || !strings.HasSuffix(m.Intro, "Next runs: report Monday 08:00 (Europe/Berlin) · budgets Monday to Friday 10:00 (Europe/Berlin).") {
		t.Errorf("title/intro = %q / %q", m.Title, m.Intro)
	}
	if !strings.HasPrefix(titles(m), "Idle public IPs: 1") || !strings.HasSuffix(titles(m), "Budgets for September 2026 (UTC days): 2 | Budgets not checked: 1") {
		t.Errorf("sections = %s", titles(m))
	}
	want := "Costs from the 1st up to and including 28 September; STACKIT last updated them 2026-09-29 09:41 (Europe/Berlin). <>\n" +
		"Team A · folder team-a · €2034.12 of €2500.00 so far (81%) · alerts at 80%, 100% <>\n" +
		"Sandbox · project sandbox · €51.00 of €50.00 so far (102%) · alerts at 100% <https://portal.example/projects/p3>"
	if got := lines(m.Sections[len(m.Sections)-2]); got != want {
		t.Errorf("lines =\n%s\nwant\n%s", got, want)
	}
	if len(m.Alerts) != 1 || m.Alerts[0] != "1 budget could not be checked; details at the end." {
		t.Errorf("alerts = %v", m.Alerts)
	}
}

func TestComposeBootWithoutReport(t *testing.T) {
	c := berlinComposer(t)
	c.BudgetsRunAt = "every day 10:00 (Europe/Berlin)"
	chk := september28(sandbox())
	chk.LastModified = time.Time{}
	m := c.Boot(nil, chk, nil, checkedAt)
	if m.Title != "costguard v0.1.0 is running" || m.Subtitle != "2026-09-29 10:00 (Europe/Berlin)" || m.Footer != "costguard v0.1.0" ||
		m.Intro != "Login and this chat work. The cleanup report is off. Next runs: budgets every day 10:00 (Europe/Berlin)." {
		t.Errorf("message = %+v", m)
	}
	if titles(m) != "Budgets for September 2026 (UTC days): 1" || m.Sections[0].Lines[0].Text != "Costs from the 1st up to and including 28 September." {
		t.Errorf("sections = %+v", m.Sections)
	}
	if m := composer().Boot(nil, nil, nil, checkedAt); m.Intro != "Login and this chat work. The cleanup report is off." || len(m.Sections) != 0 {
		t.Errorf("message = %+v", m)
	}
}

func TestComposeBootOnTheFirst(t *testing.T) {
	chk := &report.BudgetCheck{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Checked: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
		Budgets: []report.BudgetStatus{{Name: "Org", Target: "organization", LimitEUR: 100, Thresholds: []int{80}}}}
	m := composer().Boot(nil, chk, nil, checkedAt)
	if titles(m) != "Budgets for October 2026 (UTC days): 1" || m.Sections[0].Lines[0].Text != "No costs of October are in yet." ||
		m.Sections[0].Lines[1].Text != "Org · organization · €0.00 of €100.00 so far (0%) · alerts at 80%" {
		t.Errorf("sections = %+v", m.Sections)
	}
}

func TestComposeBootWhenBudgetsFail(t *testing.T) {
	err := errors.New("costguard cannot read the costs of the organization o. Check the role.\n  - HTTP 403: forbidden")
	m := composer().Boot(&report.Report{GeneratedAt: at}, nil, err, checkedAt)
	if len(m.Alerts) != 1 || m.Alerts[0] != "The budgets could not be checked: costguard cannot read the costs of the organization o. Check the role. HTTP 403: forbidden" {
		t.Errorf("alerts = %q", m.Alerts)
	}
}
