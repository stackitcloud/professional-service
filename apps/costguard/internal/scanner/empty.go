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
	"fmt"
	"sort"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// costWindowDays is the spend window of the empty-project check.
const costWindowDays = 30

// zeroSpendEUR is the spend below which a project counts as free.
const zeroSpendEUR = 0.005

// emptyProjects warns about projects older than WarnEmptyAfterDays that
// had no spend in the last 30 days and hold no servers, volumes, public
// IPs, SKE clusters or buckets in any scanned region. A project whose
// facts could not all be read is not reported.
func (s *Scanner) emptyProjects(ctx context.Context, projects []*node, invs []*inventory, rep *report.Report, errs *errorLog, now time.Time) {
	cutoff := now.AddDate(0, 0, -s.Config.WarnEmptyAfterDays)
	var old []*node
	for _, p := range projects {
		if !p.CreatedAt.IsZero() && p.CreatedAt.Before(cutoff) {
			old = append(old, p)
		}
	}
	if len(old) == 0 || s.Pacer.Pace(ctx) != nil {
		return
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	costs, err := s.Clients.Cost.ProjectCosts(ctx, s.Config.OrganizationID, today.AddDate(0, 0, -costWindowDays), today.AddDate(0, 0, -1))
	if err != nil {
		errs.add("reading cost data (no empty-project warnings this run)", "", err)
		return
	}
	byProject := map[string][]*inventory{}
	for _, inv := range invs {
		byProject[inv.projectID] = append(byProject[inv.projectID], inv)
	}

	for _, p := range old {
		if costs[p.ID] >= zeroSpendEUR {
			continue
		}
		pinvs := byProject[p.ID]
		if len(pinvs) != len(s.Config.Regions) || !noIaaS(pinvs) || !s.noServices(ctx, pinvs, errs) {
			continue
		}
		days := int(now.Sub(p.CreatedAt).Hours() / 24)
		rep.EmptyProjects = append(rep.EmptyProjects, report.Item{
			Kind: report.KindProject, ID: p.ID, Name: p.Name, ProjectID: p.ID, ProjectName: p.Name,
			Detail: fmt.Sprintf("created %s (%d days ago), €0 in the last %d days", p.CreatedAt.Format("2006-01-02"), days, costWindowDays),
		})
	}
	sort.Slice(rep.EmptyProjects, func(i, j int) bool { return rep.EmptyProjects[i].Name < rep.EmptyProjects[j].Name })
}

func noIaaS(invs []*inventory) bool {
	for _, inv := range invs {
		if !inv.ok(stackit.KindServer, stackit.KindVolume, stackit.KindPublicIP) {
			return false
		}
		if len(inv.res[stackit.KindServer])+len(inv.res[stackit.KindVolume])+len(inv.res[stackit.KindPublicIP]) > 0 {
			return false
		}
	}
	return true
}

func (s *Scanner) noServices(ctx context.Context, invs []*inventory, errs *errorLog) bool {
	for _, inv := range invs {
		s.checkSKE(ctx, inv, errs)
		if inv.skeFailed || len(inv.skeClusters) > 0 {
			return false
		}
	}
	for _, inv := range invs {
		if s.Pacer.Pace(ctx) != nil {
			return false
		}
		buckets, err := s.Clients.Services.Buckets(ctx, inv.projectID, inv.region)
		if err != nil {
			errs.add("listing buckets (empty-project check)", where(inv.projectName, inv.region), err)
			return false
		}
		if len(buckets) > 0 {
			return false
		}
	}
	return true
}

// emptyNetworkAreas warns about network areas older than
// WarnEmptyAfterDays with no project attached. Network areas belong to the
// organization, so this only runs when the whole organization is in scope.
func (s *Scanner) emptyNetworkAreas(ctx context.Context, rep *report.Report, errs *errorLog, now time.Time) {
	if s.Pacer.Pace(ctx) != nil {
		return
	}
	areas, err := s.Clients.IaaS.ListNetworkAreas(ctx, s.Config.OrganizationID)
	if err != nil {
		errs.add("listing network areas (no empty-network-area warnings this run)", "", err)
		return
	}
	cutoff := now.AddDate(0, 0, -s.Config.WarnEmptyAfterDays)
	for _, a := range areas {
		if a.ProjectCount != 0 || a.CreatedAt.IsZero() || !a.CreatedAt.Before(cutoff) || stackit.Protected(a.Labels) {
			continue
		}
		days := int(now.Sub(a.CreatedAt).Hours() / 24)
		rep.EmptyNetworkAreas = append(rep.EmptyNetworkAreas, report.Item{
			Kind: report.KindNetworkArea, ID: a.ID, Name: a.Name,
			Detail: fmt.Sprintf("created %s (%d days ago), no projects", a.CreatedAt.Format("2006-01-02"), days),
		})
	}
	sort.Slice(rep.EmptyNetworkAreas, func(i, j int) bool { return rep.EmptyNetworkAreas[i].Name < rep.EmptyNetworkAreas[j].Name })
}
