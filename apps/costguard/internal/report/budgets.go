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

import "time"

// BudgetCheck is where every budget stands in the current month (UTC days,
// like the Cost API's).
type BudgetCheck struct {
	// Month is the first day of the month checked: the month of the run.
	Month time.Time
	// Checked is the last day whose costs are in: yesterday, or an earlier
	// day when STACKIT had not updated its data yet. Before Month when no
	// cost of this month is in yet (always on the 1st).
	Checked time.Time
	// LastModified is when STACKIT last updated its cost data; zero when
	// the Cost API did not say.
	LastModified time.Time
	// Budgets lists the budgets whose target was found, in config order.
	Budgets []BudgetStatus
	// Problems lists the budgets that could not be checked and why.
	Problems    []string
	GeneratedAt time.Time
}

// NothingIn reports whether no cost of the month is in yet.
func (c *BudgetCheck) NothingIn() bool {
	return c.Checked.Before(c.Month)
}

// Reached returns the budgets at or above one of their thresholds.
func (c *BudgetCheck) Reached() []BudgetStatus {
	var out []BudgetStatus
	for _, b := range c.Budgets {
		if b.Reached > 0 {
			out = append(out, b)
		}
	}
	return out
}

// BudgetStatus is one budget in the month.
type BudgetStatus struct {
	Name string
	// Target says what the budget covers: "organization", "folder team-a"
	// or "project shop".
	Target string
	// ProjectID is set for project budgets (for the portal link).
	ProjectID  string
	LimitEUR   float64
	Thresholds []int
	// MonthEUR is the month to date up to and including the checked day,
	// DayEUR the spend of that day.
	MonthEUR float64
	DayEUR   float64
	// Reached is the highest threshold (percent of LimitEUR) the month to
	// date is at or above; 0 if none.
	Reached int
	// ForecastEUR extends the month to date in a straight line to the end
	// of the month; 0 before the 7th, when it would mostly be noise.
	ForecastEUR float64
	// Top lists the projects with the highest spend this month, at most
	// three (organization and folder budgets only).
	Top []ProjectSpend
}

// ProjectSpend is one project's month to date. Deleted projects keep
// their charges (and name) in the Cost API.
type ProjectSpend struct {
	ID   string
	Name string
	EUR  float64
}

// Percent is MonthEUR as a percentage of the limit.
func (b BudgetStatus) Percent() float64 {
	return b.MonthEUR / b.LimitEUR * 100
}

// NextThreshold returns the lowest threshold not reached yet, or 0.
func (b BudgetStatus) NextThreshold() int {
	for _, t := range b.Thresholds {
		if b.MonthEUR < ThresholdEUR(b.LimitEUR, t) {
			return t
		}
	}
	return 0
}

// ThresholdEUR is the amount at which a threshold is reached.
func ThresholdEUR(limitEUR float64, percent int) float64 {
	return limitEUR * float64(percent) / 100
}
