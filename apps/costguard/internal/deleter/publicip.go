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

// deletePublicIP re-validates the IP (fail-closed) and deletes it.
func (d *Deleter) deletePublicIP(ctx context.Context, ip report.IdlePublicIP) report.DeletionResult {
	item := report.DeletionItem{
		Kind:        "publicip",
		Name:        ip.Address,
		ID:          ip.ID,
		ProjectID:   ip.ProjectID,
		ProjectName: ip.ProjectName,
		Region:      ip.Region,
	}

	current, err := d.IaaS.GetPublicIP(ctx, ip.ProjectID, ip.Region, ip.ID)
	if err != nil {
		if isNotFound(err) {
			d.Logger.Info("public IP already gone", "id", ip.ID, "project", ip.ProjectName, "region", ip.Region)
			return report.DeletionResult{Item: item, Status: report.StatusDeleted, Reason: "already gone (404)"}
		}
		return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: "re-validation read: " + err.Error()}
	}
	if reason := d.publicIPStillCandidate(*current); reason != "" {
		d.Logger.Warn("skipping public IP: no longer a deletion target",
			"id", ip.ID, "project", ip.ProjectName, "reason", reason)
		return report.DeletionResult{Item: item, Status: report.StatusSkipped, Reason: reason}
	}

	d.logDeletion("public IP", item)
	if err := d.deleteWithRetry(ctx, func(ctx context.Context) error {
		return d.IaaS.DeletePublicIP(ctx, ip.ProjectID, ip.Region, ip.ID)
	}); err != nil {
		d.Logger.Error("deleting public IP failed", "id", ip.ID, "project", ip.ProjectName, "error", err)
		return report.DeletionResult{Item: item, Status: report.StatusFailed, Reason: err.Error()}
	}
	d.Logger.Info("deleted public IP", "id", ip.ID, "name", item.Name, "project", ip.ProjectName, "region", ip.Region)
	return report.DeletionResult{Item: item, Status: report.StatusDeleted}
}

// publicIPStillCandidate returns "" when the freshly-read IP is still a
// valid deletion target, otherwise the skip reason. Every check is
// fail-closed: a missing or stale mark never deletes.
func (d *Deleter) publicIPStillCandidate(ip stackit.PublicIP) string {
	if d.Whitelist.PublicIPProtected(ip.ID) {
		return "whitelisted"
	}
	if d.safeLabeled(ip.Labels) {
		return "safe label present"
	}
	if ip.AttachedNIC != "" {
		return "attached to a network interface"
	}
	deadline, ok := stackit.MarkDeadline(ip.Labels)
	if !ok {
		return "mark label missing or malformed"
	}
	if deadline.After(d.Now()) {
		return "mark deadline not reached (" + deadlineString(deadline) + ")"
	}
	return ""
}
