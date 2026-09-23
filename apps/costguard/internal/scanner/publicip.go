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

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// idlePublicIPs lists the idle public IPs of one project/region and
// maintains the mark label: an idle IP without a mark gets the
// configured grace period stamped on it; an IP that is no longer idle has
// its mark removed (self-heal). Whitelisted IPs never become candidates.
func (s *Scanner) idlePublicIPs(ctx context.Context, p scannedProject, region string, addError func(string, ...any)) []report.IdlePublicIP {
	if s.Pacer.Pace(ctx) != nil {
		return nil
	}
	ips, err := s.Clients.IaaS.ListPublicIPs(ctx, p.ID, region)
	if err != nil {
		addError("public IP scan %s/%s: %v", p.Name, region, err)
		return nil
	}
	out := make([]report.IdlePublicIP, 0)
	for _, ip := range ips {
		if s.Whitelist.PublicIPProtected(ip.ID) {
			continue
		}
		deadline, hasMark := stackit.MarkDeadline(ip.Labels)
		candidate := ip.AttachedNIC == "" && !s.hasSafeLabel(ip.Labels)

		switch {
		case candidate && !hasMark:
			// Stamp a fresh mark. A failed write is non-fatal for the
			// report (the candidate is still shown) but is surfaced; the
			// delete run re-validates the mark and stays fail-closed.
			newDeadline := s.Now().Add(s.Config.GracePeriod)
			if merr := s.Clients.IaaS.SetPublicIPMark(ctx, p.ID, region, ip.ID, stackit.FormatMark(newDeadline)); merr != nil {
				addError("marking public IP %s (%s/%s): %v", ip.ID, p.Name, region, merr)
			}
			out = append(out, report.IdlePublicIP{
				ID:           ip.ID,
				ProjectID:    p.ID,
				ProjectName:  p.Name,
				Region:       region,
				Address:      ip.Address,
				MarkDeadline: &newDeadline,
			})
		case candidate && hasMark:
			// Keep the existing deadline — never extend the grace period
			// (the clock is not reset by re-scanning).
			out = append(out, report.IdlePublicIP{
				ID:           ip.ID,
				ProjectID:    p.ID,
				ProjectName:  p.Name,
				Region:       region,
				Address:      ip.Address,
				MarkDeadline: &deadline,
			})
		case !candidate && hasMark:
			// Self-heal: the IP became attached again or gained the safe
			// label, so remove the stale mark.
			if merr := s.Clients.IaaS.ClearPublicIPMark(ctx, p.ID, region, ip.ID); merr != nil {
				addError("clearing mark on public IP %s (%s/%s): %v", ip.ID, p.Name, region, merr)
			}
		}
	}
	return out
}
