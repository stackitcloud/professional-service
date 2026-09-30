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

package app

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/fake"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const budgetsOn = `budgets:
  days: [Mon, Tue, Wed, Thu, Fri]
  time: "10:00"
  limits:
    - name: Org
      organization: true
      monthlyEur: 100
      thresholds: [80, 100]
`

func setupBudgets(t *testing.T, yaml string) *env {
	t.Helper()
	e := setupReportOnly(t, budgetsOn+yaml)
	e.store.LastModified = time.Date(2026, 9, 29, 7, 41, 0, 0, time.UTC)
	e.store.Daily = map[string]stackit.ProjectDays{"p1": {Name: "shop", EUR: map[string]float64{"2026-09-10": 70}}}
	return e
}

func (e *env) spend(day string, eur float64) {
	e.store.Daily["p1"].EUR[day] += eur
}

func TestBudgetsRunPostsReachedBudgets(t *testing.T) {
	e := setupBudgets(t, "")
	e.spend("2026-09-28", 15)
	if code := e.run("budgets"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: budget (") || !strings.Contains(msgs[0], "Org: €85,00 of €100,00 in September (85%)") {
		t.Errorf("messages = %v", msgs)
	}
	if len(e.login.waits) != 1 || e.login.waits[0] != 0 {
		t.Errorf("the budgets run must not wait for the login: %v", e.login.waits)
	}
	if calls := e.store.CallsWith("daily costs"); len(calls) != 1 || calls[0] != "daily costs 2026-09-01 2026-09-28" {
		t.Errorf("cost calls = %v", calls)
	}
	if calls := append(e.store.CallsWith("label"), e.store.CallsWith("delete")...); len(calls) != 0 {
		t.Errorf("budgets must not write: %v", calls)
	}
	if !strings.Contains(e.logs.String(), `"msg":"budget","name":"Org"`) || !strings.Contains(e.logs.String(), `"reached":80`) {
		t.Errorf("budgets must be logged:\n%s", e.logs)
	}
	if code := e.run("budgets"); code != ExitOK || len(e.hook.messages()) != 2 {
		t.Errorf("second run: exit %d, messages %d", code, len(e.hook.messages()))
	}
}

func TestBudgetsRunWithoutNewsStaysQuiet(t *testing.T) {
	e := setupBudgets(t, "")
	e.spend("2026-09-28", 5)
	if code := e.run("budgets"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	if msgs := e.hook.messages(); len(msgs) != 0 {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBudgetsRunWithLateCostsUsesTheDaysIn(t *testing.T) {
	e := setupBudgets(t, "")
	e.store.LastModified = time.Date(2026, 9, 28, 7, 41, 0, 0, time.UTC)
	e.spend("2026-09-27", 15)
	if code := e.run("budgets"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Costs up to and including 27 September 2026 (UTC)") || !strings.Contains(msgs[0], "Org: €85,00 of €100,00 in September (85%)") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBudgetsRunOnTheFirstPostsNothing(t *testing.T) {
	e := setupBudgets(t, "    - name: Team B\n      folder: team-b\n      monthlyEur: 10\n      thresholds: [100]\n")
	now = func() time.Time { return time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC) }
	e.spend("2026-09-30", 500)
	if code := e.run("budgets"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	if msgs := e.hook.messages(); len(msgs) != 0 {
		t.Errorf("messages = %v", msgs)
	}
	if calls := e.store.CallsWith("daily costs"); len(calls) != 0 {
		t.Errorf("cost calls = %v", calls)
	}
	if !strings.Contains(e.logs.String(), "no cost of this month is in yet") {
		t.Errorf("logs:\n%s", e.logs)
	}
}

func TestBudgetsRunWithProblemsPostsAndFails(t *testing.T) {
	e := setupBudgets(t, "    - name: Team B\n      folder: team-b\n      monthlyEur: 10\n      thresholds: [100]\n")
	if code := e.run("budgets"); code != ExitFatal {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "1 budget could not be checked; details at the end.") || !strings.Contains(msgs[0], `folder \"team-b\" matches nothing`) {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBudgetsRunNeedsBudgets(t *testing.T) {
	e := setupReportOnly(t, "")
	if code := e.run("budgets"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: budgets run failed") || !strings.Contains(msgs[0], "only works with budgets on") {
		t.Errorf("messages = %v", msgs)
	}
	if len(e.store.Calls) != 0 || len(e.login.waits) != 0 {
		t.Errorf("nothing may happen: calls %v, login %v", e.store.Calls, e.login.waits)
	}
}

func TestBudgetsRunCostErrorPostsFailure(t *testing.T) {
	e := setupBudgets(t, "")
	e.store.Errs = map[string]error{"daily": fake.Status(http.StatusForbidden)}
	if code := e.run("budgets"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: budgets run failed") || !strings.Contains(msgs[0], "costguard.reader") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBudgetsDeliveryFailureIsFatal(t *testing.T) {
	e := setupBudgets(t, "")
	e.spend("2026-09-28", 15)
	e.hook.status = http.StatusBadRequest
	if code := e.run("budgets"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
}

func TestBootShowsTheBudgets(t *testing.T) {
	e := setupBudgets(t, "")
	e.addCandidates()
	if code := e.run("boot"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: test is running") || !strings.Contains(msgs[0], "Detached volumes: 1") ||
		!strings.Contains(msgs[0], "Budgets for September 2026 (UTC days): 1") || !strings.Contains(msgs[0], "Org · organization · €70,00 of €100,00 so far (70%)") ||
		!strings.Contains(msgs[0], "Next runs: report Monday 08:00 (Europe/Berlin) · budgets Monday to Friday 10:00 (Europe/Berlin).") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBootWithReportOffScansNoResources(t *testing.T) {
	e := setupWith(t, "reportEnabled: false\n"+budgetsOn)
	e.store.Daily = map[string]stackit.ProjectDays{}
	e.addCandidates()
	if code := e.run("boot"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "The cleanup report is off. Next runs: budgets Monday to Friday 10:00 (Europe/Berlin).") ||
		strings.Contains(msgs[0], "Detached volumes") {
		t.Errorf("messages = %v", msgs)
	}
	if calls := e.store.CallsWith("list projects"); len(calls) != 0 {
		t.Errorf("no cleanup scan with report off: %v", calls)
	}
}

func TestBootPostsWhenTheBudgetsFail(t *testing.T) {
	e := setupBudgets(t, "")
	e.store.Errs = map[string]error{"daily": fake.Status(http.StatusInternalServerError)}
	if code := e.run("boot"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: test is running") || !strings.Contains(msgs[0], "The budgets could not be checked: costguard cannot read the costs") {
		t.Errorf("messages = %v", msgs)
	}
	e = setupBudgets(t, "    - name: Team B\n      folder: team-b\n      monthlyEur: 10\n      thresholds: [100]\n")
	if code := e.run("boot"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "Budgets not checked: 1") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestBootScanFailureStillFails(t *testing.T) {
	e := setupBudgets(t, "")
	e.store.Errs = map[string]error{"projects:" + org: fake.Status(http.StatusForbidden)}
	if code := e.run("boot"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "costguard: boot run failed") {
		t.Errorf("messages = %v", msgs)
	}
}
