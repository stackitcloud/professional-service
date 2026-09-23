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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
)

// fakeStackit points the real SDK clients (no auth) at a test server so
// the wrapper mapping logic is exercised end-to-end without touching the
// real STACKIT APIs.
func fakeStackit(t *testing.T, handler http.Handler) *Set {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pre-set the content type: the SDK client rejects responses whose
		// Content-Type is not explicitly JSON (short bodies would be sniffed
		// as text/plain by net/http).
		w.Header().Set("Content-Type", "application/json")
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	set, err := New(coreconfig.WithEndpoint(ts.URL), coreconfig.WithoutAuthentication())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return set
}

func jsonHandler(t *testing.T, status int, body any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

func TestNewBuildsAllClients(t *testing.T) {
	set := fakeStackit(t, http.NotFoundHandler())
	if set.ResourceManager == nil || set.IaaS == nil || set.Cost == nil || set.SKE == nil || set.ObjectStorage == nil {
		t.Fatal("New must build all five clients")
	}
}

func TestResourceManagerListProjects(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/projects" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("containerParentId") != "org-1" {
			t.Errorf("containerParentId = %q", r.URL.Query().Get("containerParentId"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"containerId":    "friendly-1",
					"creationTime":   "2026-01-02T03:04:05Z",
					"labels":         map[string]string{"k": "v"},
					"lifecycleState": "ACTIVE",
					"name":           "proj-a",
					"parent":         map[string]any{"containerId": "org-1", "id": "org-1", "name": "Org", "type": "organization"},
					"projectId":      "pid-1",
					"updateTime":     "2026-01-02T00:00:00Z",
				},
				map[string]any{
					"containerId":    "friendly-2",
					"creationTime":   "2026-01-01T00:00:00Z",
					"lifecycleState": "DELETED",
					"name":           "proj-b",
					"parent":         map[string]any{"containerId": "org-1", "id": "org-1", "name": "Org", "type": "organization"},
					"projectId":      "pid-2",
					"updateTime":     "2026-01-01T00:00:00Z",
				},
			},
			"limit": 20, "offset": 0,
		})
	}))
	projects, err := set.ResourceManager.ListProjects(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %v", projects)
	}
	if projects[0].ID != "pid-1" || projects[0].Name != "proj-a" || projects[0].LifecycleState != "ACTIVE" {
		t.Errorf("project[0] = %+v", projects[0])
	}
	if projects[0].ParentContainerID != "org-1" {
		t.Errorf("parent id = %q", projects[0].ParentContainerID)
	}
	if projects[0].ParentContainerName != "" {
		t.Errorf("parent name must come from the walk, not the API: %q", projects[0].ParentContainerName)
	}
	if projects[0].Labels["k"] != "v" {
		t.Errorf("labels = %v", projects[0].Labels)
	}
	if projects[0].CreationTime.IsZero() {
		t.Error("CreationTime not parsed")
	}
}

func TestResourceManagerListProjectsError(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusInternalServerError, map[string]any{"error": "boom"}))
	if _, err := set.ResourceManager.ListProjects(context.Background(), "org-1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResourceManagerListFolders(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"containerId":  "friendly-f",
					"creationTime": "2026-01-01T00:00:00Z",
					"folderId":     "fid-1",
					"labels":       map[string]string{"do-not-delete": "true"},
					"name":         "folder-a",
					"parent":       map[string]any{"containerId": "org-1", "id": "org-1", "name": "Org", "type": "organization"},
					"updateTime":   "2026-01-01T00:00:00Z",
				},
			},
			"limit": 20, "offset": 0,
		})
	}))
	folders, err := set.ResourceManager.ListFolders(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(folders) != 1 || folders[0].ID != "fid-1" || folders[0].Name != "folder-a" {
		t.Fatalf("folders = %+v", folders)
	}
	if folders[0].Labels["do-not-delete"] != "true" {
		t.Errorf("labels = %v", folders[0].Labels)
	}
}

func TestResourceManagerGetProject(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/projects/pid-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"containerId": "friendly-1", "creationTime": "2026-01-01T00:00:00Z",
			"labels": map[string]string{}, "lifecycleState": "ACTIVE", "name": "p", "updateTime": "2026-01-02T00:00:00Z", "id": "pid-1",
			"parent": map[string]any{"containerId": "org-1", "id": "org-1", "name": "O", "type": "organization"}, "projectId": "pid-1",
		})
	}))
	p, err := set.ResourceManager.GetProject(context.Background(), "pid-1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.ID != "pid-1" || p.Name != "p" {
		t.Errorf("project = %+v", p)
	}
}

func TestResourceManagerGetProjectNotFound(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusNotFound, map[string]any{"error": "not found"}))
	if _, err := set.ResourceManager.GetProject(context.Background(), "missing"); err == nil {
		t.Fatal("expected 404 error")
	}
}

func TestResourceManagerGetFolder(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/folders/fid-9" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"containerId": "friendly-f", "creationTime": "2026-01-01T00:00:00Z", "folderId": "fid-9",
			"labels": map[string]string{}, "name": "folder-x", "updateTime": "2026-01-02T00:00:00Z", "id": "fid-9",
			"parent": map[string]any{"containerId": "org-1", "id": "org-1", "name": "O", "type": "organization"},
		})
	}))
	f, err := set.ResourceManager.GetFolder(context.Background(), "fid-9")
	if err != nil {
		t.Fatalf("GetFolder: %v", err)
	}
	if f.ID != "fid-9" || f.Name != "folder-x" {
		t.Errorf("folder = %+v", f)
	}
}

func TestIaaSListPublicIPsAttachedAndIdle(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/public-ips" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": "ip-attached", "ip": "1.2.3.4", "networkInterface": "nic-1"},
				map[string]any{"id": "ip-idle", "ip": "5.6.7.8", "networkInterface": nil},
				map[string]any{"id": "ip-nil", "ip": "9.9.9.9"},
			},
		})
	}))
	ips, err := set.IaaS.ListPublicIPs(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01")
	if err != nil {
		t.Fatalf("ListPublicIPs: %v", err)
	}
	if len(ips) != 3 {
		t.Fatalf("ips = %+v", ips)
	}
	if ips[0].AttachedNIC != "nic-1" {
		t.Errorf("attached IP: %+v", ips[0])
	}
	if ips[1].AttachedNIC != "" || ips[2].AttachedNIC != "" {
		t.Errorf("idle IPs must have empty NIC: %+v / %+v", ips[1], ips[2])
	}
	if ips[0].ProjectID != "11111111-1111-1111-1111-111111111111" || ips[0].Region != "eu01" {
		t.Errorf("project/region not filled: %+v", ips[0])
	}
}

func TestIaaSUpdatePublicIPLabelsSetsAndClearsMark(t *testing.T) {
	var lastBody map[string]any
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/public-ips/22222222-2222-2222-2222-222222222222" {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		lastBody = map[string]any{}
		_ = json.Unmarshal(body, &lastBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "22222222-2222-2222-2222-222222222222"})
	}))
	if err := set.IaaS.SetPublicIPMark(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "22222222-2222-2222-2222-222222222222", "20260922T091503Z"); err != nil {
		t.Fatalf("SetPublicIPMark: %v", err)
	}
	labels, _ := lastBody["labels"].(map[string]any)
	if labels[MarkLabelKey] != "20260922T091503Z" {
		t.Errorf("mark payload = %v", lastBody)
	}
	if err := set.IaaS.ClearPublicIPMark(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("ClearPublicIPMark: %v", err)
	}
	labels, _ = lastBody["labels"].(map[string]any)
	if _, present := labels[MarkLabelKey]; !present || labels[MarkLabelKey] != nil {
		t.Errorf("clear payload must send null for the mark key: %v", lastBody)
	}
}

func TestIaaSDeletePublicIP(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/public-ips/22222222-2222-2222-2222-222222222222" {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := set.IaaS.DeletePublicIP(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("DeletePublicIP: %v", err)
	}
}

func TestIaaSListVolumes(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": "33333333-3333-3333-3333-333333333333", "name": "data", "size": 100, "status": "AVAILABLE", "serverId": nil, "availabilityZone": "eu01-1",
					"labels": map[string]any{"k": "v"}, "createdAt": "2026-01-01T00:00:00Z"},
				map[string]any{"id": "44444444-4444-4444-4444-444444444444", "name": "boot", "size": 50, "status": "ATTACHED", "serverId": "srv-1", "availabilityZone": "eu01-1"},
			},
		})
	}))
	vols, err := set.IaaS.ListVolumes(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01")
	if err != nil {
		t.Fatalf("ListVolumes: %v", err)
	}
	if len(vols) != 2 {
		t.Fatalf("vols = %+v", vols)
	}
	if vols[0].SizeGB != 100 || vols[0].Status != "AVAILABLE" || vols[0].ServerID != "" {
		t.Errorf("vol[0] = %+v", vols[0])
	}
	if vols[0].Labels["k"] != "v" || vols[0].CreatedAt.IsZero() {
		t.Errorf("labels/createdAt: %+v", vols[0])
	}
	if vols[1].ServerID != "srv-1" {
		t.Errorf("vol[1] = %+v", vols[1])
	}
}

func TestIaaSUpdateVolumeLabels(t *testing.T) {
	var lastBody map[string]any
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/volumes/33333333-3333-3333-3333-333333333333" {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		lastBody = map[string]any{}
		_ = json.Unmarshal(body, &lastBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "33333333-3333-3333-3333-333333333333", "availabilityZone": "eu01-1"})
	}))
	if err := set.IaaS.SetVolumeMark(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "33333333-3333-3333-3333-333333333333", "20260922T091503Z"); err != nil {
		t.Fatalf("SetVolumeMark: %v", err)
	}
	if labels, _ := lastBody["labels"].(map[string]any); labels[MarkLabelKey] != "20260922T091503Z" {
		t.Errorf("mark payload = %v", lastBody)
	}
	if err := set.IaaS.ClearVolumeMark(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("ClearVolumeMark: %v", err)
	}
	if labels, _ := lastBody["labels"].(map[string]any); labels[MarkLabelKey] != nil {
		t.Errorf("clear payload = %v", lastBody)
	}
}

func TestIaaSDeleteVolumeAndSnapshots(t *testing.T) {
	var deleted []string
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/volumes/33333333-3333-3333-3333-333333333333":
			deleted = append(deleted, "33333333-3333-3333-3333-333333333333")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/snapshots/55555555-5555-5555-5555-555555555555":
			deleted = append(deleted, "55555555-5555-5555-5555-555555555555")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/projects/11111111-1111-1111-1111-111111111111/regions/eu01/snapshots":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": "55555555-5555-5555-5555-555555555555", "name": "s", "volumeId": "33333333-3333-3333-3333-333333333333"},
				map[string]any{"id": "66666666-6666-6666-6666-666666666666", "name": "s", "volumeId": "44444444-4444-4444-4444-444444444444"},
			}})
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	snaps, err := set.IaaS.ListSnapshots(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	var forVol []Snapshot
	for _, s := range snaps {
		if s.VolumeID == "33333333-3333-3333-3333-333333333333" {
			forVol = append(forVol, s)
		}
	}
	if len(forVol) != 1 || forVol[0].ID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("filter = %+v", forVol)
	}
	if err := set.IaaS.DeleteSnapshot(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "55555555-5555-5555-5555-555555555555"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	if err := set.IaaS.DeleteVolume(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01", "33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("DeleteVolume: %v", err)
	}
	if len(deleted) != 2 || deleted[0] != "55555555-5555-5555-5555-555555555555" || deleted[1] != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("deleted order = %v (snapshot must precede volume)", deleted)
	}
}

func TestIaaSListServers(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": "srv-1", "name": "web-1", "status": "ONLINE", "machineType": "c2-medium"},
				map[string]any{"id": "srv-2", "name": "web-2", "status": "OFFLINE", "machineType": "c2-medium"},
			},
		})
	}))
	servers, err := set.IaaS.ListServers(context.Background(), "11111111-1111-1111-1111-111111111111", "eu01")
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 2 || servers[0].Status != "ONLINE" {
		t.Errorf("servers = %+v", servers)
	}
}

func TestIaaSNetworkAreas(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/organizations/77777777-7777-7777-7777-777777777777/network-areas":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": "sna-1", "name": "empty-area", "projectCount": 0, "labels": map[string]any{}, "createdAt": "2026-01-01T00:00:00Z"},
				map[string]any{"id": "sna-2", "name": "busy-area", "projectCount": 3},
			}})
		case "/v2/organizations/77777777-7777-7777-7777-777777777777/network-areas/88888888-8888-8888-8888-888888888888":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "88888888-8888-8888-8888-888888888888", "name": "empty-area", "projectCount": 0, "createdAt": "2026-01-01T00:00:00Z"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	areas, err := set.IaaS.ListNetworkAreas(context.Background(), "77777777-7777-7777-7777-777777777777")
	if err != nil {
		t.Fatalf("ListNetworkAreas: %v", err)
	}
	if len(areas) != 2 || areas[0].ProjectCount != 0 || areas[1].ProjectCount != 3 {
		t.Fatalf("areas = %+v", areas)
	}
	na, err := set.IaaS.GetNetworkArea(context.Background(), "77777777-7777-7777-7777-777777777777", "88888888-8888-8888-8888-888888888888")
	if err != nil {
		t.Fatalf("GetNetworkArea: %v", err)
	}
	if na.ID != "88888888-8888-8888-8888-888888888888" || na.Name != "empty-area" {
		t.Errorf("area = %+v", na)
	}
}

func TestCostListCostsForCustomer(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/costs/org-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("from") != "2026-09-01" || r.URL.Query().Get("to") != "2026-09-01" {
			t.Errorf("from/to = %s/%s", r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		}
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"customerAccountId": "org-1", "projectId": "11111111-1111-1111-1111-111111111111", "projectName": "proj-a", "totalCharge": 12345.0, "totalDiscount": 0.0},
			map[string]any{"customerAccountId": "org-1", "projectId": "p2", "projectName": "proj-b", "totalCharge": 100.0, "totalDiscount": 0.0},
		})
	}))
	records, err := set.Cost.ListCostsForCustomer(context.Background(), "org-1", mustDate("2026-09-01"), mustDate("2026-09-01"))
	if err != nil {
		t.Fatalf("ListCostsForCustomer: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	if records[0].ChargeEUR != 123.45 {
		t.Errorf("cents->EUR conversion = %v (want 123.45)", records[0].ChargeEUR)
	}
	if records[0].ProjectID != "11111111-1111-1111-1111-111111111111" || records[1].ProjectName != "proj-b" {
		t.Errorf("records = %+v", records)
	}
}

func TestCostError(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusBadGateway, map[string]any{"error": "upstream"}))
	if _, err := set.Cost.ListCostsForCustomer(context.Background(), "org-1", mustDate("2026-09-01"), mustDate("2026-09-01")); err == nil {
		t.Fatal("expected error")
	}
}

func mustDate(s string) (t time.Time) {
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return parsed
}
