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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// maxRowsPerSection bounds rows per collapsible section before the
// section is split, keeping cards within webhook size limits.
const maxRowsPerSection = 30

// Notifier renders and delivers the report to a Google Chat space webhook.
type Notifier struct {
	webhookURL string
	buttons    *notifier.Button
	client     *http.Client
	logger     *slog.Logger
}

// New builds a Google Chat notifier. buttons may be nil (static text
// only).
func New(webhookURL string, buttons *notifier.Button, logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{
		webhookURL: webhookURL,
		buttons:    buttons,
		client:     &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// SendReport renders the report card and posts it.
func (n *Notifier) SendReport(ctx context.Context, rep *report.Report) error {
	return n.post(ctx, renderReport(rep, n.buttons))
}

// SendDeletionSummary renders and posts the post-deletion confirmation.
func (n *Notifier) SendDeletionSummary(ctx context.Context, summary *report.DeletionSummary) error {
	return n.post(ctx, renderDeletionSummary(summary))
}

func (n *Notifier) post(ctx context.Context, msg message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encoding Google Chat payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to Google Chat webhook: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Google Chat webhook returned %s: %s", resp.Status, string(body))
	}
	n.logger.Debug("googlechat: report delivered", "status", resp.Status)
	return nil
}

// ---- report rendering ----

func reportTitle(rep *report.Report) (title, subtitle string) {
	title = "costguard — cost report"
	subtitle = fmt.Sprintf("scope=%s · %s", rep.Scope, rep.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"))
	if rep.DryRun {
		subtitle += " · DRY RUN"
	}
	return title, subtitle
}

func renderReport(rep *report.Report, btn *notifier.Button) message {
	title, subtitle := reportTitle(rep)
	var sections []section
	sections = append(sections, summarySection(rep))
	sections = append(sections, staleProjectsSections(rep, btn)...)
	sections = append(sections, emptySNASection(rep)...)
	sections = append(sections, idleIPSections(rep, btn)...)
	sections = append(sections, detachedVolumeSections(rep, btn)...)
	sections = append(sections, anomalySection(rep))
	sections = append(sections, topProjectsSection(rep))
	sections = append(sections, chartSection(rep))
	sections = append(sections, footerSection(rep))

	return message{CardsV2: &cardsV2{
		Header:   &header{Title: title, Subtitle: subtitle},
		Sections: sections,
	}}
}

// summarySection is the always-visible savings line plus any
// scan errors (a partial scan must be visible).
func summarySection(rep *report.Report) section {
	widgets := []widget{
		wText(fmt.Sprintf("Estimated savings if all candidates are deleted: %s/month (%s/day).",
			notifier.FormatEUR(rep.EstimatedMonthlySavingsEUR),
			notifier.FormatEUR(rep.EstimatedDailySavingsEUR)), "BOLD", "", ""),
	}
	if len(rep.ScanErrors) > 0 {
		widgets = append(widgets, wText(fmt.Sprintf("Warning: %d scanner error(s) this run — the report may be partial: %s",
			len(rep.ScanErrors), joinLines(rep.ScanErrors)), "", "", warningColor))
	}
	return section{Widgets: widgets}
}

// staleProjectsSections groups stale projects by folder and
// chunks them into collapsible sections.
func staleProjectsSections(rep *report.Report, btn *notifier.Button) []section {
	if len(rep.StaleProjects) == 0 {
		return nil
	}
	title := fmt.Sprintf("Stale projects (%d) — report only, not auto-deleted", len(rep.StaleProjects))
	var rows []rowWidget
	for _, g := range notifier.GroupProjectsByFolder(rep.StaleProjects) {
		rows = append(rows, rowWidget{kv: wText(folderHeading(g.FolderName), "BOLD", "HEADER_SMALL", "")})
		for i, p := range g.Projects {
			rows = append(rows, rowWidget{
				kv:  kvWidget([]kvColumn{staleProjectRow(p)}),
				btn: protectButtons(rep, btn, projectButtons(g.Projects[i:i+1])...),
			})
		}
	}
	return sectionsFromRows(title, rows)
}

func staleProjectRow(p report.StaleProject) kvColumn {
	return kvRow(
		p.Name,
		fmt.Sprintf("%d days old · created %s", p.AgeDays, p.CreatedAt.UTC().Format("2006-01-02")),
		probeSummary(p),
	)
}

func folderHeading(name string) string {
	if name == "" {
		return "Organization (root)"
	}
	return "Folder: " + name
}

func probeSummary(p report.StaleProject) string {
	parts := make([]string, 0, 3)
	if len(p.SKEClusters) > 0 {
		parts = append(parts, fmt.Sprintf("%d SKE cluster(s)", len(p.SKEClusters)))
	}
	if len(p.StorageBuckets) > 0 {
		parts = append(parts, fmt.Sprintf("%d bucket(s)", len(p.StorageBuckets)))
	}
	if p.ServerCount > 0 {
		parts = append(parts, fmt.Sprintf("%d server(s)", p.ServerCount))
	}
	if len(parts) == 0 {
		return "no SKE/buckets/servers found"
	}
	return joinWith(parts, ", ")
}

// rowWidget is one report row: its keyValueList plus any protect buttons
// that travel with it (so chunking keeps a button with its row).
type rowWidget struct {
	kv  widget
	btn []widget
}

func kvWidget(cols []kvColumn) widget {
	return widget{KeyValueList: &keyValueList{
		ColumnProperties: []colProps{{Weight: 1}},
		Columns:          cols,
	}}
}

// sectionsFromRows turns rows into collapsible sections of at most
// maxRowsPerSection rows, splitting the title with (i/n) when needed.
func sectionsFromRows(title string, rows []rowWidget) []section {
	if len(rows) == 0 {
		return nil
	}
	var out []section
	chunks := (len(rows) + maxRowsPerSection - 1) / maxRowsPerSection
	for start := 0; start < len(rows); start += maxRowsPerSection {
		end := start + maxRowsPerSection
		if end > len(rows) {
			end = len(rows)
		}
		t := title
		if chunks > 1 {
			t = fmt.Sprintf("%s (%d/%d)", title, start/maxRowsPerSection+1, chunks)
		}
		var widgets []widget
		for _, r := range rows[start:end] {
			widgets = append(widgets, r.kv)
			widgets = append(widgets, r.btn...)
		}
		out = append(out, collapsible(t, widgets))
	}
	return out
}

func emptySNASection(rep *report.Report) []section {
	if len(rep.EmptySNAs) == 0 {
		return nil
	}
	title := fmt.Sprintf("Empty network areas (%d) — report only, not auto-deleted", len(rep.EmptySNAs))
	rows := make([]rowWidget, 0, len(rep.EmptySNAs))
	for _, s := range rep.EmptySNAs {
		rows = append(rows, rowWidget{kv: kvWidget([]kvColumn{kvRow(
			s.Name,
			fmt.Sprintf("%d days old · created %s", s.AgeDays, s.CreatedAt.UTC().Format("2006-01-02")),
			s.ID,
		)})})
	}
	return sectionsFromRows(title, rows)
}

func idleIPSections(rep *report.Report, btn *notifier.Button) []section {
	if len(rep.IdlePublicIPs) == 0 {
		return nil
	}
	title := fmt.Sprintf("Idle public IPs (%d)", len(rep.IdlePublicIPs))
	rows := make([]rowWidget, 0, len(rep.IdlePublicIPs))
	for i, ip := range rep.IdlePublicIPs {
		rows = append(rows, rowWidget{
			kv: kvWidget([]kvColumn{kvRow(
				ip.Address,
				fmt.Sprintf("%s (%s) · %s", ip.ProjectName, ip.Region, ip.ID),
				"deletes after "+notifier.FormatDeadline(derefTime(ip.MarkDeadline)),
			)}),
			btn: protectButtons(rep, btn, ipButtons(rep.IdlePublicIPs[i:i+1])...),
		})
	}
	return sectionsFromRows(title, rows)
}

func detachedVolumeSections(rep *report.Report, btn *notifier.Button) []section {
	if len(rep.DetachedVolumes) == 0 {
		return nil
	}
	title := fmt.Sprintf("Detached volumes (%d)", len(rep.DetachedVolumes))
	rows := make([]rowWidget, 0, len(rep.DetachedVolumes))
	for i, v := range rep.DetachedVolumes {
		rows = append(rows, rowWidget{
			kv: kvWidget([]kvColumn{kvRow(
				v.Name,
				fmt.Sprintf("%d GB · est. %s/month", v.SizeGB, notifier.FormatEUR(v.EstimatedMonthlyCostEUR)),
				fmt.Sprintf("%s (%s) · deletes after %s", v.ProjectName, v.Region, notifier.FormatDeadline(derefTime(v.MarkDeadline))),
			)}),
			btn: protectButtons(rep, btn, volumeButtons(rep.DetachedVolumes[i:i+1])...),
		})
	}
	return sectionsFromRows(title, rows)
}

func anomalySection(rep *report.Report) section {
	if !rep.AnomalyDetected {
		return section{}
	}
	return section{
		Header:              &header{Title: "Cost anomaly detected"},
		CollapsibleSection:  true,
		HeaderBarProperties: &headerBar{BackgroundColor: warningColor},
		Widgets: []widget{
			wText(fmt.Sprintf("Spend is up %.1f%% over the last 7 days versus the 7 days before. Review the top spenders below.",
				rep.AnomalyPct), "", "", warningColor),
		},
	}
}

func topProjectsSection(rep *report.Report) section {
	if len(rep.TopProjects) == 0 {
		return section{}
	}
	cols := make([]kvColumn, 0, len(rep.TopProjects))
	for i, p := range rep.TopProjects {
		name := p.ProjectName
		if name == "" {
			name = p.ProjectID
		}
		cols = append(cols, kvColumn{
			TopLabel: kvText(fmt.Sprintf("#%d %s", i+1, name)),
			Content:  kvText(notifier.FormatEUR(p.Cost30dEUR) + " / 30d"),
		})
	}
	return section{
		Header:              &header{Title: "Top 10 projects by 30-day spend"},
		CollapsibleSection:  true,
		HeaderBarProperties: &headerBar{BackgroundColor: cardColor},
		Widgets:             []widget{{KeyValueList: &keyValueList{ColumnProperties: []colProps{{Weight: 1}}, Columns: cols}}},
	}
}

func chartSection(rep *report.Report) section {
	if rep.ChartURL == "" {
		return section{}
	}
	return section{
		Header:             &header{Title: "Costs, last 30 days"},
		CollapsibleSection: true,
		Widgets: []widget{
			wText(fmt.Sprintf("Total %s · average %s/day.",
				notifier.FormatEUR(rep.TotalCost30dEUR), notifier.FormatEUR(rep.AvgCostPerDayEUR)), "", "", ""),
			widget{MediaInfo: &mediaInfo{ContentURL: rep.ChartURL}},
		},
	}
}

func footerSection(rep *report.Report) section {
	mode := "live"
	if rep.DryRun {
		mode = "dry run"
	}
	return section{Widgets: []widget{
		wText(fmt.Sprintf("costguard v1 · %s · generated %s", mode, rep.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC")),
			"", "SMALL", ""),
	}}
}

// ---- deletion summary rendering ----

func renderDeletionSummary(summary *report.DeletionSummary) message {
	deleted, failed, skipped := summary.Counts()
	mode := "deletion run"
	title := "costguard — " + mode
	var widgets []widget
	widgets = append(widgets, wText(fmt.Sprintf("Deleted %d · failed %d · skipped %d.", deleted, failed, skipped), "BOLD", "", ""))
	if failed > 0 {
		widgets = append(widgets, wText("Failures:", "BOLD", "HEADER_SMALL", warningColor))
		for _, r := range summary.ResultsByStatus(report.StatusFailed) {
			widgets = append(widgets, wText(fmt.Sprintf("- %s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason), "", "", warningColor))
		}
	}
	if skipped > 0 {
		widgets = append(widgets, wText("Skipped (protected):", "BOLD", "HEADER_SMALL", ""))
		for _, r := range summary.ResultsByStatus(report.StatusSkipped) {
			widgets = append(widgets, wText(fmt.Sprintf("- %s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason), "", "", ""))
		}
	}
	if deleted > 0 {
		widgets = append(widgets, wText("Deleted:", "BOLD", "HEADER_SMALL", ""))
		for _, r := range summary.ResultsByStatus(report.StatusDeleted) {
			widgets = append(widgets, wText(fmt.Sprintf("- %s %s (%s/%s)", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region), "", "", ""))
		}
	}
	return message{CardsV2: &cardsV2{
		Header:   &header{Title: title, Subtitle: summary.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC")},
		Sections: []section{{Widgets: widgets}},
	}}
}

// ---- button + chunking helpers ----

type protectTarget struct {
	label string
	req   notifier.ProtectRequest
}

func projectButtons(projects []report.StaleProject) []protectTarget {
	out := make([]protectTarget, 0, len(projects))
	for _, p := range projects {
		// Zero deadline: protectButtons falls back to one reporting
		// cycle (projects are report-only).
		out = append(out, protectTarget{
			label: "Do not delete: " + p.Name,
			req: notifier.ProtectRequest{
				ID:   p.ID,
				Type: whitelist.TypeProject,
			},
		})
	}
	return out
}

func ipButtons(ips []report.IdlePublicIP) []protectTarget {
	out := make([]protectTarget, 0, len(ips))
	for _, ip := range ips {
		out = append(out, protectTarget{
			label: "Do not delete: " + ip.Address,
			req: notifier.ProtectRequest{
				ID:       ip.ID,
				Type:     whitelist.TypePublicIP,
				Project:  ip.ProjectID,
				Region:   ip.Region,
				Deadline: derefTime(ip.MarkDeadline),
			},
		})
	}
	return out
}

func volumeButtons(vols []report.DetachedVolume) []protectTarget {
	out := make([]protectTarget, 0, len(vols))
	for _, v := range vols {
		out = append(out, protectTarget{
			label: "Do not delete: " + v.Name,
			req: notifier.ProtectRequest{
				ID:       v.ID,
				Type:     whitelist.TypeVolume,
				Project:  v.ProjectID,
				Region:   v.Region,
				Deadline: derefTime(v.MarkDeadline),
			},
		})
	}
	return out
}

// protectButtons renders one button widget per target when the callback
// is configured; otherwise it renders nothing (static text only). A zero
// request deadline falls back to one reporting cycle (report-only items).
func protectButtons(rep *report.Report, btn *notifier.Button, targets ...protectTarget) []widget {
	if btn == nil {
		return nil
	}
	var out []widget
	for _, t := range targets {
		req := t.req
		if req.Deadline.IsZero() {
			req.Deadline = notifier.ButtonDeadline(rep.GeneratedAt, nil)
		}
		out = append(out, wButton(t.label, btn.URL(req)))
	}
	return out
}

func collapsible(title string, widgets []widget) section {
	return section{
		Header:              &header{Title: title},
		CollapsibleSection:  true,
		HeaderBarProperties: &headerBar{BackgroundColor: cardColor},
		Widgets:             widgets,
	}
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func joinLines(lines []string) string {
	return strings.Join(lines, "; ")
}

func joinWith(parts []string, sep string) string {
	return strings.Join(parts, sep)
}
