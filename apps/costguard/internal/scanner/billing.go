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

package scanner

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// costWindowDays is the look-back window for the daily series, the
// per-project 30-day totals and the anomaly baseline.
const costWindowDays = 30

// anomalyWindowDays is the size of each of the two windows compared by
// the anomaly check (last 7 days vs the 7 days before).
const anomalyWindowDays = 7

const billingDateLayout = "2006-01-02"

// topProjectsLimit caps the top-spending projects section.
const topProjectsLimit = 10

// dateRange returns the last days complete days before the current UTC
// day, oldest first. The current day is excluded: its billing data is
// still incomplete.
func dateRange(days int, now time.Time) []time.Time {
	u := now.UTC()
	today := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	dates := make([]time.Time, 0, days)
	for i := days; i >= 1; i-- {
		dates = append(dates, today.AddDate(0, 0, -i))
	}
	return dates
}

// fillBilling queries the 30-day window with one paced call per day (in
// parallel) and fills the cost-insight section of the report.
// Partial failures leave holes in the series; each failure is already in
// ScanErrors.
func (s *Scanner) fillBilling(ctx context.Context, rep *report.Report, addError func(string, ...any)) {
	dates := dateRange(costWindowDays, s.Now())

	var mu sync.Mutex
	dailyTotals := make([]float64, len(dates))
	projectTotals := make(map[string]float64)
	projectNames := make(map[string]string)

	jobs := make([]func(context.Context), 0, len(dates))
	for i, d := range dates {
		i, d := i, d
		jobs = append(jobs, func(ctx context.Context) {
			if s.Pacer.Pace(ctx) != nil {
				return
			}
			records, err := s.Clients.Cost.ListCostsForCustomer(ctx, s.Config.OrgID, d, d)
			if err != nil {
				addError("cost day %s: %v", d.Format(billingDateLayout), err)
				return
			}
			total := 0.0
			for _, r := range records {
				total += r.ChargeEUR
				mu.Lock()
				projectTotals[r.ProjectID] += r.ChargeEUR
				if r.ProjectName != "" {
					projectNames[r.ProjectID] = r.ProjectName
				}
				mu.Unlock()
			}
			mu.Lock()
			dailyTotals[i] = total
			mu.Unlock()
		})
	}
	runPooled(ctx, s.Workers, jobs)

	total30 := 0.0
	rep.DailyCosts = make([]report.DailyCost, 0, len(dates))
	for i, d := range dates {
		rep.DailyCosts = append(rep.DailyCosts, report.DailyCost{
			Date:    d.Format(billingDateLayout),
			CostEUR: round2(dailyTotals[i]),
		})
		total30 += dailyTotals[i]
	}
	rep.TotalCost30dEUR = round2(total30)
	rep.AvgCostPerDayEUR = round2(total30 / costWindowDays)

	rep.TopProjects = topProjects(projectTotals, projectNames, topProjectsLimit)
	rep.AnomalyDetected, rep.AnomalyPct = detectAnomaly(dailyTotals, s.Config.CostAnomalyThresholdPct)
}

// topProjects returns the top-n projects by 30-day spend, ties broken by
// name for a stable report.
func topProjects(totals map[string]float64, names map[string]string, n int) []report.ProjectCost {
	type item struct {
		id, name string
		cost     float64
	}
	list := make([]item, 0, len(totals))
	for id, cost := range totals {
		list = append(list, item{id: id, name: names[id], cost: cost})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].cost != list[j].cost {
			return list[i].cost > list[j].cost
		}
		return list[i].name < list[j].name
	})
	if len(list) > n {
		list = list[:n]
	}
	out := make([]report.ProjectCost, 0, len(list))
	for _, e := range list {
		out = append(out, report.ProjectCost{
			ProjectID:   e.id,
			ProjectName: e.name,
			Cost30dEUR:  round2(e.cost),
		})
	}
	return out
}

// detectAnomaly compares the average daily charge of the last
// anomalyWindowDays days against the window directly before it. The window
// is flagged when the increase exceeds thresholdPct. A zero baseline cannot
// be compared in percent terms, so it never flags.
func detectAnomaly(daily []float64, thresholdPct float64) (bool, float64) {
	if len(daily) < anomalyWindowDays*2 {
		return false, 0
	}
	recent := average(daily[len(daily)-anomalyWindowDays:])
	baseline := average(daily[len(daily)-2*anomalyWindowDays : len(daily)-anomalyWindowDays])
	if baseline <= 0 {
		return false, 0
	}
	pct := (recent - baseline) / baseline * 100
	return pct > thresholdPct, round2(pct)
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
