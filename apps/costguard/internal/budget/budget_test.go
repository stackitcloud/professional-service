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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/fake"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const (
	org       = "00000000-0000-0000-0000-00000000000a"
	teamA     = "00000000-0000-0000-0000-0000000000f1"
	teamASub  = "00000000-0000-0000-0000-0000000000f2"
	shopID    = "00000000-0000-0000-0000-0000000000a1"
	apiID     = "00000000-0000-0000-0000-0000000000a2"
	webID     = "00000000-0000-0000-0000-0000000000a3"
	deletedID = "00000000-0000-0000-0000-0000000000d1"
)

// The run on Tuesday 29 September 2026 sees September up to Monday the
// 28th; STACKIT updated the costs that morning.
var (
	runAt   = time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	updated = time.Date(2026, 9, 29, 7, 41, 0, 0, time.UTC)
)

type env struct {
	store *fake.Store
	cfg   config.Config
	logs  *bytes.Buffer
	now   time.Time
}

// setup builds an organization with a folder "team-a" (projects api and,
// in its subfolder, web) and a project "shop" directly below the
// organization.
func setup(limits ...config.Budget) *env {
	e := &env{store: fake.New(), logs: &bytes.Buffer{}, now: runAt}
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	e.store.AddFolder(teamA, "team-a", org, nil)
	e.store.AddFolder(teamASub, "sub", teamA, nil)
	e.store.AddProject(shopID, "shop", org, created, nil)
	e.store.AddProject(apiID, "api", teamA, created, nil)
	e.store.AddProject(webID, "web", teamASub, created, nil)
	e.store.Daily = map[string]stackit.ProjectDays{}
	e.store.LastModified = updated
	e.cfg = config.Config{OrganizationID: org, TimeZone: "UTC", Budgets: &config.Budgets{Days: config.Weekdays, Time: "10:00", Limits: limits}}
	return e
}

// spend adds a project's charge on a September day.
func (e *env) spend(id, name string, day int, eur float64) {
	e.spendOn(id, name, time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC), eur)
}

func (e *env) spendOn(id, name string, day time.Time, eur float64) {
	p, ok := e.store.Daily[id]
	if !ok {
		p = stackit.ProjectDays{Name: name, EUR: map[string]float64{}}
	}
	p.EUR[day.Format(dateLayout)] += eur
	e.store.Daily[id] = p
}

// at sets the run's time (08:00 UTC) and STACKIT's last update (that
// morning).
func (e *env) at(year int, month time.Month, day int) {
	e.now = time.Date(year, month, day, 8, 0, 0, 0, time.UTC)
	e.store.LastModified = time.Date(year, month, day, 7, 41, 0, 0, time.UTC)
}

func (e *env) check(t *testing.T) *report.BudgetCheck {
	t.Helper()
	res, err := e.checker().Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return res
}

func (e *env) checker() *Checker {
	c := New(e.store.Set(), e.cfg, slog.New(slog.NewJSONHandler(e.logs, nil)))
	c.Pacer, c.Now = scanner.NopPacer{}, func() time.Time { return e.now }
	return c
}

func orgBudget(limit float64, thresholds ...int) config.Budget {
	return config.Budget{Name: "Whole organization", Organization: true, MonthlyEUR: limit, Thresholds: thresholds}
}

func TestOrganizationBudgetReached(t *testing.T) {
	e := setup(orgBudget(100, 80, 100))
	for d := 1; d <= 27; d++ {
		e.spend(shopID, "shop", d, 2)
	}
	e.spend(apiID, "api", 3, 20)
	e.spend(deletedID, "old-project", 2, 5) // deleted this month: still counts
	e.spend(shopID, "shop", 28, 3)
	e.spend(shopID, "shop", 29, 50) // today: not asked for, not counted

	res := e.check(t)
	if res.Month.Format(dateLayout) != "2026-09-01" || res.Checked.Format(dateLayout) != "2026-09-28" || res.NothingIn() ||
		!res.LastModified.Equal(updated) || len(res.Problems) != 0 {
		t.Fatalf("res = %+v", res)
	}
	b := res.Budgets[0]
	if b.Target != "organization" || b.MonthEUR != 82 || b.DayEUR != 3 || b.Reached != 80 || b.LimitEUR != 100 {
		t.Errorf("budget = %+v", b)
	}
	// 82 € in 28 days, 30 days in September.
	if b.ForecastEUR != 87.86 {
		t.Errorf("forecast = %v", b.ForecastEUR)
	}
	want := []report.ProjectSpend{{ID: shopID, Name: "shop", EUR: 57}, {ID: apiID, Name: "api", EUR: 20}, {ID: deletedID, Name: "old-project", EUR: 5}}
	if fmt.Sprint(b.Top) != fmt.Sprint(want) {
		t.Errorf("top = %v", b.Top)
	}
	if calls := e.store.CallsWith("list"); len(calls) != 0 {
		t.Errorf("organization budgets need no tree: %v", calls)
	}
	if calls := e.store.CallsWith("daily costs"); len(calls) != 1 || calls[0] != "daily costs 2026-09-01 2026-09-28" {
		t.Errorf("cost calls = %v", calls)
	}
	if got := res.Reached(); len(got) != 1 {
		t.Errorf("Reached() = %+v", got)
	}
}

func TestReachedEveryRunUntilTheMonthEnds(t *testing.T) {
	// No state: a budget above 80 % since the 10th is listed again, also
	// by the first run after an install in the middle of the month.
	e := setup(orgBudget(100, 80, 100))
	e.spend(shopID, "shop", 10, 85)
	for _, day := range []int{29, 30} {
		e.at(2026, 9, day)
		if b := e.check(t).Budgets[0]; b.Reached != 80 || b.MonthEUR != 85 || b.DayEUR != 0 {
			t.Errorf("%d September: %+v", day, b)
		}
	}
}

func TestHighestThresholdReached(t *testing.T) {
	e := setup(orgBudget(100, 50, 80, 100, 150))
	e.spend(shopID, "shop", 27, 70)
	e.spend(shopID, "shop", 28, 35)
	if b := e.check(t).Budgets[0]; b.Reached != 100 || b.NextThreshold() != 150 {
		t.Errorf("budget = %+v", b)
	}
}

func TestBelowEveryThreshold(t *testing.T) {
	e := setup(orgBudget(100, 80))
	e.spend(shopID, "shop", 28, 79.99)
	res := e.check(t)
	if b := res.Budgets[0]; b.Reached != 0 || len(res.Reached()) != 0 {
		t.Errorf("budget = %+v", b)
	}
}

func TestCorrectionThatLowersTheTotal(t *testing.T) {
	// A credit yesterday takes the month below 80 %: not listed any more.
	e := setup(orgBudget(100, 80))
	e.spend(shopID, "shop", 20, 85)
	e.spend(shopID, "shop", 28, -10)
	if b := e.check(t).Budgets[0]; b.Reached != 0 || b.MonthEUR != 75 || b.DayEUR != -10 {
		t.Errorf("budget = %+v", b)
	}
}

func TestThresholdExactlyReachedWithFloatSums(t *testing.T) {
	// 0.1 + 0.2 is 0.30000000000000004 in float64: rounded to cents,
	// the same answer always gives the same result.
	e := setup(orgBudget(0.3, 100))
	e.spend(shopID, "shop", 27, 0.1)
	e.spend(apiID, "api", 28, 0.2)
	if b := e.check(t).Budgets[0]; b.Reached != 100 || b.MonthEUR != 0.3 {
		t.Errorf("budget = %+v", b)
	}
}

func TestFirstOfTheMonthAsksNothing(t *testing.T) {
	// No cost of October exists yet: nothing is read, nothing is reached.
	e := setup(orgBudget(1, 80))
	e.at(2026, 10, 1)
	e.spend(shopID, "shop", 30, 500)
	res := e.check(t)
	if !res.NothingIn() || res.Month.Format(dateLayout) != "2026-10-01" || res.Checked.Format(dateLayout) != "2026-09-30" {
		t.Errorf("res = %+v", res)
	}
	if b := res.Budgets[0]; b.MonthEUR != 0 || b.Reached != 0 || b.ForecastEUR != 0 || b.Top != nil {
		t.Errorf("budget = %+v", b)
	}
	if calls := e.store.CallsWith("daily costs"); len(calls) != 0 {
		t.Errorf("cost calls = %v", calls)
	}
}

func TestSecondOfTheMonthCountsOnlyTheNewMonth(t *testing.T) {
	e := setup(orgBudget(10, 80))
	e.at(2026, 10, 2)
	e.spend(shopID, "shop", 30, 500) // September
	e.spendOn(shopID, "shop", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 9)
	res := e.check(t)
	if b := res.Budgets[0]; b.MonthEUR != 9 || b.DayEUR != 9 || b.Reached != 80 {
		t.Errorf("budget = %+v", b)
	}
	if calls := e.store.CallsWith("daily costs"); calls[0] != "daily costs 2026-10-01 2026-10-01" {
		t.Errorf("cost calls = %v", calls)
	}
}

func TestForecastFromTheSeventh(t *testing.T) {
	for _, tc := range []struct {
		day  time.Time
		want float64
	}{
		{time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), 0},     // too early
		{time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), 300},   // 30 days
		{time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), 310},   // 31 days
		{time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC), 280},   // 28 days
		{time.Date(2028, 2, 7, 0, 0, 0, 0, time.UTC), 290},   // leap year
		{time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC), 310}, // 300 € in 30 days, 31 days
	} {
		e := setup(orgBudget(1000, 100))
		next := tc.day.AddDate(0, 0, 1)
		e.at(next.Year(), next.Month(), next.Day())
		for d := time.Date(tc.day.Year(), tc.day.Month(), 1, 0, 0, 0, 0, time.UTC); !d.After(tc.day); d = d.AddDate(0, 0, 1) {
			e.spendOn(shopID, "shop", d, 10)
		}
		if b := e.check(t).Budgets[0]; b.ForecastEUR != tc.want {
			t.Errorf("%s: forecast %v, want %v", tc.day.Format(dateLayout), b.ForecastEUR, tc.want)
		}
	}
}

func TestLateCostsCountUpToTheLastDayIn(t *testing.T) {
	// STACKIT last updated yesterday morning: the 28th is not in, so the
	// month to date ends on the 27th (the answer's later days are ignored).
	e := setup(orgBudget(100, 80))
	e.store.LastModified = time.Date(2026, 9, 28, 7, 40, 0, 0, time.UTC)
	e.spend(shopID, "shop", 27, 90)
	e.spend(shopID, "shop", 28, 50)
	res := e.check(t)
	if res.Checked.Format(dateLayout) != "2026-09-27" || res.NothingIn() {
		t.Errorf("res = %+v", res)
	}
	if b := res.Budgets[0]; b.MonthEUR != 90 || b.DayEUR != 90 || b.Reached != 80 {
		t.Errorf("budget = %+v", b)
	}
}

func TestLateCostsOnTheSecond(t *testing.T) {
	// On 2 October STACKIT has not updated since 1 October: no cost of
	// October is in yet.
	e := setup(orgBudget(1, 80))
	e.at(2026, 10, 2)
	e.store.LastModified = time.Date(2026, 10, 1, 7, 40, 0, 0, time.UTC)
	e.spendOn(shopID, "shop", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 5)
	res := e.check(t)
	if !res.NothingIn() || res.Budgets[0].MonthEUR != 0 || res.Budgets[0].Reached != 0 {
		t.Errorf("res = %+v", res)
	}
}

func TestMissingLastModifiedIsUsedAsItIs(t *testing.T) {
	e := setup(orgBudget(100, 80))
	e.store.LastModified = time.Time{}
	e.spend(shopID, "shop", 28, 90)
	res := e.check(t)
	if res.Checked.Format(dateLayout) != "2026-09-28" || res.Budgets[0].Reached != 80 {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(e.logs.String(), "did not say when it last updated the costs") {
		t.Errorf("logs = %s", e.logs)
	}
}

func TestFolderBudgetSumsEveryProjectBelow(t *testing.T) {
	for _, descendants := range []bool{false, true} {
		e := setup(config.Budget{Name: "Team A", Folder: "Team-A", MonthlyEUR: 50, Thresholds: []int{50}})
		e.store.ListDescendants = descendants // listings that return more than direct children
		e.spend(apiID, "api", 5, 10)
		e.spend(webID, "web", 28, 20)
		e.spend(shopID, "shop", 28, 100) // not in the folder
		res := e.check(t)
		if len(res.Budgets) != 1 || len(res.Problems) != 0 {
			t.Fatalf("descendants %v: res = %+v", descendants, res)
		}
		b := res.Budgets[0]
		if b.Target != "folder team-a" || b.MonthEUR != 30 || b.DayEUR != 20 || b.Reached != 50 || b.ProjectID != "" || len(b.Top) != 2 || b.Top[0].Name != "web" {
			t.Errorf("descendants %v: budget = %+v", descendants, b)
		}
	}
}

func TestFolderBudgetByID(t *testing.T) {
	e := setup(config.Budget{Name: "Sub", Folder: teamASub, MonthlyEUR: 50, Thresholds: []int{50}})
	e.spend(webID, "web", 1, 7)
	e.spend(apiID, "api", 1, 100)
	if b := e.check(t).Budgets[0]; b.Target != "folder sub" || b.MonthEUR != 7 {
		t.Errorf("budget = %+v", b)
	}
}

func TestProjectBudgetByNameOrID(t *testing.T) {
	e := setup(
		config.Budget{Name: "Shop", Project: " SHOP ", MonthlyEUR: 10, Thresholds: []int{100}},
		config.Budget{Name: "API", Project: apiID, MonthlyEUR: 10, Thresholds: []int{100}},
	)
	// A project being deleted doesn't make the name ambiguous.
	e.store.Containers["00000000-0000-0000-0000-0000000000a9"] = stackit.Container{ID: "00000000-0000-0000-0000-0000000000a9", Name: "shop", ParentID: org, LifecycleState: "DELETING"}
	e.spend(shopID, "shop", 28, 10)
	e.spend(apiID, "api", 2, 3)
	res := e.check(t)
	if len(res.Problems) != 0 || len(res.Budgets) != 2 {
		t.Fatalf("res = %+v", res)
	}
	shop, api := res.Budgets[0], res.Budgets[1]
	if shop.Target != "project shop" || shop.ProjectID != shopID || shop.Reached != 100 || shop.Top != nil {
		t.Errorf("shop = %+v", shop)
	}
	if api.Target != "project api" || api.ProjectID != apiID || api.MonthEUR != 3 || api.Reached != 0 {
		t.Errorf("api = %+v", api)
	}
}

func TestTargetsThatMatchNothingOrSeveral(t *testing.T) {
	e := setup(
		orgBudget(100, 80),
		config.Budget{Name: "Gone", Folder: "team-b", MonthlyEUR: 1, Thresholds: []int{80}},
		config.Budget{Name: "Twins", Folder: "twin", MonthlyEUR: 1, Thresholds: []int{80}},
		config.Budget{Name: "Nobody", Project: "00000000-0000-0000-0000-0000000000ff", MonthlyEUR: 1, Thresholds: []int{80}},
	)
	e.store.AddFolder("00000000-0000-0000-0000-0000000000e1", "twin", org, nil)
	e.store.AddFolder("00000000-0000-0000-0000-0000000000e2", "Twin", teamA, nil)
	res := e.check(t)
	want := []string{
		`budget "Gone": folder "team-b" matches nothing (or it is not readable)`,
		`budget "Twins": folder "twin" matches 2 folders; use the ID`,
		`budget "Nobody": project "00000000-0000-0000-0000-0000000000ff" matches nothing (or it is not readable)`,
	}
	if strings.Join(res.Problems, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems = %q", res.Problems)
	}
	if len(res.Budgets) != 1 || res.Budgets[0].Target != "organization" {
		t.Errorf("the organization budget must still be checked: %+v", res.Budgets)
	}
}

func TestPartlyUnreadableTree(t *testing.T) {
	e := setup(orgBudget(100, 80), config.Budget{Name: "Shop", Project: "shop", MonthlyEUR: 1, Thresholds: []int{80}})
	e.store.Errs = map[string]error{"folders:" + teamA: fake.Status(503)}
	res := e.check(t)
	if len(res.Problems) != 1 || !strings.Contains(res.Problems[0], `budget "Shop": not checked, the organization's folders and projects could not all be read: HTTP 503`) {
		t.Errorf("problems = %q", res.Problems)
	}
	if len(res.Budgets) != 1 || res.Budgets[0].Target != "organization" {
		t.Errorf("budgets = %+v", res.Budgets)
	}
}

func TestUnreadableOrganization(t *testing.T) {
	e := setup(config.Budget{Name: "Shop", Project: "shop", MonthlyEUR: 1, Thresholds: []int{80}})
	e.store.Errs = map[string]error{"projects:" + org: fake.Status(403)}
	_, err := e.checker().Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "costguard cannot read the organization "+org+". Check that its service account has the costguard.reader role") ||
		!strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("err = %v", err)
	}
}

func TestCostAPIErrors(t *testing.T) {
	for code, hint := range map[int]bool{403: true, 401: true, 500: false} {
		e := setup(orgBudget(100, 80))
		e.store.Errs = map[string]error{"daily": fake.Status(code)}
		_, err := e.checker().Check(context.Background())
		if err == nil || !strings.Contains(err.Error(), "costguard cannot read the costs of the organization "+org+".") ||
			strings.Contains(err.Error(), "costguard.reader") != hint || !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", code)) {
			t.Errorf("%d: err = %v", code, err)
		}
	}
}

func TestTopListsOnlyProjectsWithSpend(t *testing.T) {
	e := setup(orgBudget(100, 80))
	e.spend(shopID, "shop", 1, 5)
	e.spend(apiID, "api", 1, 0)
	e.spend(webID, "web", 1, 5)
	e.spend(deletedID, "old", 1, 1)
	e.spend("00000000-0000-0000-0000-0000000000a4", "fourth", 1, 0.5)
	top := e.check(t).Budgets[0].Top
	if len(top) != 3 || top[0].ID != shopID || top[1].ID != webID || top[2].ID != deletedID {
		t.Errorf("top = %+v", top) // equal amounts: ordered by ID
	}
}

func TestCheckNeedsBudgetsAndAContext(t *testing.T) {
	e := setup()
	e.cfg.Budgets = nil
	if _, err := e.checker().Check(context.Background()); err == nil {
		t.Error("want error without budgets")
	}
	e = setup(config.Budget{Name: "Shop", Project: "shop", MonthlyEUR: 1, Thresholds: []int{80}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.checker().Check(ctx); err == nil {
		t.Error("want error when cancelled")
	}
	e = setup(orgBudget(1, 80))
	if _, err := e.checker().Check(ctx); err == nil {
		t.Error("want error when cancelled before the cost call")
	}
}

func TestNewUsesProductionDefaults(t *testing.T) {
	c := New(fake.New().Set(), config.Config{}, slog.Default())
	if p, ok := c.Pacer.(*scanner.FixedPacer); !ok || p.Interval != scanner.DefaultCallPacing || c.Now == nil {
		t.Errorf("checker = %+v", c)
	}
}
