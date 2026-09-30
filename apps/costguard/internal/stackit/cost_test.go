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

package stackit

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	costv3 "github.com/stackitcloud/stackit-sdk-go/services/cost/v3api"
)

// day is one entry of reportData as the Cost API sends it (cents).
func day(date string, cents float64) map[string]any {
	return map[string]any{"charge": cents, "discount": 0.0, "quantity": 1, "quantityDecimal": "1",
		"timePeriod": map[string]any{"start": date, "end": date}}
}

func projectCosts(id, name string, total float64, days ...map[string]any) map[string]any {
	p := map[string]any{"customerAccountId": "org", "projectId": id, "projectName": name, "totalCharge": total, "totalDiscount": 0.0}
	if days != nil {
		p["reportData"] = days
	}
	return p
}

func TestCostDailyCosts(t *testing.T) {
	rt := newRouter(t)
	rt.routes["GET /v3/costs/org"] = func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("from") != "2026-09-01" || q.Get("to") != "2026-09-28" || q.Get("depth") != "project" || q.Get("granularity") != "daily" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Last-Modified", "Tue, 29 Sep 2026 07:41:12 GMT")
		_ = json.NewEncoder(w).Encode([]any{
			projectCosts("p1", "shop", 1500, day("2026-09-01", 1000), day("2026-09-28", 500)),
			projectCosts("p2", "gone", 0),
			projectCosts("", "no id", 7, day("2026-09-02", 7)),
		})
	}
	set := testSet(t, rt)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	got, err := set.Cost.DailyCosts(ctx, "org", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 9, 29, 7, 41, 12, 0, time.UTC); !got.LastModified.Equal(want) {
		t.Errorf("LastModified = %v", got.LastModified)
	}
	p1 := got.Projects["p1"]
	if p1.Name != "shop" || p1.EUR["2026-09-01"] != 10 || p1.EUR["2026-09-28"] != 5 || len(p1.EUR) != 2 {
		t.Errorf("p1 = %+v", p1)
	}
	if p2, ok := got.Projects["p2"]; !ok || len(p2.EUR) != 0 {
		t.Errorf("p2 = %+v, %v", p2, ok)
	}
	if len(got.Projects) != 2 {
		t.Errorf("projects = %v", got.Projects)
	}
	if _, err := set.Cost.DailyCosts(ctx, "nope", from, to); err == nil || !strings.Contains(err.Error(), "listing daily costs 2026-09-01 .. 2026-09-28") {
		t.Errorf("err = %v", err)
	}
}

func TestCostDailyCostsWithoutLastModified(t *testing.T) {
	rt := newRouter(t)
	rt.json("GET", "/v3/costs/org", 200, []any{projectCosts("p1", "shop", 100, day("2026-09-01", 100))})
	got, err := testSet(t, rt).Cost.DailyCosts(ctx, "org", time.Now(), time.Now())
	if err != nil || !got.LastModified.IsZero() || got.Projects["p1"].EUR["2026-09-01"] != 1 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestCostDailyCostsRefusesWhatItCannotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		body any
		want string
	}{
		"total without days": {[]any{projectCosts("p1", "shop", 100)}, "a total but no daily costs for project shop (p1)"},
		"bad day":            {[]any{projectCosts("p1", "shop", 100, day("29.09.2026", 100))}, `the day "29.09.2026" for project p1`},
		"no day":             {[]any{projectCosts("p1", "shop", 100, map[string]any{"charge": 1.0, "discount": 0.0, "quantity": 1, "quantityDecimal": "1", "timePeriod": map[string]any{}})}, `the day "" for project p1`},
		"day without charge": {[]any{projectCosts("p1", "shop", 100, map[string]any{"discount": 0.0, "quantity": 1, "quantityDecimal": "1", "timePeriod": map[string]any{"start": "2026-09-01"}})}, "reading the daily costs of project p1"},
	} {
		rt := newRouter(t)
		rt.json("GET", "/v3/costs/org", 200, tc.body)
		_, err := testSet(t, rt).Cost.DailyCosts(ctx, "org", time.Now(), time.Now())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

// The SDK normally fills ProjectCostWithDetailedServices; the other
// variants are read the same way.
func TestReportDataFromEveryVariant(t *testing.T) {
	days := []costv3.ReportData{{Charge: 250, TimePeriod: costv3.ReportDataTimePeriod{Start: ptr("2026-09-03")}}}
	extra := map[string]any{"reportData": []any{day("2026-09-03", 250)}}
	for name, p := range map[string]costv3.ProjectCost{
		"reports":    {ProjectCostWithReports: &costv3.ProjectCostWithReports{ProjectId: "p", ProjectName: "n", TotalCharge: 250, ReportData: days}},
		"summarized": {ProjectCostWithSummarizedServices: &costv3.ProjectCostWithSummarizedServices{ProjectId: "p", ProjectName: "n", TotalCharge: 250, AdditionalProperties: extra}},
		"project":    {SummarizedProjectCost: &costv3.SummarizedProjectCost{ProjectId: "p", ProjectName: "n", TotalCharge: 250, AdditionalProperties: extra}},
	} {
		pc, err := reportData(p)
		if err != nil || pc.id != "p" || pc.name != "n" || len(pc.days) != 1 || pc.days[0].Charge != 250 || pc.days[0].TimePeriod.GetStart() != "2026-09-03" {
			t.Errorf("%s: %+v, %v", name, pc, err)
		}
	}
	if pc, err := reportData(costv3.ProjectCost{}); err != nil || pc.id != "" {
		t.Errorf("empty: %+v, %v", pc, err)
	}
	bad := costv3.ProjectCost{SummarizedProjectCost: &costv3.SummarizedProjectCost{ProjectId: "p", AdditionalProperties: map[string]any{"reportData": "nope"}}}
	if _, err := reportData(bad); err == nil {
		t.Error("want error for unreadable report data")
	}
}

func ptr[T any](v T) *T { return &v }
