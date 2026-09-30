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

package report

import (
	"testing"
	"time"
)

func TestBudgetStatusHelpers(t *testing.T) {
	b := BudgetStatus{Name: "A", LimitEUR: 200, Thresholds: []int{50, 80, 100}, MonthEUR: 160}
	if b.Percent() != 80 || b.NextThreshold() != 100 || ThresholdEUR(200, 80) != 160 {
		t.Errorf("percent %v, next %d", b.Percent(), b.NextThreshold())
	}
	b.MonthEUR = 250
	if b.NextThreshold() != 0 {
		t.Errorf("every threshold passed: next %d", b.NextThreshold())
	}
	chk := BudgetCheck{Budgets: []BudgetStatus{b, {Name: "B", Reached: 80}, {Name: "C"}}}
	if got := chk.Reached(); len(got) != 1 || got[0].Name != "B" {
		t.Errorf("Reached = %+v", got)
	}
}

func TestBudgetCheckNothingIn(t *testing.T) {
	oct := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if !(&BudgetCheck{Month: oct, Checked: oct.AddDate(0, 0, -1)}).NothingIn() || (&BudgetCheck{Month: oct, Checked: oct}).NothingIn() {
		t.Error("NothingIn is wrong")
	}
}
