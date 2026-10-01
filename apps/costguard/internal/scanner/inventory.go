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
	"sync"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

type inventory struct {
	projectID   string
	projectName string
	region      string
	res         map[stackit.Kind][]stackit.Resource
	failed      map[stackit.Kind]bool

	lbAddresses map[string]string
	lbFailed    bool
	skeClusters []string
	skeChecked  bool
	skeFailed   bool
}

func (inv *inventory) key() string {
	return inv.projectID + "/" + inv.region
}

func (inv *inventory) ok(kinds ...stackit.Kind) bool {
	for _, k := range kinds {
		if inv.failed[k] {
			return false
		}
	}
	return true
}

func (s *Scanner) inventory(ctx context.Context, projects []*node, errs *errorLog) []*inventory {
	var mu sync.Mutex
	var out []*inventory
	var jobs []func(context.Context)
	for _, p := range projects {
		for _, region := range s.Config.Regions {
			jobs = append(jobs, func(ctx context.Context) {
				inv := s.list(ctx, p, region, errs)
				mu.Lock()
				out = append(out, inv)
				mu.Unlock()
			})
		}
	}
	runPooled(ctx, s.Workers, jobs)
	return out
}

func (s *Scanner) list(ctx context.Context, p *node, region string, errs *errorLog) *inventory {
	inv := &inventory{
		projectID:   p.ID,
		projectName: p.Name,
		region:      region,
		res:         map[stackit.Kind][]stackit.Resource{},
		failed:      map[stackit.Kind]bool{},
	}
	for _, kind := range stackit.Kinds {
		if s.Pacer.Pace(ctx) != nil {
			inv.failed[kind] = true
			continue
		}
		items, err := s.Clients.IaaS.List(ctx, kind, p.ID, region)
		switch {
		case stackit.NotEnabled(err):
		case err != nil:
			inv.failed[kind] = true
			errs.add("listing "+kind.Name()+"s", where(p.Name, region), err)
		default:
			inv.res[kind] = items
		}
	}

	if needsLoadBalancers(inv) && s.Pacer.Pace(ctx) == nil {
		addrs, err := s.Clients.Services.LoadBalancerAddresses(ctx, p.ID, region)
		if err != nil {
			inv.lbFailed = true
			errs.add("load balancer lookup (public IPs without a network interface are left alone)", where(p.Name, region), err)
		}
		inv.lbAddresses = addrs
	}
	if !s.DeleteRun && needsSKE(inv) {
		s.checkSKE(ctx, inv, errs)
	}
	return inv
}

func (s *Scanner) checkSKE(ctx context.Context, inv *inventory, errs *errorLog) {
	if inv.skeChecked {
		return
	}
	inv.skeChecked = true
	if s.Pacer.Pace(ctx) != nil {
		inv.skeFailed = true
		return
	}
	clusters, err := s.Clients.Services.SKEClusters(ctx, inv.projectID, inv.region)
	if err != nil {
		inv.skeFailed = true
		errs.add("SKE cluster lookup (detached volumes there are not flagged)", where(inv.projectName, inv.region), err)
	}
	inv.skeClusters = clusters
}

func where(project, region string) string {
	return project + " (" + region + ")"
}

func needsLoadBalancers(inv *inventory) bool {
	for _, ip := range inv.res[stackit.KindPublicIP] {
		if ip.NICID == "" && !stackit.Protected(ip.Labels) {
			return true
		}
	}
	return false
}

func needsSKE(inv *inventory) bool {
	if !inv.ok(stackit.KindSnapshot) {
		return false
	}
	snapshots := snapshotCounts(inv)
	for _, v := range inv.res[stackit.KindVolume] {
		if unusedVolume(v) && !stackit.Requested(v.Labels) && !stackit.Protected(v.Labels) && snapshots[v.ID] == 0 {
			return true
		}
	}
	return false
}

func snapshotCounts(inv *inventory) map[string]int {
	out := map[string]int{}
	for _, s := range inv.res[stackit.KindSnapshot] {
		out[s.VolumeID]++
	}
	return out
}

func unusedVolume(v stackit.Resource) bool {
	return v.Status == stackit.VolumeStatusAvailable && v.ServerID == ""
}
