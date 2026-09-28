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

package stackit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"
	albv2 "github.com/stackitcloud/stackit-sdk-go/services/alb/v2api"
	lbv2 "github.com/stackitcloud/stackit-sdk-go/services/loadbalancer/v2api"
)

var ctx = context.Background()

// router serves canned JSON per "METHOD path" and records requests.
type router struct {
	t      *testing.T
	mu     sync.Mutex
	routes map[string]func(w http.ResponseWriter, r *http.Request)
	seen   []string
	bodies map[string]string
}

func newRouter(t *testing.T) *router {
	return &router{t: t, routes: map[string]func(http.ResponseWriter, *http.Request){}, bodies: map[string]string{}}
}

func (rt *router) json(method, path string, status int, body any) {
	rt.routes[method+" "+path] = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The SDK rejects responses that are not explicitly JSON.
	w.Header().Set("Content-Type", "application/json")
	key := r.Method + " " + r.URL.Path
	body, _ := io.ReadAll(r.Body)
	rt.mu.Lock()
	rt.seen = append(rt.seen, key+"?"+r.URL.RawQuery)
	rt.bodies[key] = string(body)
	h, ok := rt.routes[key]
	rt.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
		return
	}
	h(w, r)
}

func (rt *router) server() string {
	ts := httptest.NewServer(rt)
	rt.t.Cleanup(ts.Close)
	return ts.URL
}

func newSet(t *testing.T, rt *router) *Set {
	t.Helper()
	set, err := New(coreconfig.WithEndpoint(rt.server()), coreconfig.WithoutAuthentication())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return set
}

func project(id, name, parent string, labels map[string]string) map[string]any {
	return map[string]any{
		"containerId": "c-" + id, "creationTime": "2026-01-02T03:04:05Z", "lifecycleState": "ACTIVE",
		"name": name, "projectId": id, "updateTime": "2026-01-02T03:04:05Z", "labels": labels,
		"parent": map[string]any{"containerId": "c-" + parent, "id": parent, "type": "FOLDER"},
	}
}

func folder(id, name, parent string) map[string]any {
	return map[string]any{
		"containerId": "c-" + id, "creationTime": "2026-01-02T03:04:05Z", "folderId": id, "name": name,
		"updateTime": "2026-01-02T03:04:05Z", "labels": map[string]string{"do-not-delete": "true"},
		"parent": map[string]any{"containerId": "c-" + parent, "id": parent, "type": "ORGANIZATION"},
	}
}

func TestListProjectsFollowsPagination(t *testing.T) {
	rt := newRouter(t)
	calls := 0
	rt.routes["GET /v2/projects"] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("containerParentId") != "f1" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		// The API caps the page at 2 and says so.
		items := []any{project("p1", "a", "f1", nil), project("p2", "b", "f1", nil)}
		if r.URL.Query().Get("offset") == "2" {
			items = []any{project("p3", "c", "f1", map[string]string{"k": "v"})}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "limit": 2, "offset": 0})
	}
	got, err := newSet(t, rt).ResourceManager.ListProjects(ctx, "f1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || calls != 2 {
		t.Fatalf("got %d projects in %d calls", len(got), calls)
	}
	if got[2].ID != "p3" || got[2].ParentID != "f1" || got[2].Labels["k"] != "v" || got[2].LifecycleState != "ACTIVE" {
		t.Errorf("project = %+v", got[2])
	}
}

func TestListFoldersStopsOnEmptyPage(t *testing.T) {
	rt := newRouter(t)
	rt.routes["GET /v2/folders"] = func(w http.ResponseWriter, r *http.Request) {
		items := []any{folder("f1", "Platform", "org")}
		if r.URL.Query().Get("offset") != "0" {
			items = []any{}
		}
		// No usable limit echoed: pagination continues until an empty page.
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "limit": 0, "offset": 0})
	}
	got, err := newSet(t, rt).ResourceManager.ListFolders(ctx, "org")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Platform" || !Protected(got[0].Labels) {
		t.Fatalf("folders = %+v", got)
	}
}

func TestPaginateBoundsAndErrors(t *testing.T) {
	n := 0
	err := paginate("x", func(int) (int, float32, error) { n++; return 5, 5, nil })
	if err == nil || n != rmMaxPages {
		t.Errorf("err=%v pages=%d", err, n)
	}
	rt := newRouter(t)
	rt.json("GET", "/v2/folders", 500, map[string]any{"message": "boom"})
	rt.json("GET", "/v2/projects", 500, map[string]any{"message": "boom"})
	set := newSet(t, rt)
	if _, err := set.ResourceManager.ListFolders(ctx, "org"); err == nil {
		t.Error("folders: want error")
	}
	if _, err := set.ResourceManager.ListProjects(ctx, "org"); err == nil {
		t.Error("projects: want error")
	}
}

func TestGetProjectAndFolderWithAncestors(t *testing.T) {
	rt := newRouter(t)
	parents := []any{
		map[string]any{"containerId": "c-org", "id": "org", "name": "Org", "type": "ORGANIZATION"},
		map[string]any{"containerId": "c-top", "id": "top", "name": "Top", "type": "FOLDER", "parentId": "org"},
		map[string]any{"containerId": "c-mid", "id": "mid", "name": "Mid", "type": "FOLDER", "parentId": "top"},
	}
	p := project("p1", "proj", "mid", map[string]string{"a": "b"})
	p["parents"] = parents
	rt.json("GET", "/v2/projects/p1", 200, p)
	f := folder("mid", "Mid", "top")
	f["parent"] = map[string]any{"containerId": "c-top", "id": "top", "type": "FOLDER"}
	f["parents"] = parents[:2]
	rt.json("GET", "/v2/folders/mid", 200, f)
	set := newSet(t, rt)

	got, err := set.ResourceManager.GetProject(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ancestors) != 2 || got.Ancestors[0].Name != "Mid" || got.Ancestors[1].ID != "top" {
		t.Errorf("ancestors = %+v", got.Ancestors)
	}
	if !strings.Contains(rt.seen[0], "includeParents=true") {
		t.Errorf("includeParents not requested: %v", rt.seen)
	}
	fo, err := set.ResourceManager.GetFolder(ctx, "mid")
	if err != nil {
		t.Fatal(err)
	}
	if fo.Name != "Mid" || len(fo.Ancestors) != 1 || fo.Ancestors[0].ID != "top" {
		t.Errorf("folder = %+v", fo)
	}

	_, err = set.ResourceManager.GetProject(ctx, "missing")
	if !IsNotFound(err) {
		t.Errorf("want 404, got %v", err)
	}
	if _, err := set.ResourceManager.GetFolder(ctx, "missing"); !IsNotFound(err) {
		t.Errorf("want 404, got %v", err)
	}
}

const (
	pID    = "aaaaaaaa-0000-0000-0000-000000000001"
	xID    = "bbbbbbbb-0000-0000-0000-000000000001"
	goneID = "bbbbbbbb-0000-0000-0000-000000000002"
	netID  = "cccccccc-0000-0000-0000-000000000001"
	orgID  = "dddddddd-0000-0000-0000-000000000001"
)

const iaasBase = "/v2/projects/" + pID + "/regions/eu01/"

func iaasFixtures(rt *router) {
	rt.json("GET", iaasBase+"servers", 200, map[string]any{"items": []any{
		map[string]any{"id": "s1", "name": "web", "machineType": "t1", "status": "ACTIVE", "labels": map[string]any{"delete": "true"}, "createdAt": "2026-01-01T00:00:00Z"},
	}})
	rt.json("GET", iaasBase+"volumes", 200, map[string]any{"items": []any{
		map[string]any{"id": "v1", "name": "data", "availabilityZone": "eu01-1", "status": "AVAILABLE", "size": 50},
		map[string]any{"id": "v2", "availabilityZone": "eu01-1", "status": "ATTACHED", "serverId": "s1", "labels": map[string]any{"n": 1}},
	}})
	rt.json("GET", iaasBase+"public-ips", 200, map[string]any{"items": []any{
		map[string]any{"id": "ip1", "ip": "192.0.2.1", "networkInterface": nil},
		map[string]any{"id": "ip2", "ip": "192.0.2.2", "networkInterface": "n1"},
	}})
	rt.json("GET", iaasBase+"snapshots", 200, map[string]any{"items": []any{
		map[string]any{"id": "sn1", "name": "snap", "volumeId": "v1", "size": 50},
	}})
	rt.json("GET", iaasBase+"images", 200, map[string]any{"items": []any{
		map[string]any{"id": "i1", "name": "mine", "diskFormat": "qcow2", "owner": pID},
		map[string]any{"id": "i2", "name": "public", "diskFormat": "qcow2", "owner": "ffffffff-0000-0000-0000-000000000000"},
	}})
	rt.json("GET", iaasBase+"nics", 200, map[string]any{"items": []any{
		map[string]any{"id": "n1", "name": "eth0", "networkId": "net1", "device": "s1", "status": "ACTIVE"},
	}})
	rt.json("GET", iaasBase+"security-groups", 200, map[string]any{"items": []any{
		map[string]any{"id": "sg1", "name": "default"},
	}})
}

func TestIaaSListMapsEveryKind(t *testing.T) {
	rt := newRouter(t)
	iaasFixtures(rt)
	iaas := newSet(t, rt).IaaS
	list := func(k Kind) []Resource {
		t.Helper()
		out, err := iaas.List(ctx, k, pID, "eu01")
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		return out
	}
	if s := list(KindServer); len(s) != 1 || s[0].Name != "web" || !Requested(s[0].Labels) || s[0].Status != "ACTIVE" || s[0].CreatedAt.IsZero() {
		t.Errorf("servers = %+v", s)
	}
	v := list(KindVolume)
	if len(v) != 2 || v[0].SizeGB != 50 || v[0].Status != VolumeStatusAvailable || v[1].ServerID != "s1" || v[0].ProjectID != pID || v[0].Region != "eu01" {
		t.Errorf("volumes = %+v", v)
	}
	if len(v[1].Labels) != 0 {
		t.Errorf("non-string label values must be dropped: %v", v[1].Labels)
	}
	ips := list(KindPublicIP)
	if len(ips) != 2 || ips[0].NICID != "" || ips[1].NICID != "n1" || ips[0].Address != "192.0.2.1" || ips[0].Name != "192.0.2.1" {
		t.Errorf("ips = %+v", ips)
	}
	if s := list(KindSnapshot); len(s) != 1 || s[0].VolumeID != "v1" {
		t.Errorf("snapshots = %+v", s)
	}
	if i := list(KindImage); len(i) != 1 || i[0].ID != "i1" {
		t.Errorf("images must be the project's own: %+v", i)
	}
	if n := list(KindNIC); len(n) != 1 || n[0].NetworkID != "net1" || n[0].ServerID != "s1" {
		t.Errorf("nics = %+v", n)
	}
	if g := list(KindSecurityGroup); len(g) != 1 || g[0].Name != "default" {
		t.Errorf("security groups = %+v", g)
	}
	if _, err := iaas.List(ctx, "bogus", "p1", "eu01"); err == nil {
		t.Error("unknown kind must fail")
	}
}

func TestIaaSListErrorsAndNotFound(t *testing.T) {
	iaas := newSet(t, newRouter(t)).IaaS
	for _, k := range Kinds {
		_, err := iaas.List(ctx, k, pID, "eu01")
		if !IsNotFound(err) || !strings.Contains(err.Error(), string(k)) {
			t.Errorf("%s: want wrapped 404, got %v", k, err)
		}
	}
}

func TestIaaSGetAndDeleteEveryKind(t *testing.T) {
	rt := newRouter(t)
	paths := map[Kind]string{
		KindServer:        iaasBase + "servers/" + xID,
		KindVolume:        iaasBase + "volumes/" + xID,
		KindPublicIP:      iaasBase + "public-ips/" + xID,
		KindSnapshot:      iaasBase + "snapshots/" + xID,
		KindImage:         iaasBase + "images/" + xID,
		KindNIC:           iaasBase + "networks/" + netID + "/nics/" + xID,
		KindSecurityGroup: iaasBase + "security-groups/" + xID,
	}
	bodies := map[Kind]any{
		KindServer:        map[string]any{"id": xID, "name": "s", "machineType": "t1", "status": "DELETING"},
		KindVolume:        map[string]any{"id": xID, "availabilityZone": "z", "status": "AVAILABLE"},
		KindPublicIP:      map[string]any{"id": xID, "ip": "192.0.2.9"},
		KindSnapshot:      map[string]any{"id": xID, "volumeId": "v"},
		KindImage:         map[string]any{"id": xID, "name": "i", "diskFormat": "raw"},
		KindNIC:           map[string]any{"id": xID, "networkId": netID},
		KindSecurityGroup: map[string]any{"id": xID, "name": "g"},
	}
	for k, p := range paths {
		rt.json("GET", p, 200, bodies[k])
		rt.json("DELETE", p, 204, nil)
	}
	iaas := newSet(t, rt).IaaS
	for _, k := range Kinds {
		ref := Resource{Kind: k, ID: xID, ProjectID: pID, Region: "eu01", NetworkID: netID}
		got, err := iaas.Get(ctx, ref)
		if err != nil || got.ID != xID || got.Kind != k {
			t.Errorf("get %s: %+v %v", k, got, err)
		}
		if err := iaas.Delete(ctx, ref); err != nil {
			t.Errorf("delete %s: %v", k, err)
		}
		ref.ID = goneID
		if _, err := iaas.Get(ctx, ref); !IsNotFound(err) {
			t.Errorf("get %s: want 404, got %v", k, err)
		}
		if err := iaas.Delete(ctx, ref); !IsNotFound(err) {
			t.Errorf("delete %s: want 404, got %v", k, err)
		}
	}
	bogus := Resource{Kind: "bogus"}
	if _, err := iaas.Get(ctx, bogus); err == nil {
		t.Error("get bogus kind must fail")
	}
	if err := iaas.Delete(ctx, bogus); err == nil {
		t.Error("delete bogus kind must fail")
	}
}

func TestIaaSSetLabelPatchesOnlyThatKey(t *testing.T) {
	rt := newRouter(t)
	rt.json("PATCH", iaasBase+"volumes/"+xID, 200, map[string]any{"id": xID, "availabilityZone": "z"})
	rt.json("PATCH", iaasBase+"public-ips/"+xID, 200, map[string]any{"id": xID})
	iaas := newSet(t, rt).IaaS
	val := LabelTrue
	if err := iaas.SetLabel(ctx, Resource{Kind: KindVolume, ID: xID, ProjectID: pID, Region: "eu01"}, LabelDelete, &val); err != nil {
		t.Fatal(err)
	}
	if got := rt.bodies["PATCH "+iaasBase+"volumes/"+xID]; !strings.Contains(got, `"labels":{"delete":"true"}`) {
		t.Errorf("volume patch = %s", got)
	}
	if err := iaas.SetLabel(ctx, Resource{Kind: KindPublicIP, ID: xID, ProjectID: pID, Region: "eu01"}, LabelDelete, nil); err != nil {
		t.Fatal(err)
	}
	if got := rt.bodies["PATCH "+iaasBase+"public-ips/"+xID]; !strings.Contains(got, `"labels":{"delete":null}`) {
		t.Errorf("ip patch = %s", got)
	}
	if err := iaas.SetLabel(ctx, Resource{Kind: KindServer}, LabelDelete, nil); err == nil {
		t.Error("labels on servers are not supported")
	}
	if err := iaas.SetLabel(ctx, Resource{Kind: KindVolume, ID: goneID, ProjectID: pID, Region: "eu01"}, LabelDelete, nil); !IsNotFound(err) {
		t.Errorf("want 404, got %v", err)
	}
}

func TestIaaSListNetworkAreas(t *testing.T) {
	rt := newRouter(t)
	rt.json("GET", "/v2/organizations/"+orgID+"/network-areas", 200, map[string]any{"items": []any{
		map[string]any{"id": "a1", "name": "hub", "projectCount": 0, "labels": map[string]any{"do-not-delete": "yes"}, "createdAt": "2026-01-01T00:00:00Z"},
	}})
	iaas := newSet(t, rt).IaaS
	areas, err := iaas.ListNetworkAreas(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(areas) != 1 || areas[0].Name != "hub" || areas[0].ProjectCount != 0 || !Protected(areas[0].Labels) {
		t.Errorf("areas = %+v", areas)
	}
	if _, err := iaas.ListNetworkAreas(ctx, goneID); err == nil {
		t.Error("want error")
	}
}

func TestCostProjectCosts(t *testing.T) {
	rt := newRouter(t)
	rt.routes["GET /v3/costs/org"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") != "2026-08-28" || r.URL.Query().Get("to") != "2026-09-26" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"customerAccountId": "org", "projectId": "11111111-1111-1111-1111-111111111111", "projectName": "a", "totalCharge": 12345.0, "totalDiscount": 0.0},
			map[string]any{"customerAccountId": "org", "projectId": "p2", "projectName": "b", "totalCharge": 0.0, "totalDiscount": 0.0},
		})
	}
	set := newSet(t, rt)
	from := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	got, err := set.Cost.ProjectCosts(ctx, "org", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if got["11111111-1111-1111-1111-111111111111"] != 123.45 {
		t.Errorf("costs = %v", got)
	}
	if v, ok := got["p2"]; !ok || v != 0 {
		t.Errorf("zero-cost project missing: %v", got)
	}
	if _, err := set.Cost.ProjectCosts(ctx, "nope", from, to); err == nil {
		t.Error("want error")
	}
}

func TestServicesSKEAndBuckets(t *testing.T) {
	rt := newRouter(t)
	rt.json("GET", "/v2/projects/p1/regions/eu01/clusters", 200, map[string]any{"items": []any{
		map[string]any{"name": "prod", "kubernetes": map[string]any{"version": "1.33"}, "nodepools": []any{}},
	}})
	rt.json("GET", "/v2/project/p1/regions/eu01/buckets", 200, map[string]any{"project": "p1", "buckets": []any{
		map[string]any{"name": "logs", "objectLockEnabled": false, "region": "eu01", "urlPathStyle": "u", "urlVirtualHostedStyle": "u"},
	}})
	rt.json("GET", "/v2/projects/p9/regions/eu01/clusters", 500, map[string]any{"message": "x"})
	rt.json("GET", "/v2/project/p9/regions/eu01/buckets", 500, map[string]any{"message": "x"})
	svc := newSet(t, rt).Services

	if c, err := svc.SKEClusters(ctx, "p1", "eu01"); err != nil || len(c) != 1 || c[0] != "prod" {
		t.Errorf("clusters = %v %v", c, err)
	}
	if b, err := svc.Buckets(ctx, "p1", "eu01"); err != nil || len(b) != 1 || b[0] != "logs" {
		t.Errorf("buckets = %v %v", b, err)
	}
	// Not enabled (404) is "nothing there", not an error.
	if c, err := svc.SKEClusters(ctx, "p2", "eu01"); err != nil || len(c) != 0 {
		t.Errorf("404 clusters = %v %v", c, err)
	}
	if b, err := svc.Buckets(ctx, "p2", "eu01"); err != nil || len(b) != 0 {
		t.Errorf("404 buckets = %v %v", b, err)
	}
	if _, err := svc.SKEClusters(ctx, "p9", "eu01"); err == nil {
		t.Error("500 must fail")
	}
	if _, err := svc.Buckets(ctx, "p9", "eu01"); err == nil {
		t.Error("500 must fail")
	}
}

func lbServices(t *testing.T, lb, alb *router) Services {
	t.Helper()
	opts := func(rt *router) []coreconfig.ConfigurationOption {
		return []coreconfig.ConfigurationOption{coreconfig.WithEndpoint(rt.server()), coreconfig.WithoutAuthentication()}
	}
	lbc, err := lbv2.NewAPIClient(opts(lb)...)
	if err != nil {
		t.Fatal(err)
	}
	albc, err := albv2.NewAPIClient(opts(alb)...)
	if err != nil {
		t.Fatal(err)
	}
	return &services{lb: lbc.DefaultAPI, alb: albc.DefaultAPI}
}

func TestServicesLoadBalancerAddresses(t *testing.T) {
	lb, alb := newRouter(t), newRouter(t)
	path := "/v2/projects/p1/regions/eu01/load-balancers"
	lb.routes["GET "+path] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageId") == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"loadBalancers": []any{
				map[string]any{"name": "web", "externalAddress": "192.0.2.10"},
				map[string]any{"name": "internal"},
			}, "nextPageId": "page-2"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"loadBalancers": []any{
			map[string]any{"name": "api", "externalAddress": "192.0.2.11"},
		}})
	}
	alb.json("GET", path, 200, map[string]any{"loadBalancers": []any{
		map[string]any{"name": "shop", "externalAddress": "192.0.2.12"},
	}})
	got, err := lbServices(t, lb, alb).LoadBalancerAddresses(ctx, "p1", "eu01")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"192.0.2.10": "network load balancer web",
		"192.0.2.11": "network load balancer api",
		"192.0.2.12": "application load balancer shop",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}

	// Neither service enabled: no addresses, no error.
	if got, err := lbServices(t, newRouter(t), newRouter(t)).LoadBalancerAddresses(ctx, "p1", "eu01"); err != nil || len(got) != 0 {
		t.Errorf("404: %v %v", got, err)
	}
}

func TestServicesLoadBalancerErrors(t *testing.T) {
	path := "/v2/projects/p1/regions/eu01/load-balancers"
	failing := newRouter(t)
	failing.json("GET", path, 403, map[string]any{"message": "forbidden"})
	if _, err := lbServices(t, failing, newRouter(t)).LoadBalancerAddresses(ctx, "p1", "eu01"); err == nil {
		t.Error("LB 403 must fail")
	}
	failingALB := newRouter(t)
	failingALB.json("GET", path, 403, map[string]any{"message": "forbidden"})
	if _, err := lbServices(t, newRouter(t), failingALB).LoadBalancerAddresses(ctx, "p1", "eu01"); err == nil {
		t.Error("ALB 403 must fail")
	}
	endless := newRouter(t)
	endless.json("GET", path, 200, map[string]any{"loadBalancers": []any{}, "nextPageId": "again"})
	if _, err := lbServices(t, endless, newRouter(t)).LoadBalancerAddresses(ctx, "p1", "eu01"); err == nil {
		t.Error("endless LB pagination must fail")
	}
	if _, err := lbServices(t, newRouter(t), endless).LoadBalancerAddresses(ctx, "p1", "eu01"); err == nil {
		t.Error("endless ALB pagination must fail")
	}
}

func TestNotEnabled(t *testing.T) {
	apiErr := func(code int, body string) error {
		var e error = &oapierror.GenericOpenAPIError{StatusCode: code, Body: []byte(body), ErrorMessage: fmt.Sprintf("%d", code)}
		return fmt.Errorf("wrapped like the wrappers do: %w", e)
	}
	cases := []struct {
		err  error
		want bool
	}{
		{apiErr(404, ""), true},
		{apiErr(403, `{"message":"Project is not enabled for this service"}`), true},
		{apiErr(400, `{"message":"service NOT ACTIVATED in region"}`), true},
		{apiErr(403, `{"message":"missing permission"}`), false},
		{apiErr(401, `{"message":"not enabled"}`), false},
		{apiErr(500, `{"message":"not enabled"}`), false},
		{io.EOF, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := NotEnabled(c.err); got != c.want {
			t.Errorf("NotEnabled(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestServicesTreatNotEnabledAsEmpty(t *testing.T) {
	rt := newRouter(t)
	notEnabled := map[string]any{"message": "project is not enabled"}
	rt.json("GET", "/v2/projects/p1/regions/eu01/clusters", 403, notEnabled)
	rt.json("GET", "/v2/project/p1/regions/eu01/buckets", 400, notEnabled)
	rt.json("GET", "/v2/projects/p2/regions/eu01/clusters", 403, map[string]any{"message": "forbidden"})
	svc := newSet(t, rt).Services
	if c, err := svc.SKEClusters(ctx, "p1", "eu01"); err != nil || len(c) != 0 {
		t.Errorf("not-enabled SKE = %v %v", c, err)
	}
	if b, err := svc.Buckets(ctx, "p1", "eu01"); err != nil || len(b) != 0 {
		t.Errorf("not-enabled buckets = %v %v", b, err)
	}
	if _, err := svc.SKEClusters(ctx, "p2", "eu01"); err == nil {
		t.Error("a plain 403 must stay an error")
	}

	path := "/v2/projects/p1/regions/eu01/load-balancers"
	lb, alb := newRouter(t), newRouter(t)
	lb.json("GET", path, 403, notEnabled)
	alb.json("GET", path, 400, notEnabled)
	if got, err := lbServices(t, lb, alb).LoadBalancerAddresses(ctx, "p1", "eu01"); err != nil || len(got) != 0 {
		t.Errorf("not-enabled load balancers = %v %v", got, err)
	}
}

func TestDescribe(t *testing.T) {
	apiErr := func(code int, body string) error {
		var e error = &oapierror.GenericOpenAPIError{StatusCode: code, Body: []byte(body), ErrorMessage: "raw"}
		return fmt.Errorf("deleting server x: %w", e)
	}
	cases := map[string]error{
		"HTTP 403: missing permission iaas.server.delete": apiErr(403, `{"message":" missing permission iaas.server.delete "}`),
		"HTTP 500":       apiErr(500, `not json`),
		"HTTP 409":       apiErr(409, `{"code":409}`),
		"something else": errors.New("something else"),
		"HTTP 400: " + strings.Repeat("ü", maxDescribed) + "…": apiErr(400, `{"message":"`+strings.Repeat("ü", maxDescribed+50)+`"}`),
	}
	for want, err := range cases {
		if got := Describe(err); got != want {
			t.Errorf("Describe(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestLabelHelpers(t *testing.T) {
	cases := []struct {
		labels               map[string]string
		requested, protected bool
	}{
		{nil, false, false},
		{map[string]string{"delete": "true"}, true, false},
		{map[string]string{"delete": "TRUE"}, true, false},
		{map[string]string{"delete": "yes"}, false, false},
		{map[string]string{"do-not-delete": "true"}, false, true},
		{map[string]string{"do-not-delete": ""}, false, true},
		{map[string]string{"do-not-delete": "False"}, false, false},
	}
	for _, c := range cases {
		if Requested(c.labels) != c.requested || Protected(c.labels) != c.protected {
			t.Errorf("%v: requested=%v protected=%v", c.labels, Requested(c.labels), Protected(c.labels))
		}
	}
	if _, ok := StatusCode(io.EOF); ok || IsNotFound(nil) {
		t.Error("non-API errors have no status")
	}
	if copyLabels(nil) != nil || stringLabels(nil) != nil {
		t.Error("nil labels must stay nil")
	}
}
