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

type BudgetCheck struct {
	Month time.Time
	Checked time.Time
	LastModified time.Time
	Budgets []BudgetStatus
	Problems    []string
	GeneratedAt time.Time
}

func (c *BudgetCheck) NothingIn() bool {
	return c.Checked.Before(c.Month)
}

func (c *BudgetCheck) Reached() []BudgetStatus {
	var out []BudgetStatus
	for _, b := range c.Budgets {
		if b.Reached > 0 {
			out = append(out, b)
		}
	}
	return out
}

type BudgetStatus struct {
	Name string
	Target string
	ProjectID  string
	LimitEUR   float64
	Thresholds []int
	MonthEUR float64
	DayEUR   float64
	Reached int
	ForecastEUR float64
	Top []ProjectSpend
}

type ProjectSpend struct {
	ID   string
	Name string
	EUR  float64
}

func (b BudgetStatus) Percent() float64 {
	return b.MonthEUR / b.LimitEUR * 100
}

func ThresholdEUR(limitEUR float64, percent int) float64 {
	return limitEUR * float64(percent) / 100
}
