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
	"net/http"
	"testing"
)

// jsonEncode is a small helper mirroring the pattern in stackit_test.go.
func jsonEncode(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
}

const (
	testProject = "11111111-1111-1111-1111-111111111111"
	testRegion  = "eu01"
	testIP      = "22222222-2222-2222-2222-222222222222"
	testVolume  = "33333333-3333-3333-3333-333333333333"
)

func TestIaaSGetPublicIP(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/projects/"+testProject+"/regions/"+testRegion+"/public-ips/"+testIP {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		_ = jsonEncode(w, map[string]any{"id": testIP, "ip": "203.0.113.7", "networkInterface": "nic-9", "labels": map[string]any{MarkLabelKey: "20260922T091503Z"}})
	}))
	ip, err := set.IaaS.GetPublicIP(context.Background(), testProject, testRegion, testIP)
	if err != nil {
		t.Fatalf("GetPublicIP: %v", err)
	}
	if ip.Address != "203.0.113.7" || ip.AttachedNIC != "nic-9" || ip.ProjectID != testProject || ip.Region != testRegion {
		t.Errorf("ip = %+v", ip)
	}
	if _, ok := MarkDeadline(ip.Labels); !ok {
		t.Errorf("labels lost in mapping: %+v", ip.Labels)
	}
}

func TestIaaSGetPublicIPIdleAndError(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{"id": testIP, "ip": "203.0.113.7", "networkInterface": nil})
	}))
	ip, err := set.IaaS.GetPublicIP(context.Background(), testProject, testRegion, testIP)
	if err != nil {
		t.Fatalf("GetPublicIP: %v", err)
	}
	if ip.AttachedNIC != "" {
		t.Errorf("idle IP must map to empty NIC: %+v", ip)
	}

	set404 := fakeStackit(t, jsonHandler(t, http.StatusNotFound, map[string]any{"error": "gone"}))
	if _, err := set404.IaaS.GetPublicIP(context.Background(), testProject, testRegion, testIP); err == nil {
		t.Fatal("expected 404 error")
	}
}

func TestIaaSGetVolume(t *testing.T) {
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/projects/"+testProject+"/regions/"+testRegion+"/volumes/"+testVolume {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		_ = jsonEncode(w, map[string]any{"id": testVolume, "name": "data", "size": 250, "status": "AVAILABLE", "serverId": nil, "availabilityZone": "eu01-1", "createdAt": "2026-01-01T00:00:00Z"})
	}))
	v, err := set.IaaS.GetVolume(context.Background(), testProject, testRegion, testVolume)
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if v.SizeGB != 250 || v.Status != "AVAILABLE" || v.ServerID != "" || v.Name != "data" {
		t.Errorf("volume = %+v", v)
	}
	if v.CreatedAt.IsZero() {
		t.Error("CreatedAt not parsed")
	}

	set404 := fakeStackit(t, jsonHandler(t, http.StatusNotFound, map[string]any{"error": "gone"}))
	if _, err := set404.IaaS.GetVolume(context.Background(), testProject, testRegion, testVolume); err == nil {
		t.Fatal("expected 404 error")
	}
}

// TestIaaSAllErrorPaths drives every IaaS method against a failing API to
// cover the error branches.
func TestIaaSAllErrorPaths(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusInternalServerError, map[string]any{"error": "boom"}))
	ctx := context.Background()
	cases := []struct {
		name string
		call func() error
	}{
		{"ListNetworkAreas", func() error { _, err := set.IaaS.ListNetworkAreas(ctx, "org"); return err }},
		{"GetNetworkArea", func() error { _, err := set.IaaS.GetNetworkArea(ctx, "org", "sna"); return err }},
		{"ListPublicIPs", func() error { _, err := set.IaaS.ListPublicIPs(ctx, testProject, testRegion); return err }},
		{"GetPublicIP", func() error { _, err := set.IaaS.GetPublicIP(ctx, testProject, testRegion, testIP); return err }},
		{"SetPublicIPMark", func() error { return set.IaaS.SetPublicIPMark(ctx, testProject, testRegion, testIP, "x") }},
		{"ClearPublicIPMark", func() error { return set.IaaS.ClearPublicIPMark(ctx, testProject, testRegion, testIP) }},
		{"DeletePublicIP", func() error { return set.IaaS.DeletePublicIP(ctx, testProject, testRegion, testIP) }},
		{"ListVolumes", func() error { _, err := set.IaaS.ListVolumes(ctx, testProject, testRegion); return err }},
		{"GetVolume", func() error { _, err := set.IaaS.GetVolume(ctx, testProject, testRegion, testVolume); return err }},
		{"SetVolumeMark", func() error { return set.IaaS.SetVolumeMark(ctx, testProject, testRegion, testVolume, "x") }},
		{"ClearVolumeMark", func() error { return set.IaaS.ClearVolumeMark(ctx, testProject, testRegion, testVolume) }},
		{"DeleteVolume", func() error { return set.IaaS.DeleteVolume(ctx, testProject, testRegion, testVolume) }},
		{"ListSnapshots", func() error { _, err := set.IaaS.ListSnapshots(ctx, testProject, testRegion); return err }},
		{"DeleteSnapshot", func() error { return set.IaaS.DeleteSnapshot(ctx, testProject, testRegion, "snap") }},
		{"ListServers", func() error { _, err := set.IaaS.ListServers(ctx, testProject, testRegion); return err }},
	}
	for _, c := range cases {
		if err := c.call(); err == nil {
			t.Errorf("%s: expected error against a failing API", c.name)
		}
	}
}

// TestIaaSListEndpointsEmptyResponses covers the "no items" branches. The
// SDK validates responses against its schema, so the list wrappers must be
// present (empty).
func TestIaaSListEndpointsEmptyResponses(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusOK, map[string]any{"items": []any{}, "limit": 20, "offset": 0}))
	ctx := context.Background()
	if ips, err := set.IaaS.ListPublicIPs(ctx, testProject, testRegion); err != nil || len(ips) != 0 {
		t.Errorf("ListPublicIPs empty: %v %d", err, len(ips))
	}
	if vols, err := set.IaaS.ListVolumes(ctx, testProject, testRegion); err != nil || len(vols) != 0 {
		t.Errorf("ListVolumes empty: %v %d", err, len(vols))
	}
	if snaps, err := set.IaaS.ListSnapshots(ctx, testProject, testRegion); err != nil || len(snaps) != 0 {
		t.Errorf("ListSnapshots empty: %v %d", err, len(snaps))
	}
	if servers, err := set.IaaS.ListServers(ctx, testProject, testRegion); err != nil || len(servers) != 0 {
		t.Errorf("ListServers empty: %v %d", err, len(servers))
	}
	if areas, err := set.IaaS.ListNetworkAreas(ctx, "77777777-7777-7777-7777-777777777777"); err != nil || len(areas) != 0 {
		t.Errorf("ListNetworkAreas empty: %v %d", err, len(areas))
	}
}

func TestResourceManagerErrorAndEmptyPaths(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusInternalServerError, map[string]any{"error": "boom"}))
	ctx := context.Background()
	if _, err := set.ResourceManager.ListFolders(ctx, "org"); err == nil {
		t.Error("ListFolders: expected error")
	}
	if _, err := set.ResourceManager.GetFolder(ctx, "fid"); err == nil {
		t.Error("GetFolder: expected error")
	}

	empty := fakeStackit(t, jsonHandler(t, http.StatusOK, map[string]any{"items": []any{}, "limit": 20, "offset": 0}))
	if projects, err := empty.ResourceManager.ListProjects(ctx, "org"); err != nil || len(projects) != 0 {
		t.Errorf("ListProjects empty: %v %d", err, len(projects))
	}
	if folders, err := empty.ResourceManager.ListFolders(ctx, "org"); err != nil || len(folders) != 0 {
		t.Errorf("ListFolders empty: %v %d", err, len(folders))
	}
}

func TestSKEAndObjectStorageErrorPaths(t *testing.T) {
	set := fakeStackit(t, jsonHandler(t, http.StatusInternalServerError, map[string]any{"error": "boom"}))
	ctx := context.Background()
	if _, err := set.SKE.ListClusterNames(ctx, testProject); err == nil {
		t.Error("ListClusterNames: expected error")
	}
	if _, err := set.ObjectStorage.ListBucketNames(ctx, testProject); err == nil {
		t.Error("ListBucketNames: expected error")
	}

	empty := fakeStackit(t, jsonHandler(t, http.StatusOK, map[string]any{"items": []any{}, "buckets": []any{}, "project": testProject}))
	if clusters, err := empty.SKE.ListClusterNames(ctx, testProject); err != nil || len(clusters) != 0 {
		t.Errorf("ListClusterNames empty: %v %d", err, len(clusters))
	}
	if buckets, err := empty.ObjectStorage.ListBucketNames(ctx, testProject); err != nil || len(buckets) != 0 {
		t.Errorf("ListBucketNames empty: %v %d", err, len(buckets))
	}
}

func TestCostListSummarizedVariants(t *testing.T) {
	// The SDK's anyOf(ProjectCost) unmarshal tries the variants in a fixed
	// order and matches the first one whose custom unmarshal succeeds.
	// Type-mismatching the earlier variants' list fields steers the match:
	// A: services+reportData broken -> matches summarized-services
	// B: only services broken -> matches the reports variant,
	// which cost.go deliberately skips (default branch)
	set := fakeStackit(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, []any{
			map[string]any{
				"customerAccountId": "org-1", "projectId": "p-sum", "projectName": "Sum",
				"totalCharge": 2000.0, "totalDiscount": 0.0,
				"services": "oops", "reportData": "oops",
			},
			map[string]any{
				"customerAccountId": "org-1", "projectId": "p-rep", "projectName": "Rep",
				"totalCharge": 100.0, "totalDiscount": 0.0,
				"services": "oops",
			},
		})
	}))
	records, err := set.Cost.ListCostsForCustomer(context.Background(), "org-1", mustDate("2026-09-01"), mustDate("2026-09-01"))
	if err != nil {
		t.Fatalf("ListCostsForCustomer: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %+v (want only the summarized-services entry; the reports variant is skipped)", records)
	}
	if records[0].ProjectID != "p-sum" || records[0].ProjectName != "Sum" || records[0].ChargeEUR != 20.0 {
		t.Errorf("record[0] = %+v", records[0])
	}
}
