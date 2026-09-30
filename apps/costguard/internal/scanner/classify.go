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
	"fmt"
	"sort"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

var kindOrder = func() map[stackit.Kind]int {
	m := map[stackit.Kind]int{}
	for i, k := range stackit.Kinds {
		m[k] = i
	}
	return m
}()

func classify(invs []*inventory, rep *report.Report, errs *errorLog) *Result {
	sort.Slice(invs, func(i, j int) bool {
		if invs[i].projectName != invs[j].projectName {
			return invs[i].projectName < invs[j].projectName
		}
		if invs[i].projectID != invs[j].projectID {
			return invs[i].projectID < invs[j].projectID
		}
		return invs[i].region < invs[j].region
	})
	res := &Result{Context: DeleteContext{
		LoadBalancerAddresses: map[string]map[string]string{},
		NICServers:            map[string]string{},
	}}
	var newIPs, newVolumes []candidate
	for _, inv := range invs {
		ips, vols := classifyOne(inv, rep, res, errs)
		newIPs = append(newIPs, ips...)
		newVolumes = append(newVolumes, vols...)
	}

	sort.SliceStable(newVolumes, func(i, j int) bool { return newVolumes[i].item.SizeGB > newVolumes[j].item.SizeGB })
	rep.WaitingIdlePublicIPs = takeNew(newIPs, &rep.IdlePublicIPs, res)
	rep.WaitingDetachedVolumes = takeNew(newVolumes, &rep.DetachedVolumes, res)
	sort.SliceStable(res.Delete, func(i, j int) bool {
		return kindOrder[res.Delete[i].Kind] < kindOrder[res.Delete[j].Kind]
	})

	return res
}

type candidate struct {
	item report.Item
	res  stackit.Resource
}

func takeNew(cands []candidate, list *[]report.Item, res *Result) int {
	for i, c := range cands {
		if i == report.ListLimit {
			return len(cands) - report.ListLimit
		}
		*list = append(*list, c.item)
		res.Flag = append(res.Flag, c.res)
	}
	return 0
}

func classifyOne(inv *inventory, rep *report.Report, res *Result, errs *errorLog) (newIPs, newVolumes []candidate) {
	if inv.lbAddresses != nil {
		res.Context.LoadBalancerAddresses[inv.key()] = inv.lbAddresses
	}
	nicServer := map[string]string{}
	for _, nic := range inv.res[stackit.KindNIC] {
		if nic.ServerID != "" {
			nicServer[nic.ID] = nic.ServerID
			res.Context.NICServers[nic.ID] = nic.ServerID
		}
	}
	deleting := map[string]string{}
	for _, srv := range inv.res[stackit.KindServer] {
		if (stackit.Requested(srv.Labels) && !stackit.Protected(srv.Labels)) || srv.Status == stackit.ServerStatusDeleting {
			deleting[srv.ID] = srv.Name
		}
	}
	item := func(r stackit.Resource, detail string, isNew bool) report.Item {
		return report.Item{
			Kind: string(r.Kind), ID: r.ID, Name: r.Name, ProjectID: inv.projectID, ProjectName: inv.projectName,
			Region: inv.region, Detail: detail, New: isNew, NetworkID: r.NetworkID, VolumeID: r.VolumeID, SizeGB: r.SizeGB,
		}
	}
	request := func(r stackit.Resource, detail string) {
		rep.Requested = append(rep.Requested, item(r, detail, false))
		res.Delete = append(res.Delete, r)
	}
	unflag := func(r stackit.Resource, detail string) {
		rep.BackInUse = append(rep.BackInUse, item(r, detail, false))
		res.Unflag = append(res.Unflag, r)
	}
	protectedMarked := func(r stackit.Resource) {
		rep.ProtectedMarked = append(rep.ProtectedMarked, item(r, "", false))
		if r.Kind == stackit.KindVolume || r.Kind == stackit.KindPublicIP {
			res.ClearStale = append(res.ClearStale, r)
		}
	}

	for _, kind := range []stackit.Kind{stackit.KindServer, stackit.KindNIC, stackit.KindSnapshot, stackit.KindSecurityGroup} {
		for _, r := range inv.res[kind] {
			switch {
			case stackit.Requested(r.Labels) && stackit.Protected(r.Labels):
				protectedMarked(r)
			case stackit.Requested(r.Labels):
				request(r, requestDetail(r))
			}
		}
	}

	snapshots := snapshotCounts(inv)
	newest := newestSnapshots(inv)
	for _, v := range inv.res[stackit.KindVolume] {
		if stackit.Protected(v.Labels) {
			if stackit.Requested(v.Labels) {
				protectedMarked(v)
			}
			continue
		}
		size := fmt.Sprintf("%d GB", v.SizeGB)
		switch {
		case stackit.Requested(v.Labels) && unusedVolume(v):
			rep.DetachedVolumes = append(rep.DetachedVolumes, item(v, size, false))
			res.Delete = append(res.Delete, v)
		case stackit.Requested(v.Labels) && deleting[v.ServerID] != "":
			request(v, size+", deleted after its server "+deleting[v.ServerID])
		case stackit.Requested(v.Labels):
			if inv.ok(stackit.KindServer) {
				unflag(v, "attached to a server")
			}
		case !unusedVolume(v) || !inv.ok(stackit.KindSnapshot):
		case snapshots[v.ID] > 0:
			it := item(v, size+", "+report.Count(snapshots[v.ID], "snapshot"), false)
			it.SnapshotID = newest[v.ID].ID
			rep.WithSnapshots = append(rep.WithSnapshots, it)
		case !inv.skeChecked || inv.skeFailed:
		case len(inv.skeClusters) > 0:
		default:
			newVolumes = append(newVolumes, candidate{item(v, size, true), v})
		}
	}

	for _, ip := range inv.res[stackit.KindPublicIP] {
		if stackit.Protected(ip.Labels) {
			if stackit.Requested(ip.Labels) {
				protectedMarked(ip)
			}
			continue
		}
		requested := stackit.Requested(ip.Labels)
		if ip.NICID != "" {
			if !requested {
				continue
			}
			server, known := nicServer[ip.NICID]
			switch {
			case known && deleting[server] != "":
				request(ip, "deleted after its server "+deleting[server])
			case inv.ok(stackit.KindNIC, stackit.KindServer):
				unflag(ip, "attached to a network interface")
			}
			continue
		}
		if inv.lbFailed {
			continue
		}
		if lb, used := inv.lbAddresses[ip.Address]; used {
			if requested {
				unflag(ip, "used by "+lb)
			}
			continue
		}
		if requested {
			rep.IdlePublicIPs = append(rep.IdlePublicIPs, item(ip, "", false))
			res.Delete = append(res.Delete, ip)
		} else {
			newIPs = append(newIPs, candidate{item(ip, "", true), ip})
		}
	}
	return newIPs, newVolumes
}

func newestSnapshots(inv *inventory) map[string]stackit.Resource {
	out := map[string]stackit.Resource{}
	for _, s := range inv.res[stackit.KindSnapshot] {
		cur, ok := out[s.VolumeID]
		if !ok || s.CreatedAt.After(cur.CreatedAt) || (s.CreatedAt.Equal(cur.CreatedAt) && s.ID > cur.ID) {
			out[s.VolumeID] = s
		}
	}
	return out
}

func requestDetail(r stackit.Resource) string {
	switch r.Kind {
	case stackit.KindServer:
		if r.Status != "" {
			return "status " + r.Status
		}
	case stackit.KindSnapshot:
		return fmt.Sprintf("%d GB", r.SizeGB)
	case stackit.KindNIC:
		if r.ServerID != "" {
			return "attached to a server"
		}
	}
	return ""
}
