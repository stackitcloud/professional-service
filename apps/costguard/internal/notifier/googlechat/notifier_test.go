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

package googlechat

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// sampleReport builds a report exercising every section.
func sampleReport() *report.Report {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	mark := now.Add(8 * time.Hour)
	return &report.Report{
		GeneratedAt: now,
		DryRun:      true,
		Scope:       "organisation",
		OrgID:       "org-1",
		Regions:     []string{"eu01"},

		StaleProjects: []report.StaleProject{
			{ID: "p1", Name: "old-a", AgeDays: 120, CreatedAt: now.AddDate(0, 0, -120), ParentFolderID: "f1", ParentFolderName: "Team A", SKEClusters: []string{"prod"}, ServerCount: 2},
			{ID: "p2", Name: "old-b", AgeDays: 95, CreatedAt: now.AddDate(0, 0, -95), ParentFolderID: "f1", ParentFolderName: "Team A"},
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
		TopProjects: []report.ProjectCost{
			{ProjectID: "pp1", ProjectName: "proj-a", Cost30dEUR: 440.0},
		},
		AnomalyDetected:  true,
		AnomalyPct:       133.33,
		TotalCost30dEUR:  590.0,
		AvgCostPerDayEUR: 19.67,
		ChartURL:         "https://s3.example/chart.png",
		ScanErrors:       []string{"cost day 2026-08-23: boom"},

		EstimatedMonthlySavingsEUR: 15.83,
		EstimatedDailySavingsEUR:   0.53,
	}
}

func capture(t *testing.T, handler func(body []byte) int) (string, *[]byte) {
	t.Helper()
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = b
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(handler(b))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &got
}

func TestSendReportPayloadShape(t *testing.T) {
	url, body := capture(t, func(b []byte) int { return 200 })
	n := New(url, nil, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	var msg message
	if err := json.Unmarshal(*body, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, *body)
	}
	if msg.CardsV2 == nil {
		t.Fatal("cardsV2 missing")
	}
	if !strings.Contains(msg.CardsV2.Header.Title, "costguard") {
		t.Errorf("header title = %q", msg.CardsV2.Header.Title)
	}
	if !strings.Contains(msg.CardsV2.Header.Subtitle, "DRY RUN") {
		t.Errorf("subtitle missing dry-run flag: %q", msg.CardsV2.Header.Subtitle)
	}
	all := strings.Join(sectionTitles(msg.CardsV2.Sections), " | ")
	for _, want := range []string{
		"Stale projects (2)", "Empty network areas (1)", "Idle public IPs (1)",
		"Detached volumes (1)", "Cost anomaly detected", "Top 10 projects",
		"Costs, last 30 days",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing section %q in: %s", want, all)
		}
	}
	// Savings line present.
	var foundSavings bool
	for _, s := range msg.CardsV2.Sections {
		if strings.Contains(widgetText(s), "15.83") {
			foundSavings = true
		}
	}
	if !foundSavings {
		t.Error("estimated savings line missing")
	}
	// No buttons when the callback is unconfigured.
	for _, s := range msg.CardsV2.Sections {
		for _, w := range s.Widgets {
			if w.ButtonList != nil {
				t.Error("button rendered without callback configured")
			}
		}
	}
}

func TestSendReportButtonsSigned(t *testing.T) {
	url, body := capture(t, func(b []byte) int { return 200 })
	btn := notifier.NewButton("https://cb.example.com", "s3cr3t")
	n := New(url, btn, testLogger())

	if err := n.SendReport(context.Background(), sampleReport()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	var msg message
	if err := json.Unmarshal(*body, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	var buttonURLs []string
	for _, s := range msg.CardsV2.Sections {
		for _, w := range s.Widgets {
			if w.ButtonList != nil {
				for _, b := range w.ButtonList.Buttons {
					buttonURLs = append(buttonURLs, b.OnClick.OpenLink.URL)
				}
			}
		}
	}
	// 2 stale projects + 1 IP + 1 volume = 4 buttons.
	if len(buttonURLs) != 4 {
		t.Fatalf("buttons = %d, want 4: %v", len(buttonURLs), buttonURLs)
	}
	for _, raw := range buttonURLs {
		u, err := parseProtectURL(raw)
		if err != nil {
			t.Fatalf("button URL: %v", err)
		}
		if !notifier.VerifySig(btn.Secret, u.id, u.typ, u.project, u.region, u.exp, u.sig) {
			t.Errorf("button signature invalid: %s", raw)
		}
		if u.typ != "volume" && u.typ != "publicip" && u.typ != "project" {
			t.Errorf("unexpected button type %q", u.typ)
		}
	}
}

func TestSendReportChunking(t *testing.T) {
	url, body := capture(t, func(b []byte) int { return 200 })
	n := New(url, nil, testLogger())

	rep := sampleReport()
	rep.StaleProjects = nil
	rep.IdlePublicIPs = nil
	rep.DetachedVolumes = nil
	for i := 0; i < 35; i++ {
		rep.EmptySNAs = append(rep.EmptySNAs, report.EmptySNA{ID: "sna", Name: "sna"})
	}
	if err := n.SendReport(context.Background(), rep); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	var msg message
	if err := json.Unmarshal(*body, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	var snaSections int
	for _, s := range msg.CardsV2.Sections {
		if s.Header != nil && strings.HasPrefix(s.Header.Title, "Empty network areas") {
			snaSections++
		}
	}
	if snaSections != 2 {
		t.Errorf("SNA sections = %d, want 2 (35 rows chunked by 30)", snaSections)
	}
}

func TestSendReportEmptyOmitsSections(t *testing.T) {
	url, body := capture(t, func(b []byte) int { return 200 })
	n := New(url, nil, testLogger())
	rep := &report.Report{GeneratedAt: time.Now(), Scope: "organisation"}

	if err := n.SendReport(context.Background(), rep); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	var msg message
	if err := json.Unmarshal(*body, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	for _, s := range msg.CardsV2.Sections {
		if s.Header != nil && strings.Contains(s.Header.Title, "Idle public IPs") {
			t.Error("empty IP section must be omitted")
		}
	}
}

func TestSendReportWebhookError(t *testing.T) {
	url, _ := capture(t, func(b []byte) int { return 500 })
	n := New(url, nil, testLogger())
	if err := n.SendReport(context.Background(), sampleReport()); err == nil {
		t.Error("SendReport = nil error, want webhook 500 error")
	}
}

func TestSendDeletionSummary(t *testing.T) {
	url, body := capture(t, func(b []byte) int { return 200 })
	n := New(url, nil, testLogger())
	summary := &report.DeletionSummary{
		GeneratedAt: time.Now(),
		Results: []report.DeletionResult{
			{Item: report.DeletionItem{Kind: "volume", Name: "v1", ProjectName: "p", Region: "eu01"}, Status: report.StatusDeleted},
			{Item: report.DeletionItem{Kind: "publicip", Name: "i1", ProjectName: "p", Region: "eu01"}, Status: report.StatusFailed, Reason: "boom"},
			{Item: report.DeletionItem{Kind: "volume", Name: "v2", ProjectName: "p", Region: "eu01"}, Status: report.StatusSkipped, Reason: "protected"},
		},
	}
	if err := n.SendDeletionSummary(context.Background(), summary); err != nil {
		t.Fatalf("SendDeletionSummary: %v", err)
	}
	var msg message
	if err := json.Unmarshal(*body, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	text := widgetText(msg.CardsV2.Sections[0])
	for _, want := range []string{"Deleted 1", "failed 1", "skipped 1", "boom", "protected"} {
		if !strings.Contains(text, want) {
			t.Errorf("deletion summary missing %q in: %s", want, text)
		}
	}
}

// ---- payload inspection helpers ----

func sectionTitles(sections []section) []string {
	out := make([]string, 0, len(sections))
	for _, s := range sections {
		if s.Header != nil {
			out = append(out, s.Header.Title)
		}
	}
	return out
}

func widgetText(s section) string {
	var sb strings.Builder
	for _, w := range s.Widgets {
		if w.TextParagraph != nil {
			for _, e := range w.TextParagraph.Elements {
				if e.TextContent != nil {
					sb.WriteString(e.TextContent.Text)
					sb.WriteByte(' ')
				}
			}
		}
		if w.KeyValueList != nil {
			for _, c := range w.KeyValueList.Columns {
				if c.TopLabel != nil {
					sb.WriteString(c.TopLabel.Text)
				}
				if c.Content != nil {
					sb.WriteString(c.Content.Text)
				}
				if c.BottomLabel != nil {
					sb.WriteString(c.BottomLabel.Text)
				}
				sb.WriteByte(' ')
			}
		}
	}
	return sb.String()
}

type protectParts struct {
	id, typ, project, region, sig string
	exp                           int64
}

func parseProtectURL(raw string) (*protectParts, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	exp, err := strconv.ParseInt(q.Get(notifier.ParamExp), 10, 64)
	if err != nil {
		return nil, err
	}
	return &protectParts{
		id:      q.Get(notifier.ParamID),
		typ:     q.Get(notifier.ParamType),
		project: q.Get(notifier.ParamProject),
		region:  q.Get(notifier.ParamRegion),
		exp:     exp,
		sig:     q.Get(notifier.ParamSig),
	}, nil
}
