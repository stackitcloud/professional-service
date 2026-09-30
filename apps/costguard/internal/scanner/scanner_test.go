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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/fake"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const (
	org     = "00000000-0000-0000-0000-00000000000a"
	fPlat   = "00000000-0000-0000-0000-0000000000f1"
	fTeams  = "00000000-0000-0000-0000-0000000000f2"
	fParent = "00000000-0000-0000-0000-0000000000f3"
	pA      = "00000000-0000-0000-0000-0000000000a1"
	pC      = "00000000-0000-0000-0000-0000000000c1"
)

var (
	now = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	old = now.AddDate(0, -6, 0)
)

func testConfig(mod func(*config.Config)) config.Config {
	c := config.Config{
		OrganizationID:     org,
		Regions:            []string{"eu01"},
		WarnEmptyAfterDays: 30,
		Prices:             config.Prices{PublicIPMonthlyEUR: 5, VolumeGBMonthlyEUR: 0.1},
	}
	if mod != nil {
		mod(&c)
	}
	return c
}

func newScanner(st *fake.Store, c config.Config) *Scanner {
	return &Scanner{
		Clients: st.Set(),
		Config:  c,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Pacer:   NopPacer{},
		Now:     func() time.Time { return now },
		Workers: 3,
	}
}

func scan(t *testing.T, st *fake.Store, c config.Config) *Result {
	t.Helper()
	res, err := newScanner(st, c).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return res
}

type opt func(*stackit.Resource)

func r(kind stackit.Kind, id, project string, opts ...opt) stackit.Resource {
	res := stackit.Resource{Kind: kind, ID: id, Name: id + "-name", ProjectID: project, Region: "eu01"}
	for _, o := range opts {
		o(&res)
	}
	return res
}

func del() opt  { return withLabel(stackit.LabelDelete, "true") }
func keep() opt { return withLabel(stackit.LabelDoNotDelete, "true") }
func withLabel(k, v string) opt {
	return func(r *stackit.Resource) {
		if r.Labels == nil {
			r.Labels = map[string]string{}
		}
		r.Labels[k] = v
	}
}
func detached(gb int64) opt {
	return func(r *stackit.Resource) { r.Status, r.SizeGB = stackit.VolumeStatusAvailable, gb }
}
func attachedTo(server string) opt {
	return func(r *stackit.Resource) { r.Status, r.ServerID = "IN-USE", server }
}
func nic(id string) opt   { return func(r *stackit.Resource) { r.NICID = id } }
func addr(a string) opt   { return func(r *stackit.Resource) { r.Address = a } }
func of(vol string) opt   { return func(r *stackit.Resource) { r.VolumeID = vol } }
func status(s string) opt { return func(r *stackit.Resource) { r.Status = s } }

func ids(items []report.Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func resIDs(rs []stackit.Resource) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func eq(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func oneProject() *fake.Store {
	st := fake.New()
	st.AddProject("p1", "proj-one", org, now.AddDate(0, 0, -1), nil)
	return st
}

func TestScanAppliesEveryRule(t *testing.T) {
	st := oneProject()
	st.Add(
		r(stackit.KindServer, "s1", "p1", del(), status("ACTIVE")),
		r(stackit.KindServer, "s2", "p1"),
		r(stackit.KindServer, "s3", "p1", del(), keep()),

		r(stackit.KindVolume, "v1", "p1", detached(100)),
		r(stackit.KindVolume, "v2", "p1", detached(10)),
		r(stackit.KindVolume, "v3", "p1", detached(20), del()),
		r(stackit.KindVolume, "v4", "p1", attachedTo("s2"), del()),
		r(stackit.KindVolume, "v5", "p1", attachedTo("s1"), del()),
		r(stackit.KindVolume, "v6", "p1", detached(5), keep()),
		r(stackit.KindVolume, "v7", "p1", attachedTo("s2")),

		r(stackit.KindSnapshot, "sn1", "p1", of("v2")),
		r(stackit.KindSnapshot, "sn2", "p1", of("v7"), del()),

		r(stackit.KindNIC, "n1", "p1", attachedTo("s1")),
		r(stackit.KindNIC, "n2", "p1", attachedTo("s2")),
		r(stackit.KindNIC, "n3", "p1", del()),

		r(stackit.KindPublicIP, "ip1", "p1", addr("192.0.2.1")),
		r(stackit.KindPublicIP, "ip2", "p1", addr("192.0.2.2")),
		r(stackit.KindPublicIP, "ip3", "p1", addr("192.0.2.3"), del()),
		r(stackit.KindPublicIP, "ip4", "p1", addr("192.0.2.4"), del(), nic("n2")),
		r(stackit.KindPublicIP, "ip5", "p1", addr("192.0.2.5"), del(), nic("n1")),
		r(stackit.KindPublicIP, "ip6", "p1", addr("192.0.2.6"), del()),
		r(stackit.KindPublicIP, "ip7", "p1", addr("192.0.2.7"), keep()),
		r(stackit.KindPublicIP, "ip8", "p1", addr("192.0.2.8"), nic("n2")),

		r(stackit.KindSecurityGroup, "sg1", "p1", del()),
		r(stackit.KindSecurityGroup, "sg2", "p1", del(), keep()),

		r(stackit.KindVolume, "v8", "p1", detached(30), del(), keep()),
		r(stackit.KindPublicIP, "ip9", "p1", addr("192.0.2.9"), del(), keep()),
	)
	st.LB["p1/eu01"] = map[string]string{"192.0.2.2": "network load balancer web", "192.0.2.6": "application load balancer shop"}

	res := scan(t, st, testConfig(nil))
	rep := res.Report

	eq(t, "idle IPs", ids(rep.IdlePublicIPs), []string{"ip3", "ip1"})
	eq(t, "detached volumes", ids(rep.DetachedVolumes), []string{"v3", "v1"})
	eq(t, "with snapshots", ids(rep.WithSnapshots), []string{"v2"})
	eq(t, "back in use", sorted(ids(rep.BackInUse)), []string{"ip4", "ip6", "v4"})
	eq(t, "requested", sorted(ids(rep.Requested)), []string{"ip5", "n3", "s1", "sg1", "sn2", "v5"})
	eq(t, "flag", sorted(resIDs(res.Flag)), []string{"ip1", "v1"})
	eq(t, "unflag", sorted(resIDs(res.Unflag)), []string{"ip4", "ip6", "v4"})
	eq(t, "delete order", resIDs(res.Delete), []string{"s1", "ip3", "ip5", "n3", "sn2", "v3", "v5", "sg1"})
	eq(t, "protected but marked", sorted(ids(rep.ProtectedMarked)), []string{"ip9", "s3", "sg2", "v8"})
	eq(t, "clear stale (only volumes and IPs)", sorted(resIDs(res.ClearStale)), []string{"ip9", "v8"})

	if rep.IdlePublicIPs[0].New || !rep.IdlePublicIPs[1].New || rep.DetachedVolumes[0].New || !rep.DetachedVolumes[1].New {
		t.Errorf("New flags wrong: %+v %+v", rep.IdlePublicIPs, rep.DetachedVolumes)
	}
	if rep.DetachedVolumes[0].SizeGB != 20 || rep.DetachedVolumes[1].SizeGB != 100 {
		t.Errorf("volume sizes are needed for the savings: %+v", rep.DetachedVolumes)
	}
	for _, it := range rep.Requested {
		if it.ID == "v5" && !strings.Contains(it.Detail, "after its server s1-name") {
			t.Errorf("v5 detail = %q", it.Detail)
		}
	}
	for _, it := range rep.BackInUse {
		if it.ID == "ip6" && it.Detail != "used by application load balancer shop" {
			t.Errorf("ip6 detail = %q", it.Detail)
		}
	}
	if rep.WithSnapshots[0].Detail != "10 GB, 1 snapshot" {
		t.Errorf("snapshot detail = %q", rep.WithSnapshots[0].Detail)
	}
	if res.Context.NICServers["n1"] != "s1" || res.Context.LoadBalancerAddresses["p1/eu01"]["192.0.2.2"] == "" {
		t.Errorf("delete context = %+v", res.Context)
	}
	if res.Context.ProjectNames["p1"] != "proj-one" || rep.Scope != "whole organization" {
		t.Errorf("names/scope wrong: %+v %q", res.Context.ProjectNames, rep.Scope)
	}
	if len(rep.ScanErrors) != 0 || res.Blocked() {
		t.Errorf("unexpected errors/block: %v %v", rep.ScanErrors, rep.Blocked)
	}
	if rep.ToDelete() != 10 {
		t.Errorf("ToDelete = %d", rep.ToDelete())
	}
}

func TestScanCapsNewCandidatesButNeverMarkedOnes(t *testing.T) {
	st := oneProject()
	for i := 0; i < 15; i++ {
		st.Add(r(stackit.KindVolume, fmt.Sprintf("new-%02d", i), "p1", detached(int64(10+i))))
		st.Add(r(stackit.KindPublicIP, fmt.Sprintf("ip-new-%02d", i), "p1", addr(fmt.Sprintf("192.0.2.%d", i))))
	}
	for i := 0; i < 12; i++ {
		st.Add(r(stackit.KindPublicIP, fmt.Sprintf("ip-marked-%02d", i), "p1", addr(fmt.Sprintf("198.51.100.%d", i)), del()))
	}
	res := scan(t, st, testConfig(nil))
	rep := res.Report

	if len(rep.IdlePublicIPs) != 22 || rep.WaitingIdlePublicIPs != 5 {
		t.Errorf("IPs listed %d, waiting %d", len(rep.IdlePublicIPs), rep.WaitingIdlePublicIPs)
	}
	if len(rep.DetachedVolumes) != 10 || rep.WaitingDetachedVolumes != 5 || rep.DetachedVolumes[0].SizeGB != 24 || rep.DetachedVolumes[9].SizeGB != 15 {
		t.Errorf("volumes listed %d (first %d GB, last %d GB), waiting %d",
			len(rep.DetachedVolumes), rep.DetachedVolumes[0].SizeGB, rep.DetachedVolumes[len(rep.DetachedVolumes)-1].SizeGB, rep.WaitingDetachedVolumes)
	}
	if len(res.Flag) != 20 || len(res.Delete) != 12 {
		t.Errorf("flag %d, delete %d", len(res.Flag), len(res.Delete))
	}
	listed := map[string]bool{}
	for _, it := range append(append([]report.Item{}, rep.IdlePublicIPs...), rep.DetachedVolumes...) {
		listed[it.ID] = true
	}
	for _, r := range append(append([]stackit.Resource{}, res.Flag...), res.Delete...) {
		if !listed[r.ID] {
			t.Errorf("%s is acted on but not listed", r.ID)
		}
	}
}

func TestScanDeletingServerKeepsItsVolumeLabelled(t *testing.T) {
	st := oneProject()
	st.Add(
		r(stackit.KindServer, "s1", "p1", status(stackit.ServerStatusDeleting)),
		r(stackit.KindVolume, "v1", "p1", attachedTo("s1"), del()),
	)
	res := scan(t, st, testConfig(nil))
	eq(t, "delete", resIDs(res.Delete), []string{"v1"})
	eq(t, "unflag", resIDs(res.Unflag), []string{})
}

func TestScanSKEProjectsKeepTheirDetachedVolumes(t *testing.T) {
	st := oneProject()
	st.Add(r(stackit.KindVolume, "v1", "p1", detached(10)))
	st.SKE["p1/eu01"] = []string{"prod"}
	res := scan(t, st, testConfig(nil))
	eq(t, "flag", resIDs(res.Flag), []string{})
	eq(t, "volumes", ids(res.Report.DetachedVolumes), []string{})

	st.SKE = map[string][]string{}
	st.Errs["ske:p1/eu01"] = fake.Status(500)
	res = scan(t, st, testConfig(nil))
	eq(t, "flag after SKE error", resIDs(res.Flag), []string{})
	if len(res.Report.ScanErrors) != 1 {
		t.Errorf("scan errors = %v", res.Report.ScanErrors)
	}
}

func TestScanLeavesIPsAloneWhenLoadBalancersUnknown(t *testing.T) {
	st := oneProject()
	st.Add(
		r(stackit.KindPublicIP, "ip1", "p1", addr("192.0.2.1")),
		r(stackit.KindPublicIP, "ip2", "p1", addr("192.0.2.2"), del()),
	)
	st.Errs["lb:p1/eu01"] = fake.Status(403)
	res := scan(t, st, testConfig(nil))
	if len(res.Flag)+len(res.Delete)+len(res.Unflag) != 0 {
		t.Errorf("nothing may be touched: %+v", res)
	}
	if len(res.Report.ScanErrors) != 1 || !strings.Contains(res.Report.ScanErrors[0], "left alone") {
		t.Errorf("scan errors = %v", res.Report.ScanErrors)
	}
}

func TestScanErrorsAreGroupedByCause(t *testing.T) {
	st := fake.New()
	for _, p := range []string{"a", "b", "c", "d"} {
		st.AddProject(p, "proj-"+p, org, now, nil)
		st.Errs["list:snapshot:"+p+"/eu01"] = &oapierror.GenericOpenAPIError{StatusCode: 403, Body: []byte(`{"message":"missing permission iaas.snapshot.list"}`)}
	}
	st.Errs["list:server:a/eu01"] = fake.Status(500)
	res := scan(t, st, testConfig(nil))
	eq(t, "scan errors", res.Report.ScanErrors, []string{
		"listing servers: HTTP 500 (proj-a (eu01))",
		"listing snapshots: HTTP 403: missing permission iaas.snapshot.list (in 4 places, e.g. proj-a (eu01), proj-b (eu01), proj-c (eu01))",
	})

	delete(st.Errs, "list:snapshot:d/eu01")
	res = scan(t, st, testConfig(nil))
	if got := res.Report.ScanErrors[1]; !strings.HasSuffix(got, "(in 3 places: proj-a (eu01), proj-b (eu01), proj-c (eu01))") {
		t.Errorf("three places are all named: %q", got)
	}
}

func TestScanListFailuresLeaveThingsAlone(t *testing.T) {
	st := oneProject()
	st.Add(
		r(stackit.KindVolume, "v1", "p1", detached(10)),
		r(stackit.KindVolume, "v2", "p1", attachedTo("s9"), del()),
		r(stackit.KindPublicIP, "ip1", "p1", addr("192.0.2.1"), del(), nic("n9")),
	)
	st.Errs["list:snapshot:p1/eu01"] = fake.Status(500)
	st.Errs["list:server:p1/eu01"] = fake.Status(500)
	st.Errs["list:nic:p1/eu01"] = fake.Status(500)
	res := scan(t, st, testConfig(nil))
	if len(res.Flag)+len(res.Delete)+len(res.Unflag) != 0 {
		t.Errorf("nothing may be touched: flag=%v delete=%v unflag=%v", resIDs(res.Flag), resIDs(res.Delete), resIDs(res.Unflag))
	}
	if len(res.Report.ScanErrors) != 3 {
		t.Errorf("scan errors = %v", res.Report.ScanErrors)
	}

	st = oneProject()
	for _, k := range stackit.Kinds {
		st.Errs["list:"+string(k)+":p1/eu01"] = fake.Status(404)
	}
	res = scan(t, st, testConfig(nil))
	if len(res.Report.ScanErrors) != 0 {
		t.Errorf("404 must not be an error: %v", res.Report.ScanErrors)
	}
}

func skipTree() *fake.Store {
	st := fake.New()
	st.AddFolder(fPlat, "Platform", org, nil)
	st.AddFolder(fTeams, "Teams", org, nil)
	st.AddProject(pA, "pa", fPlat, old, nil)
	st.AddProject("pb", "pb", fTeams, old, map[string]string{"do-not-delete": "true"})
	st.AddProject(pC, "pc", fTeams, old, nil)
	st.AddProject("pd", "prod-billing", org, old, nil)
	for _, p := range []string{pA, "pb", pC, "pd"} {
		st.Add(r(stackit.KindVolume, "vol-"+p, p, detached(1)))
		st.Costs[p] = 1
	}
	return st
}

func TestScanSkipsByNameIDAndLabel(t *testing.T) {
	st := skipTree()
	res := scan(t, st, testConfig(func(c *config.Config) {
		c.Skip = config.Selection{Folders: []string{"PLATFORM"}, Projects: []string{"prod-billing"}}
	}))
	eq(t, "flag", resIDs(res.Flag), []string{"vol-" + pC})
	if res.Report.SkippedFolders != 1 || res.Report.SkippedProjects != 3 {
		t.Errorf("skipped %d folders, %d projects", res.Report.SkippedFolders, res.Report.SkippedProjects)
	}
	if res.Blocked() {
		t.Errorf("blocked: %v", res.Report.Blocked)
	}

	res = scan(t, skipTree(), testConfig(func(c *config.Config) {
		c.Skip = config.Selection{Folders: []string{fTeams}}
	}))
	eq(t, "flag with folder ID skip", sorted(resIDs(res.Flag)), sorted([]string{"vol-" + pA, "vol-pd"}))
}

func TestScanSkipsHoldWhenListingsReturnDescendants(t *testing.T) {
	st := skipTree()
	st.ListDescendants = true
	res := scan(t, st, testConfig(func(c *config.Config) {
		c.Skip = config.Selection{Folders: []string{"platform"}, Projects: []string{"prod-billing"}}
	}))
	eq(t, "flag", resIDs(res.Flag), []string{"vol-" + pC})
	if res.Report.SkippedFolders != 1 || res.Report.SkippedProjects != 3 {
		t.Errorf("skipped %d folders, %d projects", res.Report.SkippedFolders, res.Report.SkippedProjects)
	}

	st = skipTree()
	st.ListDescendants = true
	st.AddFolder(fParent, "Parent", org, nil)
	c := st.Containers[fTeams]
	c.ParentID = fParent
	st.Containers[fTeams] = c
	res = scan(t, st, testConfig(func(c *config.Config) {
		c.Scope.Folders = []string{fParent}
		c.Skip.Folders = []string{"teams"}
	}))
	if len(res.Flag) != 0 || res.Report.SkippedProjects != 2 {
		t.Errorf("flag=%v skipped=%d", resIDs(res.Flag), res.Report.SkippedProjects)
	}
}

func TestScanDanglingSkipEntryBlocks(t *testing.T) {
	res := scan(t, skipTree(), testConfig(func(c *config.Config) {
		c.Skip = config.Selection{Projects: []string{"renamed-project"}, Folders: []string{fParent}}
	}))
	eq(t, "blocked", res.Report.Blocked, []string{`skip.folders: "` + fParent + `"`, `skip.projects: "renamed-project"`})
	if !res.Blocked() {
		t.Error("Blocked() must be true")
	}
}

func TestScanScopeByName(t *testing.T) {
	st := skipTree()
	res := scan(t, st, testConfig(func(c *config.Config) {
		c.Scope = config.Selection{Folders: []string{"teams"}, Projects: []string{"prod-billing"}}
	}))
	eq(t, "flag", sorted(resIDs(res.Flag)), sorted([]string{"vol-" + pC, "vol-pd"}))
	if res.Report.Scope != "Teams, prod-billing" || res.Report.SkippedProjects != 1 {
		t.Errorf("scope %q skipped %d", res.Report.Scope, res.Report.SkippedProjects)
	}

	st.AddProject("pe", "PB", fPlat, old, nil)
	_, err := newScanner(st, testConfig(func(c *config.Config) {
		c.Scope = config.Selection{Projects: []string{"pb", "nope"}}
	})).Scan(context.Background())
	var se *ScopeError
	if !errors.As(err, &se) || len(se.Problems) != 2 {
		t.Fatalf("want 2 scope problems, got %v", err)
	}
	if !strings.Contains(err.Error(), "matches 2 containers") || !strings.Contains(err.Error(), `"nope" matches nothing`) {
		t.Errorf("error = %v", err)
	}
}

func TestScanScopeByIDDoesNotReadTheOrg(t *testing.T) {
	st := skipTree()
	res := scan(t, st, testConfig(func(c *config.Config) {
		c.Scope = config.Selection{Folders: []string{fTeams}, Projects: []string{pA}}
	}))
	eq(t, "flag", sorted(resIDs(res.Flag)), sorted([]string{"vol-" + pC, "vol-" + pA}))
	if got := st.CallsWith("list projects " + org); len(got) != 0 {
		t.Errorf("the org must not be listed: %v", got)
	}
	if res.Report.Scope != "Teams, pa" {
		t.Errorf("scope = %q", res.Report.Scope)
	}
}

func TestScanScopeByIDHonoursFoldersAbove(t *testing.T) {
	st := skipTree()
	st.AddFolder(fParent, "Parent", org, map[string]string{"do-not-delete": "true"})
	c := st.Containers[fTeams]
	c.ParentID = fParent
	st.Containers[fTeams] = c

	res := scan(t, st, testConfig(func(c *config.Config) { c.Scope.Folders = []string{fTeams} }))
	if len(res.Flag) != 0 || res.Report.SkippedFolders != 1 || res.Report.SkippedProjects != 2 {
		t.Errorf("flag=%v skipped=%d/%d", resIDs(res.Flag), res.Report.SkippedFolders, res.Report.SkippedProjects)
	}

	delete(st.Containers[fParent].Labels, "do-not-delete")
	st.Errs["get:"+fParent] = fake.Status(403)
	res = scan(t, st, testConfig(func(c *config.Config) {
		c.Scope.Folders = []string{fTeams}
		c.Skip.Folders = []string{"parent"}
	}))
	if len(res.Flag) != 0 || res.Blocked() {
		t.Errorf("flag=%v blocked=%v", resIDs(res.Flag), res.Report.Blocked)
	}
}

func TestScanScopeByIDErrors(t *testing.T) {
	st := skipTree()
	const inactive = "00000000-0000-0000-0000-0000000000d1"
	st.Containers[inactive] = stackit.Container{ID: inactive, Name: "old", ParentID: org, LifecycleState: "DELETING"}
	_, err := newScanner(st, testConfig(func(c *config.Config) {
		c.Scope = config.Selection{Folders: []string{fParent}, Projects: []string{inactive}}
	})).Scan(context.Background())
	var se *ScopeError
	if !errors.As(err, &se) || len(se.Problems) != 2 {
		t.Fatalf("want 2 problems, got %v", err)
	}
	if !strings.Contains(err.Error(), fParent+": HTTP 404") || !strings.Contains(err.Error(), inactive+" is DELETING") ||
		strings.Contains(err.Error(), "status code") {
		t.Errorf("scope errors must be readable: %v", err)
	}
	if got := st.CallsWith("list projects " + org); len(got) != 0 {
		t.Errorf("the ID path must not walk the organization: %v", got)
	}
}

func TestScanWalkErrorsAreScanErrors(t *testing.T) {
	st := skipTree()
	st.Errs["folders:"+fTeams] = fake.Status(500)
	st.Errs["projects:"+fPlat] = fake.Status(500)
	res := scan(t, st, testConfig(nil))
	if len(res.Report.ScanErrors) != 2 {
		t.Errorf("scan errors = %v", res.Report.ScanErrors)
	}
	eq(t, "flag", sorted(resIDs(res.Flag)), sorted([]string{"vol-" + pC, "vol-pd"}))
}

func TestScanUnreadableOrganizationFails(t *testing.T) {
	for _, key := range []string{"projects:" + org, "folders:" + org} {
		for name, scope := range map[string]config.Selection{
			"whole org":   {},
			"named scope": {Folders: []string{"Teams"}},
		} {
			st := skipTree()
			st.Errs[key] = fake.Status(403)
			_, err := newScanner(st, testConfig(func(c *config.Config) { c.Scope = scope })).Scan(context.Background())
			if err == nil || !strings.Contains(err.Error(), "cannot read the organization "+org) || !strings.Contains(err.Error(), "HTTP 403") {
				t.Errorf("%s, %s: want a clear failure, got %v", key, name, err)
			}
		}
	}
}

func TestScanUnreadableScopeFolderFails(t *testing.T) {
	st := skipTree()
	st.Errs["projects:"+fTeams] = fake.Status(403)
	_, err := newScanner(st, testConfig(func(c *config.Config) { c.Scope.Folders = []string{fTeams} })).Scan(context.Background())
	var se *ScopeError
	if !errors.As(err, &se) || !strings.Contains(err.Error(), "its content cannot be listed: HTTP 403") {
		t.Fatalf("want a scope error, got %v", err)
	}
}

func TestScanEmptyProjects(t *testing.T) {
	st := fake.New()
	st.AddProject("empty", "empty", org, old, nil)
	st.AddProject("young", "young", org, now.AddDate(0, 0, -3), nil)
	st.AddProject("costs", "costs", org, old, nil)
	st.AddProject("bucket", "bucket", org, old, nil)
	st.AddProject("ske", "ske", org, old, nil)
	st.AddProject("server", "server", org, old, nil)
	st.Costs["costs"] = 3.5
	st.BucketNames["bucket/eu01"] = []string{"logs"}
	st.SKE["ske/eu01"] = []string{"prod"}
	st.Add(r(stackit.KindServer, "s1", "server"))

	res := scan(t, st, testConfig(nil))
	eq(t, "empty projects", ids(res.Report.EmptyProjects), []string{"empty"})
	if d := res.Report.EmptyProjects[0].Detail; !strings.Contains(d, "created 2026-03-28") || !strings.Contains(d, "€0 in the last 30 days") {
		t.Errorf("detail = %q", d)
	}
	if got := st.CallsWith("costs "); len(got) != 1 || got[0] != "costs 2026-08-29 2026-09-27" {
		t.Errorf("cost window = %v", got)
	}

	st.Errs["costs"] = fake.Status(500)
	res = scan(t, st, testConfig(nil))
	if len(res.Report.EmptyProjects) != 0 || len(res.Report.ScanErrors) != 1 {
		t.Errorf("cost error: %v %v", ids(res.Report.EmptyProjects), res.Report.ScanErrors)
	}

	delete(st.Errs, "costs")
	st.Errs["buckets:empty/eu01"] = fake.Status(500)
	res = scan(t, st, testConfig(nil))
	if len(res.Report.EmptyProjects) != 0 || len(res.Report.ScanErrors) != 1 {
		t.Errorf("bucket error: %v %v", ids(res.Report.EmptyProjects), res.Report.ScanErrors)
	}
}

func TestScanEmptyNetworkAreas(t *testing.T) {
	st := oneProject()
	st.Areas = []stackit.NetworkArea{
		{ID: "a1", Name: "old-empty", CreatedAt: old},
		{ID: "a2", Name: "young", CreatedAt: now.AddDate(0, 0, -2)},
		{ID: "a3", Name: "used", ProjectCount: 2, CreatedAt: old},
		{ID: "a4", Name: "kept", CreatedAt: old, Labels: map[string]string{"do-not-delete": "true"}},
	}
	res := scan(t, st, testConfig(nil))
	eq(t, "areas", ids(res.Report.EmptyNetworkAreas), []string{"a1"})

	res = scan(t, st, testConfig(func(c *config.Config) { c.Scope.Projects = []string{"proj-one"} }))
	if len(res.Report.EmptyNetworkAreas) != 0 {
		t.Error("network areas are only checked for the whole organization")
	}

	st.Errs["areas"] = fake.Status(403)
	res = scan(t, st, testConfig(nil))
	if len(res.Report.ScanErrors) != 1 {
		t.Errorf("scan errors = %v", res.Report.ScanErrors)
	}
}

func TestScanDeleteRunSkipsWarnings(t *testing.T) {
	st := oneProject()
	st.AddProject("empty", "empty", org, old, nil)
	st.Areas = []stackit.NetworkArea{{ID: "a1", Name: "old-empty", CreatedAt: old}}
	s := newScanner(st, testConfig(nil))
	s.DeleteRun = true
	res, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Report.EmptyProjects)+len(res.Report.EmptyNetworkAreas) != 0 || len(st.CallsWith("costs")) != 0 {
		t.Errorf("warnings must be skipped: %+v, calls %v", res.Report, st.CallsWith("costs"))
	}
}

func TestScanDeleteRunSkipsSKE(t *testing.T) {
	st := oneProject()
	st.Add(r(stackit.KindVolume, "v1", "p1", detached(100)))
	st.Errs["ske:p1/eu01"] = fake.Status(403)
	s := newScanner(st, testConfig(nil))
	s.DeleteRun = true
	res, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls := st.CallsWith("ske"); len(calls) != 0 || len(res.Report.ScanErrors) != 0 || len(res.Flag) != 0 {
		t.Errorf("ske calls %v, scan errors %v, flag %v", calls, res.Report.ScanErrors, res.Flag)
	}
	res = scan(t, st, testConfig(nil))
	if len(st.CallsWith("ske")) != 1 || len(res.Report.ScanErrors) != 1 {
		t.Errorf("report run: ske calls %v, scan errors %v", st.CallsWith("ske"), res.Report.ScanErrors)
	}
}

func TestScanWarningsOff(t *testing.T) {
	st := oneProject()
	st.AddProject("empty", "empty", org, old, nil)
	st.Areas = []stackit.NetworkArea{{ID: "a1", Name: "old-empty", CreatedAt: old}}
	st.Errs["costs"] = fake.Status(403)
	res := scan(t, st, testConfig(func(c *config.Config) { c.WarnEmptyAfterDays = 0 }))
	if len(res.Report.EmptyProjects)+len(res.Report.EmptyNetworkAreas)+len(res.Report.ScanErrors) != 0 {
		t.Errorf("warnings off: nothing may be reported, got %+v", res.Report)
	}
	if len(st.CallsWith("costs")) != 0 {
		t.Error("warnings off: the cost data must not be read")
	}
}

func TestScanStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newScanner(skipTree(), testConfig(nil)).Scan(ctx); err == nil {
		t.Fatal("want context error")
	}
}

func TestPacers(t *testing.T) {
	p := &FixedPacer{Interval: 20 * time.Millisecond}
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := p.Pace(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("3 calls took %v", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if (&FixedPacer{Interval: time.Hour}).Pace(ctx) == nil || (*FixedPacer)(nil).Pace(ctx) == nil || (NopPacer{}).Pace(ctx) == nil {
		t.Error("cancelled contexts must fail")
	}
	if New(nil, testConfig(nil), nil).Workers != DefaultWorkers {
		t.Error("New must set defaults")
	}
	runPooled(context.Background(), 0, []func(context.Context){func(context.Context) {}})
}
