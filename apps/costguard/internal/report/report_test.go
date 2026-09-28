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

import "testing"

func TestReportToDelete(t *testing.T) {
	r := &Report{
		IdlePublicIPs:   []Item{{ID: "a"}},
		DetachedVolumes: []Item{{ID: "b"}, {ID: "c"}},
		Requested:       []Item{{ID: "d"}},
		WithSnapshots:   []Item{{ID: "e"}},
	}
	if r.ToDelete() != 4 {
		t.Errorf("ToDelete = %d", r.ToDelete())
	}
}

func TestEstimate(t *testing.T) {
	items := []Item{
		{Kind: "publicip"}, {Kind: "publicip"},
		{Kind: "volume", SizeGB: 100}, {Kind: "volume", SizeGB: 33},
		{Kind: "server"}, {Kind: "snapshot", SizeGB: 10},
	}
	got := Estimate(items, Prices{PublicIPMonthlyEUR: 4.82, VolumeGBMonthlyEUR: 0.0619})
	want := Savings{IdleIPs: 2, IdleIPsEUR: 9.64, Volumes: 2, VolumesGB: 133, VolumesEUR: 8.23}
	if got != want {
		t.Errorf("Estimate = %+v, want %+v", got, want)
	}
	if got.TotalEUR() != 17.87 {
		t.Errorf("TotalEUR = %v", got.TotalEUR())
	}
	if (Estimate(nil, Prices{})) != (Savings{}) {
		t.Error("no items, no savings")
	}
}

func TestDeletionSummary(t *testing.T) {
	s := &DeletionSummary{}
	if s.HasNews() {
		t.Error("empty summary has no news")
	}
	s.Results = []Result{
		{Item: Item{ID: "1"}, Status: StatusDeleted},
		{Item: Item{ID: "2"}, Status: StatusFailed},
		{Item: Item{ID: "3"}, Status: StatusDeleted},
	}
	if s.Count(StatusDeleted) != 2 || s.Count(StatusSkipped) != 0 {
		t.Errorf("counts wrong")
	}
	if got := s.ByStatus(StatusDeleted); len(got) != 2 || got[1].Item.ID != "3" {
		t.Errorf("ByStatus = %+v", got)
	}
	if !s.HasNews() {
		t.Error("results are news")
	}
	s.Results = append(s.Results, Result{Item: Item{ID: "4"}, Status: StatusDeleted, AlreadyGone: true})
	if got := s.DeletedByThisRun(); len(got) != 2 || got[0].ID != "1" || got[1].ID != "3" {
		t.Errorf("DeletedByThisRun = %+v", got)
	}
	if !(&DeletionSummary{Blocked: []string{"x"}}).HasNews() || !(&DeletionSummary{ScanErrors: []string{"x"}}).HasNews() {
		t.Error("blocked runs and scan errors are news")
	}
	if !(&DeletionSummary{Interrupted: true}).HasNews() {
		t.Error("an interrupted run is news")
	}
}
