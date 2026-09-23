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

package slack

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func sampleReport() *report.Report {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	mark := now.Add(8 * time.Hour)
	return &report.Report{
		GeneratedAt: now,
		DryRun:      true,
		Scope:       "organisation",
		StaleProjects: []report.StaleProject{
			{ID: "p1", Name: "old-a", AgeDays: 120, CreatedAt: now.AddDate(0, 0, -120), ParentFolderID: "f1", ParentFolderName: "Team A", SKEClusters: []string{"prod"}},
		},
		EmptySNAs: []report.EmptySNA{
			{ID: "sna-1", Name: "empty-1", AgeDays: 40, CreatedAt: now.AddDate(0, 0, -40)},
		},
		IdlePublicIPs: []report.IdlePublicIP{
			{ID: "ip-1", ProjectID: "pp1", ProjectName: "proj-a", Region: "eu01", Address: "1.2.3.4", MarkDeadline: &mark},
		},
		DetachedVolumes: []report.DetachedVolume{
			{ID: "vol-1", Name: "data", ProjectID: "pp1", ProjectName: "proj-a", Region: "eu01", SizeGB: 100, EstimatedMonthlyCostEUR: 6.19, MarkDeadline: &mark},
		},
		TopProjects:                []report.ProjectCost{{ProjectID: "pp1", ProjectName: "proj-a", Cost30dEUR: 440.0}},
		AnomalyDetected:            true,
		AnomalyPct:                 133.33,
		TotalCost30dEUR:            590.0,
		AvgCostPerDayEUR:           19.67,
		ChartURL:                   "https://s3.example/chart.png",
		ScanErrors:                 []string{"boom"},
		EstimatedMonthlySavingsEUR: 15.83,
		EstimatedDailySavingsEUR:   0.53,
	}
}

func TestSendReportBlocks(t *testing.T) {
	var blocks []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Errorf("payload not JSON: %v", err)
		}
		blocks = extractBlocks(payload)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	n := New(srv.URL, nil, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("no blocks in payload")
	}
	if blocks[0]["type"] != "header" {
		t.Errorf("first block type = %v, want header", blocks[0]["type"])
	}
	all := blockText(blocks)
	for _, want := range []string{
		"Stale projects (1)", "Empty network areas (1)", "Idle public IPs (1)",
		"Detached volumes (1)", "Cost anomaly detected", "Top 10 projects",
		"Costs, last 30 days", "15.83",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in blocks: %s", want, all)
		}
	}
	// Summary line first within the idle-IP group: the header
	// block is followed by a summary section before the rows.
	i := indexOfType(blocks, "header", "Idle public IPs (1)")
	if i < 0 {
		t.Fatal("idle IP header block not found")
	}
	if i+1 >= len(blocks) || blocks[i+1]["type"] != "section" {
		t.Errorf("block after IP header is %v, want a summary section", blocks[i+1]["type"])
	}
}

func TestSendReportButtonLinksSigned(t *testing.T) {
	var blocks []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		blocks = extractBlocks(payload)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	btn := notifier.NewButton("https://cb.example.com", "s3cr3t")
	n := New(srv.URL, btn, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	linkRe := regexp.MustCompile(`\[Do not delete\]\((https://cb\.example\.com/protect\?[^)]+)\)`)
	links := linkRe.FindAllStringSubmatch(blockText(blocks), -1)
	// 1 stale project + 1 IP + 1 volume = 3 links.
	if len(links) != 3 {
		t.Fatalf("links = %d, want 3: %v", len(links), links)
	}
	for _, m := range links {
		p := parseProtect(t, m[1])
		if !notifier.VerifySig(btn.Secret, p.Get(notifier.ParamID), p.Get(notifier.ParamType), p.Get(notifier.ParamProject), p.Get(notifier.ParamRegion), pExp(p), p.Get(notifier.ParamSig)) {
			t.Errorf("link signature invalid: %s", m[1])
		}
	}
}

func TestSendReportNoLinksWhenDisabled(t *testing.T) {
	var blocks []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		blocks = extractBlocks(payload)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	n := New(srv.URL, nil, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if strings.Contains(blockText(blocks), "[Do not delete]") {
		t.Error("protect link rendered without callback configured")
	}
}

func TestSendDeletionSummary(t *testing.T) {
	var blocks []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		blocks = extractBlocks(payload)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	n := New(srv.URL, nil, testLogger())
	summary := &report.DeletionSummary{
		GeneratedAt: time.Now(),
		Results: []report.DeletionResult{
			{Item: report.DeletionItem{Kind: "volume", Name: "v1", ProjectName: "p", Region: "eu01"}, Status: report.StatusDeleted},
			{Item: report.DeletionItem{Kind: "publicip", Name: "i1", ProjectName: "p", Region: "eu01"}, Status: report.StatusFailed, Reason: "boom"},
		},
	}
	if err := n.SendDeletionSummary(context.Background(), summary); err != nil {
		t.Fatalf("SendDeletionSummary: %v", err)
	}
	text := blockText(blocks)
	for _, want := range []string{"Deleted 1", "failed 1", "boom"} {
		if !strings.Contains(text, want) {
			t.Errorf("deletion summary missing %q", want)
		}
	}
}

func TestSendReportWebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	t.Cleanup(srv.Close)
	n := New(srv.URL, nil, testLogger())
	if err := n.SendReport(context.Background(), sampleReport()); err == nil {
		t.Error("SendReport = nil, want error on 403")
	}
}

// ---- payload helpers ----

func extractBlocks(payload map[string]any) []map[string]any {
	raw, _ := payload["blocks"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, rb := range raw {
		if mb, ok := rb.(map[string]any); ok {
			out = append(out, mb)
		}
	}
	return out
}

func blockText(blocks []map[string]any) string {
	var sb strings.Builder
	for _, b := range blocks {
		if txt, ok := b["text"].(map[string]any); ok {
			if s, ok := txt["text"].(string); ok {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
		}
		if elems, ok := b["elements"].([]any); ok {
			for _, e := range elems {
				if m, ok := e.(map[string]any); ok {
					if s, ok := m["text"].(string); ok {
						sb.WriteString(s)
						sb.WriteByte('\n')
					}
				}
			}
		}
	}
	return sb.String()
}

func indexOfType(blocks []map[string]any, typ, marker string) int {
	for i, b := range blocks {
		if b["type"] != typ {
			continue
		}
		if txt, ok := b["text"].(map[string]any); ok {
			if s, ok := txt["text"].(string); ok && strings.Contains(s, marker) {
				return i
			}
		}
	}
	return -1
}

func parseProtect(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u.Query()
}

func pExp(q url.Values) int64 {
	v, _ := strconv.ParseInt(q.Get(notifier.ParamExp), 10, 64)
	return v
}
