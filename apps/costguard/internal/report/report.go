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

// Package report defines the single shared shape every output adapter
// consumes: one Report struct plus the post-deletion
// DeletionSummary. Adapters never see scanner internals.
package report

import (
	"time"
)

// StaleProject is a hygiene-section candidate (reported
// but never auto-deleted in v1). AgeDays gives reviewers context for
// borderline cases.
type StaleProject struct {
	ID               string
	Name             string
	AgeDays          int
	CreatedAt        time.Time
	ParentFolderID   string
	ParentFolderName string
	SKEClusters      []string
	StorageBuckets   []string
	ServerCount      int
}

// EmptySNA is a hygiene-section candidate (reported but
// never auto-deleted in v1).
type EmptySNA struct {
	ID        string
	Name      string
	AgeDays   int
	CreatedAt time.Time
}

// IdlePublicIP is a v1 auto-deletion candidate.
// MarkDeadline is nil when the mark label is absent or unreadable.
type IdlePublicIP struct {
	ID           string
	ProjectID    string
	ProjectName  string
	Region       string
	Address      string
	MarkDeadline *time.Time
}

// DetachedVolume is a v1 auto-deletion candidate.
// EstimatedMonthlyCostEUR = SizeGB x configured per-GB rate.
type DetachedVolume struct {
	ID                      string
	Name                    string
	ProjectID               string
	ProjectName             string
	Region                  string
	SizeGB                  int64
	EstimatedMonthlyCostEUR float64
	MarkDeadline            *time.Time
}

// ProjectCost is one entry of the top-10 most expensive projects
//, ranked by 30-day spend.
type ProjectCost struct {
	ProjectID   string
	ProjectName string
	Cost30dEUR  float64
}

// DailyCost is one day of the 30-day cost series.
type DailyCost struct {
	Date    string
	CostEUR float64
}

// Report is the complete scan result shared by all output adapters.
type Report struct {
	GeneratedAt time.Time
	DryRun      bool

	Scope   string
	OrgID   string
	Regions []string

	// Hygiene section (report-only in v1).
	StaleProjects []StaleProject
	EmptySNAs     []EmptySNA

	// Deletion candidates.
	IdlePublicIPs   []IdlePublicIP
	DetachedVolumes []DetachedVolume

	// Cost insights.
	TopProjects      []ProjectCost
	AnomalyDetected  bool
	AnomalyPct       float64
	DailyCosts       []DailyCost
	TotalCost30dEUR  float64
	AvgCostPerDayEUR float64

	// Estimated savings if all candidates were deleted.
	EstimatedMonthlySavingsEUR float64
	EstimatedDailySavingsEUR   float64

	// Presigned chart URL; empty when S3 is not configured.
	ChartURL string

	// Non-fatal scanner errors, one entry per failing scanner/project
	// — surfaced so a silent partial scan is visible.
	ScanErrors []string
}

// HasFindings reports whether the report contains anything worth sending.
func (r *Report) HasFindings() bool {
	return len(r.StaleProjects) > 0 ||
		len(r.EmptySNAs) > 0 ||
		len(r.IdlePublicIPs) > 0 ||
		len(r.DetachedVolumes) > 0 ||
		r.AnomalyDetected ||
		len(r.TopProjects) > 0 ||
		r.ChartURL != ""
}

// DeletionItem identifies one resource the delete run acted on (or
// skipped), with enough context for the confirmation and the audit logs
// (name/ID/project/region).
type DeletionItem struct {
	Kind        string // "publicip" or "volume"
	Name        string
	ID          string
	ProjectID   string
	ProjectName string
	Region      string
}

// DeletionResult is the outcome of one deletion attempt.
type DeletionResult struct {
	Item   DeletionItem
	Status string // StatusDeleted, StatusFailed or StatusSkipped
	Reason string
}

// Result status values.
const (
	StatusDeleted = "deleted"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// DeletionSummary is the separate post-deletion confirmation.
// It is sent after every execution run — even when all deletions failed.
type DeletionSummary struct {
	GeneratedAt time.Time
	Results     []DeletionResult
}

// Counts tallies the summary by status.
func (s *DeletionSummary) Counts() (deleted, failed, skipped int) {
	for _, r := range s.Results {
		switch r.Status {
		case StatusDeleted:
			deleted++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		}
	}
	return deleted, failed, skipped
}

// ResultsByStatus returns the results grouped by status, in original
// order.
func (s *DeletionSummary) ResultsByStatus(status string) []DeletionResult {
	out := make([]DeletionResult, 0)
	for _, r := range s.Results {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}
