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
	"net/http/httptest"
	"testing"
	"time"

	coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
)

func TestSKEDelegateWrappers(t *testing.T) {
	var skeSet *Set
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/projects/11111111-1111-1111-1111-111111111111/clusters":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"name": "prod-cluster", "kubernetes": map[string]any{"version": "1.30.1"}, "nodepools": []any{}},
					map[string]any{"name": "staging-cluster", "kubernetes": map[string]any{"version": "1.31.0"}, "nodepools": []any{}},
				},
			})
		case "/v1/project/11111111-1111-1111-1111-111111111111/buckets":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"buckets": []any{
					map[string]any{"name": "logs", "objectLockEnabled": false, "region": "eu01", "urlPathStyle": "https://s3/logs", "urlVirtualHostedStyle": "https://logs.s3"},
					map[string]any{"name": "artifacts", "objectLockEnabled": false, "region": "eu01", "urlPathStyle": "https://s3/artifacts", "urlVirtualHostedStyle": "https://artifacts.s3"},
				},
				"project": "11111111-1111-1111-1111-111111111111",
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	var err error
	skeSet, err = New(coreconfig.WithEndpoint(ts.URL), coreconfig.WithoutAuthentication())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	clusters, err := skeSet.SKE.ListClusterNames(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("ListClusterNames: %v", err)
	}
	if len(clusters) != 2 || clusters[0] != "prod-cluster" {
		t.Errorf("clusters = %v", clusters)
	}
	buckets, err := skeSet.ObjectStorage.ListBucketNames(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("ListBucketNames: %v", err)
	}
	if len(buckets) != 2 || buckets[0] != "logs" {
		t.Errorf("buckets = %v", buckets)
	}
}

func TestMarkFormatAndParseRoundTrip(t *testing.T) {
	deadline := time.Date(2026, 9, 22, 9, 15, 3, 0, time.UTC)
	value := FormatMark(deadline)
	if value != "20260922T091503Z" {
		t.Fatalf("FormatMark = %q (want compact UTC, no colons — colons are rejected by the label APIs)", value)
	}
	back, err := ParseMark(value)
	if err != nil {
		t.Fatalf("ParseMark: %v", err)
	}
	if !back.Equal(deadline) {
		t.Errorf("round trip = %v", back)
	}
}

func TestMarkParseRejectsRFC3339(t *testing.T) {
	// Raw RFC3339 contains colons and is rejected by both label APIs;
	// ParseMark must treat it as malformed.
	if _, err := ParseMark("2026-09-22T09:15:03Z"); err == nil {
		t.Error("RFC3339 value must not parse")
	}
}

func TestMarkDeadline(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		want   time.Time
		wantOK bool
	}{
		{"absent", map[string]string{}, time.Time{}, false},
		{"nil labels", nil, time.Time{}, false},
		{"empty value", map[string]string{MarkLabelKey: ""}, time.Time{}, false},
		{"malformed", map[string]string{MarkLabelKey: "yesterday"}, time.Time{}, false},
		{"valid", map[string]string{MarkLabelKey: "20260922T091503Z"}, time.Date(2026, 9, 22, 9, 15, 3, 0, time.UTC), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MarkDeadline(tc.labels)
			if ok != tc.wantOK || (ok && !got.Equal(tc.want)) {
				t.Errorf("MarkDeadline = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
