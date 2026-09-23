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

package notifier

import (
	"fmt"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// reportCycleTTL bounds the protect button of a report-only candidate
// (stale project). Projects are never deleted in v1, so their
// button only affects the *next* report — one weekly reporting cycle of
// validity is enough, after which a fresh report ships fresh buttons.
const reportCycleTTL = 7 * 24 * time.Hour

// FolderGroup is one group of stale projects under the same parent
// folder ("stale projects grouped by parent folder").
type FolderGroup struct {
	FolderID   string
	FolderName string
	Projects   []report.StaleProject
}

// GroupProjectsByFolder groups stale projects by parent folder,
// preserving report order (first-seen order of folders and projects).
func GroupProjectsByFolder(projects []report.StaleProject) []FolderGroup {
	var groups []FolderGroup
	idx := map[string]int{}
	for _, p := range projects {
		key := p.ParentFolderID + "/" + p.ParentFolderName
		i, ok := idx[key]
		if !ok {
			groups = append(groups, FolderGroup{FolderID: p.ParentFolderID, FolderName: p.ParentFolderName})
			i = len(groups) - 1
			idx[key] = i
		}
		groups[i].Projects = append(groups[i].Projects, p)
	}
	return groups
}

// FormatEUR renders a euro amount with two decimals ("€6.19").
func FormatEUR(v float64) string {
	return fmt.Sprintf("€%.2f", v)
}

// FormatDeadline renders a mark deadline the way it appears in reports:
// "2026-09-22 20:00 UTC".
func FormatDeadline(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// ButtonDeadline picks the protect-button expiry ("Button URLs
// are signed and expire at the candidate's deletion deadline"). Deletion
// candidates use their mark deadline; report-only stale projects have no
// deletion deadline, so their buttons live for one reporting cycle.
func ButtonDeadline(generatedAt time.Time, candidateDeadline *time.Time) time.Time {
	if candidateDeadline != nil && !candidateDeadline.IsZero() {
		return *candidateDeadline
	}
	if generatedAt.IsZero() {
		return time.Now().UTC().Add(reportCycleTTL)
	}
	return generatedAt.Add(reportCycleTTL)
}
