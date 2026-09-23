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

package deleter

import (
	"context"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// volumeStatusAvailable is the IaaS status of a detached volume.
const volumeStatusAvailable = "AVAILABLE"

// deleteVolume re-validates the volume (fail-closed), removes all of
// its snapshots first, then deletes the volume.
func (d *Deleter) deleteVolume(ctx context.Context, v report.DetachedVolume) report.DeletionResult {
	item := report.DeletionItem{
		Kind:        "volume",
		Name:        v.Name,
		ID:          v.ID,
		ProjectID:   v.ProjectID,
		ProjectName: v.ProjectName,
		Region:      v.Region,
	}

	current, err := d.IaaS.GetVolume(ctx, v.ProjectID, v.Region, v.ID)
	if err != nil {
		if isNotFound(err) {
			d.Logger.Info("volume already gone", "id", v.ID, "project", v.ProjectName, "region", v.Region)
			return report.DeletionResult{Item: item, Status: report.StatusDeleted, Reason: "already gone (404)"}
		}
		return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: "re-validation read: " + err.Error()}
	}
	if reason := d.volumeStillCandidate(*current); reason != "" {
		d.Logger.Warn("skipping volume: no longer a deletion target",
			"id", v.ID, "project", v.ProjectName, "reason", reason)
		return report.DeletionResult{Item: item, Status: report.StatusSkipped, Reason: reason}
	}

	// all snapshots of the volume must be deleted first. A
	// snapshot failure blocks the volume delete (fail-closed — never drop
	// a volume that still has snapshots; the API would 409 anyway).
	snaps, err := d.IaaS.ListSnapshots(ctx, v.ProjectID, v.Region)
	if err != nil {
		return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: "listing snapshots: " + err.Error()}
	}
	for _, s := range snaps {
		if s.VolumeID != v.ID {
			continue
		}
		d.Logger.Info("deleting volume snapshot first",
			"snapshot", s.ID, "volume", v.ID, "project", v.ProjectName, "region", v.Region)
		if err := d.deleteWithRetry(ctx, func(ctx context.Context) error {
			return d.IaaS.DeleteSnapshot(ctx, v.ProjectID, v.Region, s.ID)
		}); err != nil {
			d.Logger.Error("deleting snapshot failed, volume kept", "snapshot", s.ID, "volume", v.ID, "error", err)
			return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: "deleting snapshot " + s.ID + ": " + err.Error()}
		}
	}

	d.logDeletion("volume", item)
	if err := d.deleteWithRetry(ctx, func(ctx context.Context) error {
		return d.IaaS.DeleteVolume(ctx, v.ProjectID, v.Region, v.ID)
	}); err != nil {
		d.Logger.Error("deleting volume failed", "id", v.ID, "project", v.ProjectName, "error", err)
		return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: err.Error()}
	}
	d.Logger.Info("deleted volume", "id", v.ID, "name", item.Name, "project", v.ProjectName, "region", v.Region)
	return report.DeletionResult{Item: item, Status: report.StatusDeleted}
}

// volumeStillCandidate returns "" when the freshly-read volume is still a
// valid deletion target, otherwise the skip reason (fail-closed).
func (d *Deleter) volumeStillCandidate(v stackit.Volume) string {
	if d.Whitelist.VolumeProtected(v.ID) {
		return "whitelisted"
	}
	if d.safeLabeled(v.Labels) {
		return "safe label present"
	}
	if v.ServerID != "" {
		return "attached to a server"
	}
	if v.Status != volumeStatusAvailable {
		return "not detached (status " + v.Status + ")"
	}
	deadline, ok := stackit.MarkDeadline(v.Labels)
	if !ok {
		return "mark label missing or malformed"
	}
	if deadline.After(d.Now()) {
		return "mark deadline not reached (" + deadlineString(deadline) + ")"
	}
	return ""
}
