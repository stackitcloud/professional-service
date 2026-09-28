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

// Package report holds what a run found and did, in the shape the
// notifiers render. It has no behaviour beyond small helpers.
package report

import (
	"math"
	"time"
)

// Kinds of items that are not IaaS resources.
const (
	KindProject     = "project"
	KindNetworkArea = "networkarea"
)

// ListLimit caps the new cleanup candidates one run flags per category, and
// the entries of the report-only lists. Everything already marked
// delete=true is always listed in full: nothing is deleted that was not in
// a message.
const ListLimit = 10

// Item is one resource, project or network area in a message.
type Item struct {
	// Kind is a stackit.Kind value or KindProject / KindNetworkArea.
	Kind        string
	ID          string
	Name        string
	ProjectID   string
	ProjectName string
	Region      string
	// Detail is a short extra, such as "50 GB" or "3 snapshots".
	Detail string
	// New marks cleanup candidates that this run flags for the first time.
	New bool
	// NetworkID is set for NICs (their portal page needs it).
	NetworkID string
	// SizeGB is set for volumes and snapshots.
	SizeGB int64
}

// Prices are the monthly prices behind the savings estimate.
type Prices struct {
	PublicIPMonthlyEUR float64
	VolumeGBMonthlyEUR float64
}

// Savings estimates what a set of resources costs per month, and so what
// deleting them saves. Only public IPs and volumes have a price; other kinds
// are left out of the estimate.
type Savings struct {
	IdleIPs    int
	IdleIPsEUR float64
	Volumes    int
	VolumesGB  int64
	VolumesEUR float64
}

// TotalEUR is the combined monthly estimate.
func (s Savings) TotalEUR() float64 {
	return round2(s.IdleIPsEUR + s.VolumesEUR)
}

// Estimate prices the public IPs and volumes among the items.
func Estimate(items []Item, p Prices) Savings {
	var s Savings
	for _, it := range items {
		switch it.Kind {
		case "publicip":
			s.IdleIPs++
		case "volume":
			s.Volumes++
			s.VolumesGB += it.SizeGB
		}
	}
	s.IdleIPsEUR = round2(float64(s.IdleIPs) * p.PublicIPMonthlyEUR)
	s.VolumesEUR = round2(float64(s.VolumesGB) * p.VolumeGBMonthlyEUR)
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Report is the result of a scan.
type Report struct {
	GeneratedAt    time.Time
	OrganizationID string
	// Scope describes what was scanned, e.g. "whole organization".
	Scope   string
	Regions []string

	// Blocked lists skip entries that match nothing. While it is not
	// empty, nothing is flagged or deleted.
	Blocked []string
	// ScanErrors lists everything that could not be read. The report may
	// be incomplete, but nothing unreadable is ever flagged or deleted.
	ScanErrors []string

	SkippedFolders  int
	SkippedProjects int

	// IdlePublicIPs and DetachedVolumes are the cleanup categories: unused
	// ones already labelled delete=true (all of them), then the new
	// candidates this run flags (at most ListLimit).
	IdlePublicIPs   []Item
	DetachedVolumes []Item
	// WaitingIdlePublicIPs and WaitingDetachedVolumes count the new
	// candidates beyond ListLimit. They are not flagged in this run and come
	// up in the next runs.
	WaitingIdlePublicIPs   int
	WaitingDetachedVolumes int
	// Requested lists every other resource labelled delete=true.
	Requested []Item
	// WithSnapshots lists detached volumes that are not flagged because
	// they have snapshots.
	WithSnapshots []Item
	// BackInUse lists volumes and public IPs labelled delete=true that are
	// in use again; their label is removed.
	BackInUse []Item

	// ProtectedMarked lists resources that carry both do-not-delete and
	// delete=true. do-not-delete wins; the stale delete label would turn
	// into a deletion the moment the protection is lifted. The flag run
	// removes it from volumes and public IPs; other kinds need a person.
	ProtectedMarked []Item

	EmptyProjects     []Item
	EmptyNetworkAreas []Item
}

// ToDelete counts the resources the next delete run would delete.
func (r *Report) ToDelete() int {
	return len(r.IdlePublicIPs) + len(r.DetachedVolumes) + len(r.Requested)
}

// Status is the outcome for one resource in a delete run.
type Status string

// Outcomes of a delete run.
const (
	// StatusDeleted: gone (also when it was already gone).
	StatusDeleted Status = "deleted"
	// StatusFailed: the delete call failed; the label stays and the next
	// run tries again.
	StatusFailed Status = "failed"
	// StatusUnflagged: a volume or public IP is in use again, so its
	// delete label was removed.
	StatusUnflagged Status = "unflagged"
	// StatusDeferred: not deleted in this run, the label stays (e.g. still
	// attached to a server that is being deleted).
	StatusDeferred Status = "deferred"
	// StatusSkipped: protected or no longer labelled when re-checked.
	StatusSkipped Status = "skipped"
)

// Result is what happened to one resource.
type Result struct {
	Item   Item
	Status Status
	Reason string
	// AlreadyGone marks a StatusDeleted resource that was gone before this
	// run touched it; it does not count towards the run's savings.
	AlreadyGone bool
}

// DeletionSummary is the result of a delete run.
type DeletionSummary struct {
	GeneratedAt time.Time
	Scope       string
	Blocked     []string
	ScanErrors  []string
	Results     []Result
	// Interrupted means the run was stopped (timeout or SIGTERM) before it
	// went through everything; the rest waits for the next run.
	Interrupted bool
}

// DeletedByThisRun returns the items this run deleted itself.
func (s *DeletionSummary) DeletedByThisRun() []Item {
	var out []Item
	for _, r := range s.Results {
		if r.Status == StatusDeleted && !r.AlreadyGone {
			out = append(out, r.Item)
		}
	}
	return out
}

// Count returns the number of results with the status.
func (s *DeletionSummary) Count(status Status) int {
	n := 0
	for _, r := range s.Results {
		if r.Status == status {
			n++
		}
	}
	return n
}

// ByStatus returns the results with the status, in run order.
func (s *DeletionSummary) ByStatus(status Status) []Result {
	var out []Result
	for _, r := range s.Results {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}

// HasNews reports whether the delete run has anything to tell: it only
// posts when something happened or went wrong.
func (s *DeletionSummary) HasNews() bool {
	return len(s.Results) > 0 || len(s.Blocked) > 0 || len(s.ScanErrors) > 0 || s.Interrupted
}
