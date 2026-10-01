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
	"fmt"
	"math"
	"time"
)

const (
	KindProject     = "project"
	KindNetworkArea = "networkarea"
)

var kindNames = map[string]string{
	"server":        "server",
	"volume":        "volume",
	"publicip":      "public IP",
	"snapshot":      "snapshot",
	"nic":           "network interface",
	"securitygroup": "security group",
	KindProject:     "project",
	KindNetworkArea: "network area",
}

func KindName(kind string) string {
	if n, ok := kindNames[kind]; ok {
		return n
	}
	return kind
}

func Count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

const ListLimit = 10

type Item struct {
	Kind        string
	ID          string
	Name        string
	ProjectID   string
	ProjectName string
	Region      string
	Detail      string
	New         bool
	NetworkID   string
	VolumeID    string
	SnapshotID  string
	SizeGB      int64
	MonthlyEUR  float64
	Priced      bool
}

type Savings struct {
	EUR      float64
	Unpriced int
}

func (s Savings) TotalEUR() float64 {
	return round2(s.EUR)
}

func Estimate(items []Item) Savings {
	var s Savings
	for _, it := range items {
		if it.Priced {
			s.EUR += it.MonthlyEUR
		} else {
			s.Unpriced++
		}
	}
	s.EUR = round2(s.EUR)
	return s
}

func SizeGB(items []Item) int64 {
	var n int64
	for _, it := range items {
		n += it.SizeGB
	}
	return n
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type Report struct {
	GeneratedAt    time.Time
	OrganizationID string
	Scope          string
	Regions        []string

	Blocked    []string
	ScanErrors []string

	PricesProblem string

	SkippedFolders  int
	SkippedProjects int

	IdlePublicIPs          []Item
	DetachedVolumes        []Item
	WaitingIdlePublicIPs   int
	WaitingDetachedVolumes int
	Requested              []Item
	WithSnapshots          []Item
	BackInUse              []Item

	ProtectedMarked []Item

	EmptyProjects     []Item
	EmptyNetworkAreas []Item
}

func (r *Report) ToDelete() int {
	return len(r.IdlePublicIPs) + len(r.DetachedVolumes) + len(r.Requested)
}

type Status string

const (
	StatusDeleted   Status = "deleted"
	StatusFailed    Status = "failed"
	StatusUnflagged Status = "unflagged"
	StatusDeferred  Status = "deferred"
	StatusSkipped   Status = "skipped"
)

type Result struct {
	Item        Item
	Status      Status
	Reason      string
	AlreadyGone bool
}

type DeletionSummary struct {
	GeneratedAt   time.Time
	Scope         string
	Blocked       []string
	ScanErrors    []string
	PricesProblem string
	Results       []Result
	Interrupted   bool
}

func (s *DeletionSummary) DeletedByThisRun() []Item {
	var out []Item
	for _, r := range s.Results {
		if r.Status == StatusDeleted && !r.AlreadyGone {
			out = append(out, r.Item)
		}
	}
	return out
}

func (s *DeletionSummary) Count(status Status) int {
	n := 0
	for _, r := range s.Results {
		if r.Status == status {
			n++
		}
	}
	return n
}

func (s *DeletionSummary) ByStatus(status Status) []Result {
	var out []Result
	for _, r := range s.Results {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}

func (s *DeletionSummary) HasNews() bool {
	return len(s.Results) > 0 || len(s.Blocked) > 0 || len(s.ScanErrors) > 0 || s.Interrupted
}
