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

package config

import (
	"strings"
	"testing"
)

func TestParseBudgets(t *testing.T) {
	cfg, err := Parse([]byte(`
organizationId: ` + orgID + `
reportEnabled: false
budgets:
  days: [Mon, Tue, Wed, Thu, Fri]
  time: "10:00"
  limits:
    - name: Whole organization
      organization: true
      monthlyEur: 20000
      thresholds: [80, 100]
    - name: Team A
      folder: team-a
      monthlyEur: 2500.5
      thresholds: [50]
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReportEnabled || cfg.Budgets == nil || cfg.Budgets.Time != "10:00" || len(cfg.Budgets.Limits) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	a := cfg.Budgets.Limits[1]
	if a.Name != "Team A" || a.Folder != "team-a" || a.Organization || a.MonthlyEUR != 2500.5 || len(a.Thresholds) != 1 || a.Thresholds[0] != 50 {
		t.Errorf("limit = %+v", a)
	}
	if got := cfg.BudgetsRunAt(); got != "Monday to Friday 10:00 (Europe/Berlin)" {
		t.Errorf("BudgetsRunAt = %q", got)
	}
	if _, err := Parse([]byte("budgets:\n  limits:\n    - name: x\n      folders: team-a\n")); err == nil || !strings.Contains(err.Error(), "folders") {
		t.Errorf("a misspelled budget key must fail: %v", err)
	}
}

func TestBudgetsOffAndReportOnByDefault(t *testing.T) {
	cfg := validConfig()
	if cfg.Budgets != nil || cfg.BudgetsRunAt() != "" || !cfg.ReportEnabled {
		t.Errorf("defaults: budgets %+v, report %v", cfg.Budgets, cfg.ReportEnabled)
	}
}

func TestDeleteNeedsReport(t *testing.T) {
	cfg := validConfig()
	cfg.ReportEnabled, cfg.DeleteEnabled, cfg.DeleteRunAt = false, true, "Tuesday 08:00"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "deleteEnabled needs reportEnabled") {
		t.Errorf("err = %v", err)
	}
}

func budgetConfig(limits ...Budget) *Config {
	cfg := validConfig()
	cfg.Budgets = &Budgets{Days: []string{"Mon", "Tue", "Wed", "Thu", "Fri"}, Time: "10:00", Limits: limits}
	return cfg
}

func TestValidateAcceptsBudgets(t *testing.T) {
	cfg := budgetConfig(
		Budget{Name: "Org", Organization: true, MonthlyEUR: 20000, Thresholds: []int{80, 100}},
		Budget{Name: "Team A", Folder: "team-a", MonthlyEUR: 2500, Thresholds: []int{1, 1000}},
		Budget{Name: "Sandbox", Project: "aaaaaaaa-0000-0000-0000-000000000001", MonthlyEUR: 0.5, Thresholds: []int{50}},
	)
	if err := cfg.Validate(); err != nil {
		t.Error(err)
	}
}

func TestValidateBudgetProblems(t *testing.T) {
	ok := func() Budget { return Budget{Name: "A", Organization: true, MonthlyEUR: 1, Thresholds: []int{80}} }
	for name, tc := range map[string]struct {
		limits []Budget
		time   string
		want   string
	}{
		"no limits":      {nil, "10:00", "budgets.limits must list at least one budget"},
		"bad time":       {[]Budget{ok()}, "10", `budgets.time must be HH:MM (24 hours), e.g. "10:00" (got "10")`},
		"no name":        {[]Budget{func() Budget { b := ok(); b.Name = " "; return b }()}, "10:00", "budgets.limits[0] needs a name"},
		"same name":      {[]Budget{ok(), func() Budget { b := ok(); b.Name = "a"; return b }()}, "10:00", `budget name "a" is used twice`},
		"no target":      {[]Budget{func() Budget { b := ok(); b.Organization = false; return b }()}, "10:00", `budget "A" needs exactly one target`},
		"two targets":    {[]Budget{func() Budget { b := ok(); b.Folder = "f"; return b }()}, "10:00", `budget "A" needs exactly one target`},
		"folder+project": {[]Budget{{Name: "A", Folder: "f", Project: "p", MonthlyEUR: 1, Thresholds: []int{80}}}, "10:00", "needs exactly one target"},
		"zero amount":    {[]Budget{func() Budget { b := ok(); b.MonthlyEUR = 0; return b }()}, "10:00", `budget "A": monthlyEur must be more than 0`},
		"no thresholds":  {[]Budget{func() Budget { b := ok(); b.Thresholds = nil; return b }()}, "10:00", `budget "A" needs at least one threshold`},
		"threshold 0":    {[]Budget{func() Budget { b := ok(); b.Thresholds = []int{0}; return b }()}, "10:00", "thresholds must be between 1 and 1000 percent (got 0)"},
		"threshold 1001": {[]Budget{func() Budget { b := ok(); b.Thresholds = []int{1001}; return b }()}, "10:00", "(got 1001)"},
		"not ascending":  {[]Budget{func() Budget { b := ok(); b.Thresholds = []int{100, 80}; return b }()}, "10:00", "thresholds must be ascending without repeats (got [100 80])"},
		"repeated":       {[]Budget{func() Budget { b := ok(); b.Thresholds = []int{80, 80}; return b }()}, "10:00", "ascending without repeats"},
	} {
		cfg := budgetConfig(tc.limits...)
		cfg.Budgets.Time = tc.time
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func TestAnyBudgetsTimeWorks(t *testing.T) {
	for _, clock := range []string{"00:00", "06:00", "23:59"} {
		cfg := budgetConfig(Budget{Name: "A", Organization: true, MonthlyEUR: 1, Thresholds: []int{80}})
		cfg.Budgets.Time = clock
		if err := cfg.Validate(); err != nil {
			t.Errorf("%s: %v", clock, err)
		}
	}
}

func TestBudgetDays(t *testing.T) {
	for _, tc := range []struct {
		days []string
		want string
	}{
		{nil, "budgets.days must list at least one day"},
		{[]string{"Monday"}, `budgets.days: "Monday" is not a day; write Mon, Tue, Wed, Thu, Fri, Sat or Sun`},
		{[]string{"Mon", "Mon"}, "budgets.days lists Mon twice"},
	} {
		cfg := budgetConfig(Budget{Name: "A", Organization: true, MonthlyEUR: 1, Thresholds: []int{80}})
		cfg.Budgets.Days = tc.days
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: err = %v, want %q", tc.days, err, tc.want)
		}
	}
}

func TestDaysText(t *testing.T) {
	for _, tc := range []struct {
		days []string
		want string
	}{
		{Weekdays, "every day"},
		{[]string{"Fri", "Thu", "Wed", "Tue", "Mon"}, "Monday to Friday"},
		{[]string{"Fri", "Mon", "Wed"}, "Monday, Wednesday and Friday"},
		{[]string{"Sun", "Sat"}, "Saturday and Sunday"},
		{[]string{"Tue"}, "Tuesday"},
		{[]string{"Mon", "Tue", "Wed", "Thu"}, "Monday, Tuesday, Wednesday and Thursday"},
	} {
		if got := DaysText(tc.days); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.days, got, tc.want)
		}
	}
}
