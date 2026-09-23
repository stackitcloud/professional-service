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
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// emptySNAs returns the network areas that are hygiene candidates
// (report-only in v1 — an "empty" SNA can still host
// billing VPN gateways/LBs, so auto-deletion is deferred).
func (s *Scanner) emptySNAs(ctx context.Context, addError func(string, ...any)) []report.EmptySNA {
	if s.Pacer.Pace(ctx) != nil {
		return nil
	}
	areas, err := s.Clients.IaaS.ListNetworkAreas(ctx, s.Config.OrgID)
	if err != nil {
		addError("SNA scan: %v", err)
		return nil
	}
	maxAge := time.Duration(s.Config.SNAMaxAgeDays) * 24 * time.Hour
	out := make([]report.EmptySNA, 0)
	for _, area := range areas {
		if area.ProjectCount != 0 {
			continue
		}
		if area.CreatedAt.IsZero() {
			continue
		}
		if s.Now().Sub(area.CreatedAt) <= maxAge {
			// Boundary: exactly SNAMaxAgeDays old is NOT a candidate.
			continue
		}
		if s.hasSafeLabel(area.Labels) {
			continue
		}
		out = append(out, report.EmptySNA{
			ID:        area.ID,
			Name:      area.Name,
			AgeDays:   s.ageDays(area.CreatedAt),
			CreatedAt: area.CreatedAt,
		})
	}
	return out
}
