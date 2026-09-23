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

func TestEstimateMonthlySavingsKnownInputs(t *testing.T) {
	volumes := []DetachedVolume{
		{ID: "v1", SizeGB: 100, EstimatedMonthlyCostEUR: 6.19},
		{ID: "v2", SizeGB: 50, EstimatedMonthlyCostEUR: 3.10},
	}
	ips := []IdlePublicIP{{ID: "i1"}, {ID: "i2"}}
	monthly, daily := EstimateMonthlySavings(volumes, ips, 4.82)
	// 6.19 + 3.10 + 2 x 4.82 = 18.93
	if monthly != 18.93 {
		t.Errorf("monthly = %v, want 18.93", monthly)
	}
	if daily != 0.63 { // 18.93 / 30 = 0.631 -> 0.63
		t.Errorf("daily = %v, want 0.63", daily)
	}
}

func TestEstimateMonthlySavingsEmpty(t *testing.T) {
	monthly, daily := EstimateMonthlySavings(nil, nil, 4.82)
	if monthly != 0 || daily != 0 {
		t.Errorf("empty savings = (%v, %v), want (0, 0)", monthly, daily)
	}
}

func TestFillSavings(t *testing.T) {
	r := &Report{
		DetachedVolumes: []DetachedVolume{{SizeGB: 10, EstimatedMonthlyCostEUR: 0.62}},
		IdlePublicIPs:   []IdlePublicIP{{ID: "i"}},
	}
	r.FillSavings(4.82)
	if r.EstimatedMonthlySavingsEUR != 5.44 { // 0.62 + 4.82
		t.Errorf("monthly = %v, want 5.44", r.EstimatedMonthlySavingsEUR)
	}
	if r.EstimatedDailySavingsEUR != 0.18 { // 5.44/30 = 0.1813 -> 0.18
		t.Errorf("daily = %v, want 0.18", r.EstimatedDailySavingsEUR)
	}
}

func TestHasFindings(t *testing.T) {
	cases := []struct {
		name string
		rep  Report
		want bool
	}{
		{"empty", Report{}, false},
		{"stale project", Report{StaleProjects: []StaleProject{{ID: "p"}}}, true},
		{"empty sna", Report{EmptySNAs: []EmptySNA{{ID: "s"}}}, true},
		{"idle ip", Report{IdlePublicIPs: []IdlePublicIP{{ID: "i"}}}, true},
		{"detached volume", Report{DetachedVolumes: []DetachedVolume{{ID: "v"}}}, true},
		{"anomaly", Report{AnomalyDetected: true}, true},
		{"top projects", Report{TopProjects: []ProjectCost{{ProjectID: "p"}}}, true},
		{"chart", Report{ChartURL: "https://x/y.png"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rep.HasFindings(); got != tc.want {
				t.Errorf("HasFindings = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDeletionSummaryCountsAndGrouping(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	s := &DeletionSummary{
		GeneratedAt: now,
		Results: []DeletionResult{
			{Item: DeletionItem{Kind: "volume", ID: "v1"}, Status: StatusDeleted},
			{Item: DeletionItem{Kind: "publicip", ID: "i1"}, Status: StatusFailed, Reason: "boom"},
			{Item: DeletionItem{Kind: "publicip", ID: "i2"}, Status: StatusSkipped, Reason: "protected"},
			{Item: DeletionItem{Kind: "volume", ID: "v2"}, Status: StatusDeleted},
		},
	}
	deleted, failed, skipped := s.Counts()
	if deleted != 2 || failed != 1 || skipped != 1 {
		t.Errorf("counts = (%d,%d,%d), want (2,1,1)", deleted, failed, skipped)
	}
	if got := s.ResultsByStatus(StatusDeleted); len(got) != 2 {
		t.Errorf("deleted group = %d, want 2", len(got))
	}
	if got := s.ResultsByStatus(StatusFailed); len(got) != 1 || got[0].Reason != "boom" {
		t.Errorf("failed group = %+v", got)
	}
	if got := s.ResultsByStatus("nope"); len(got) != 0 {
		t.Errorf("unknown status group = %+v", got)
	}
}

func TestDeletionSummaryEmpty(t *testing.T) {
	s := &DeletionSummary{}
	if d, f, k := s.Counts(); d != 0 || f != 0 || k != 0 {
		t.Errorf("empty counts = (%d,%d,%d)", d, f, k)
	}
}
