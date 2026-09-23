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
	"math"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// volumeStatusAvailable is the IaaS status of a volume that is not
// attached to any server.
const volumeStatusAvailable = "AVAILABLE"

// detachedVolumes lists the detached volumes of one project/region and
// maintains the mark label. Only AVAILABLE volumes with no
// server attachment are candidates; whitelisted or safe-labeled volumes
// are never candidates.
func (s *Scanner) detachedVolumes(ctx context.Context, p scannedProject, region string, addError func(string, ...any)) []report.DetachedVolume {
	if s.Pacer.Pace(ctx) != nil {
		return nil
	}
	vols, err := s.Clients.IaaS.ListVolumes(ctx, p.ID, region)
	if err != nil {
		addError("volume scan %s/%s: %v", p.Name, region, err)
		return nil
	}
	out := make([]report.DetachedVolume, 0)
	for _, v := range vols {
		if s.Whitelist.VolumeProtected(v.ID) {
			continue
		}
		deadline, hasMark := stackit.MarkDeadline(v.Labels)
		candidate := v.Status == volumeStatusAvailable && v.ServerID == "" && !s.hasSafeLabel(v.Labels)

		estimated := math.Round(float64(v.SizeGB)*s.Config.VolumeCostEurPerGB*100) / 100

		switch {
		case candidate && !hasMark:
			newDeadline := s.Now().Add(s.Config.GracePeriod)
			if merr := s.Clients.IaaS.SetVolumeMark(ctx, p.ID, region, v.ID, stackit.FormatMark(newDeadline)); merr != nil {
				addError("marking volume %s (%s/%s): %v", v.ID, p.Name, region, merr)
			}
			out = append(out, report.DetachedVolume{
				ID:                      v.ID,
				Name:                    v.Name,
				ProjectID:               p.ID,
				ProjectName:             p.Name,
				Region:                  region,
				SizeGB:                  v.SizeGB,
				EstimatedMonthlyCostEUR: estimated,
				MarkDeadline:            &newDeadline,
			})
		case candidate && hasMark:
			out = append(out, report.DetachedVolume{
				ID:                      v.ID,
				Name:                    v.Name,
				ProjectID:               p.ID,
				ProjectName:             p.Name,
				Region:                  region,
				SizeGB:                  v.SizeGB,
				EstimatedMonthlyCostEUR: estimated,
				MarkDeadline:            &deadline,
			})
		case !candidate && hasMark:
			if merr := s.Clients.IaaS.ClearVolumeMark(ctx, p.ID, region, v.ID); merr != nil {
				addError("clearing mark on volume %s (%s/%s): %v", v.ID, p.Name, region, merr)
			}
		}
	}
	return out
}
