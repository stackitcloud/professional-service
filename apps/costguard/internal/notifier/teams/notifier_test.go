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

package teams

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
			{ID: "p1", Name: "old-a", AgeDays: 120, CreatedAt: now.AddDate(0, 0, -120), ParentFolderID: "f1", ParentFolderName: "Team A"},
		},
		EmptySNAs:     []report.EmptySNA{{ID: "sna-1", Name: "empty-1", AgeDays: 40, CreatedAt: now.AddDate(0, 0, -40)}},
		IdlePublicIPs: []report.IdlePublicIP{{ID: "ip-1", ProjectID: "pp1", ProjectName: "proj-a", Region: "eu01", Address: "1.2.3.4", MarkDeadline: &mark}},
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

type envelope struct {
	Type        string       `json:"@type"`
	Context     string       `json:"@context"`
	Summary     string       `json:"summary"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	ContentType string         `json:"contentType"`
	Content     map[string]any `json:"content"`
}

func postAndCapture(t *testing.T, handler func(body []byte)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		handler(b)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestSendReportAdaptiveCard(t *testing.T) {
	var env envelope
	url := postAndCapture(t, func(b []byte) {
		if err := json.Unmarshal(b, &env); err != nil {
			t.Errorf("envelope not JSON: %v", err)
		}
	})
	n := New(url, nil, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if env.Type != "MessageCard" {
		t.Errorf("@type = %q, want MessageCard", env.Type)
	}
	if len(env.Attachments) != 1 || env.Attachments[0].ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("attachments = %+v", env.Attachments)
	}
	card := env.Attachments[0].Content
	if card["type"] != "AdaptiveCard" {
		t.Errorf("card type = %v", card["type"])
	}
	bodyText := cardText(card)
	for _, want := range []string{
		"Stale projects (1)", "Empty network areas (1)", "Idle public IPs (1)",
		"Detached volumes (1)", "Cost anomaly detected", "Top 10 projects",
		"Costs, last 30 days", "15.83",
	} {
		if !strings.Contains(bodyText, want) {
			t.Errorf("missing %q in card body: %s", want, bodyText)
		}
	}
	if _, ok := card["actions"]; ok {
		t.Error("actions present without callback configured")
	}
}

func TestSendReportActionHTTP(t *testing.T) {
	var env envelope
	url := postAndCapture(t, func(b []byte) {
		_ = json.Unmarshal(b, &env)
	})
	btn := notifier.NewButton("https://cb.example.com", "s3cr3t")
	n := New(url, btn, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	card := env.Attachments[0].Content
	actions, _ := card["actions"].([]any)
	// 1 stale project + 1 IP + 1 volume = 3 Action.Http buttons.
	if len(actions) != 3 {
		t.Fatalf("actions = %d, want 3", len(actions))
	}
	for _, a := range actions {
		action := a.(map[string]any)
		if action["type"] != "Action.Http" {
			t.Errorf("action type = %v, want Action.Http", action["type"])
		}
		if action["method"] != "POST" {
			t.Errorf("method = %v, want POST", action["method"])
		}
		if action["url"] != "https://cb.example.com/protect" {
			t.Errorf("url = %v", action["url"])
		}
		body, _ := action["body"].(string)
		var m map[string]any
		if err := json.Unmarshal([]byte(body), &m); err != nil {
			t.Fatalf("action body not JSON: %v (%s)", err, body)
		}
		exp, _ := m[notifier.ParamExp].(float64)
		if !notifier.VerifySig(btn.Secret, m[notifier.ParamID].(string), m[notifier.ParamType].(string),
			m[notifier.ParamProject].(string), m[notifier.ParamRegion].(string), int64(exp), m[notifier.ParamSig].(string)) {
			t.Errorf("action signature invalid: %s", body)
		}
	}
}

func TestSendDeletionSummary(t *testing.T) {
	var env envelope
	url := postAndCapture(t, func(b []byte) {
		_ = json.Unmarshal(b, &env)
	})
	n := New(url, nil, testLogger())
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
	text := cardText(env.Attachments[0].Content)
	for _, want := range []string{"Deleted 1", "failed 1", "boom"} {
		if !strings.Contains(text, want) {
			t.Errorf("deletion summary missing %q in: %s", want, text)
		}
	}
}

func TestSendReportWebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
	}))
	t.Cleanup(srv.Close)
	n := New(srv.URL, nil, testLogger())
	if err := n.SendReport(context.Background(), sampleReport()); err == nil {
		t.Error("SendReport = nil, want error on 502")
	}
}

// ---- helpers ----

func cardText(card map[string]any) string {
	var sb strings.Builder
	var walk func(items []any)
	walk = func(items []any) {
		for _, it := range items {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "TextBlock":
				if s, ok := m["text"].(string); ok {
					sb.WriteString(s)
					sb.WriteByte('\n')
				}
			case "FactSet":
				facts, _ := m["facts"].([]any)
				for _, f := range facts {
					fm, _ := f.(map[string]any)
					if s, ok := fm["title"].(string); ok {
						sb.WriteString(s)
					}
					if s, ok := fm["value"].(string); ok {
						sb.WriteString(" ")
						sb.WriteString(s)
					}
					sb.WriteByte('\n')
				}
			case "Container":
				if sub, ok := m["items"].([]any); ok {
					walk(sub)
				}
			}
		}
	}
	if body, ok := card["body"].([]any); ok {
		walk(body)
	}
	return sb.String()
}
