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

// Package slack renders the costguard report as a Slack Block Kit message
// and POSTs it to the channel webhook. The "Do not delete"
// button is a signed mrkdwn link: Slack incoming webhooks cannot
// receive button clicks without a full Slack App.
package slack

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

// Notifier renders and delivers the report to a Slack incoming webhook.
type Notifier struct {
	webhookURL string
	buttons    *notifier.Button
	client     *http.Client
	logger     *slog.Logger
}

// New builds a Slack notifier. buttons may be nil (static text only).
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

// SendReport renders the Block Kit message and posts it.
func (n *Notifier) SendReport(ctx context.Context, rep *report.Report) error {
	return n.post(ctx, renderReport(rep, n.buttons))
}

// SendDeletionSummary renders and posts the post-deletion confirmation.
func (n *Notifier) SendDeletionSummary(ctx context.Context, summary *report.DeletionSummary) error {
	return n.post(ctx, renderDeletionSummary(summary))
}

func (n *Notifier) post(ctx context.Context, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding Slack payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to Slack webhook: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Slack webhook returned %s: %s", resp.Status, string(body))
	}
	n.logger.Debug("slack: report delivered", "status", resp.Status)
	return nil
}

// ---- Block Kit builders ----

type mrkdwn struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func header(text string) map[string]any {
	return map[string]any{"type": "header", "text": mrkdwn{"mrkdwn", text}}
}

func section(text string) map[string]any {
	return map[string]any{"type": "section", "text": mrkdwn{"mrkdwn", text}}
}

func contextBlock(text string) map[string]any {
	return map[string]any{"type": "context", "elements": []mrkdwn{{"mrkdwn", text}}}
}

func divider() map[string]any {
	return map[string]any{"type": "divider"}
}

// protectLink renders the signed "Do not delete" mrkdwn link, or an empty
// string when buttons are disabled. A zero request deadline falls back
// to one reporting cycle (report-only items).
func (n *Notifier) protectLink(rep *report.Report, req notifier.ProtectRequest) string {
	if n.buttons == nil {
		return ""
	}
	if req.Deadline.IsZero() {
		req.Deadline = notifier.ButtonDeadline(rep.GeneratedAt, nil)
	}
	return fmt.Sprintf("[Do not delete](%s)", n.buttons.URL(req))
}

// ---- report rendering ----

func renderReport(rep *report.Report, btn *notifier.Button) map[string]any {
	n := &Notifier{buttons: btn}
	var blocks []map[string]any
	blocks = append(blocks,
		header("costguard — cost report"),
		contextBlock(fmt.Sprintf("scope=%s · %s · %s", rep.Scope, rep.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"), dryRunLabel(rep.DryRun))),
		divider(),
		section(fmt.Sprintf("*Estimated savings if all candidates are deleted: %s/month (%s/day).*",
			notifier.FormatEUR(rep.EstimatedMonthlySavingsEUR), notifier.FormatEUR(rep.EstimatedDailySavingsEUR))),
	)
	if len(rep.ScanErrors) > 0 {
		blocks = append(blocks, section(fmt.Sprintf("_Warning: %d scanner error(s) this run — the report may be partial._", len(rep.ScanErrors))))
	}
	blocks = append(blocks, divider())

	blocks = append(blocks, staleProjects(rep, n)...)
	blocks = append(blocks, emptySNAs(rep)...)
	blocks = append(blocks, idleIPs(rep, n)...)
	blocks = append(blocks, detachedVolumes(rep, n)...)
	blocks = append(blocks, anomaly(rep)...)
	blocks = append(blocks, topProjects(rep)...)
	blocks = append(blocks, chart(rep)...)

	blocks = append(blocks, divider())
	blocks = append(blocks, contextBlock(fmt.Sprintf("costguard v1 · %s", dryRunLabel(rep.DryRun))))

	return map[string]any{"blocks": blocks}
}

func dryRunLabel(dry bool) string {
	if dry {
		return "DRY RUN"
	}
	return "live"
}

// Each group emits: a header block, then the summary line as the FIRST
// visible block, then the detail rows.

func staleProjects(rep *report.Report, n *Notifier) []map[string]any {
	if len(rep.StaleProjects) == 0 {
		return nil
	}
	var blocks []map[string]any
	blocks = append(blocks, header(fmt.Sprintf("Stale projects (%d)", len(rep.StaleProjects))))
	blocks = append(blocks, section("*Report only — not auto-deleted in v1.*"))
	groups := notifier.GroupProjectsByFolder(rep.StaleProjects)
	for _, g := range groups {
		name := g.FolderName
		if name == "" {
			name = "Organization (root)"
		}
		var rows []string
		for _, p := range g.Projects {
			link := n.protectLink(rep, notifier.ProtectRequest{ID: p.ID, Type: whitelist.TypeProject})
			rows = append(rows, fmt.Sprintf("• _%s_ — %d days, %s%s",
				p.Name, p.AgeDays, probeSummary(p), suffix(link)))
		}
		blocks = append(blocks, section(fmt.Sprintf("*%s*%s", name, strings.Join(rows, "\n"))))
	}
	return blocks
}

func emptySNAs(rep *report.Report) []map[string]any {
	if len(rep.EmptySNAs) == 0 {
		return nil
	}
	var blocks []map[string]any
	blocks = append(blocks, header(fmt.Sprintf("Empty network areas (%d)", len(rep.EmptySNAs))))
	blocks = append(blocks, section("*Report only — not auto-deleted in v1.*"))
	var rows []string
	for _, s := range rep.EmptySNAs {
		rows = append(rows, fmt.Sprintf("• _%s_ — %d days, created %s", s.Name, s.AgeDays, s.CreatedAt.UTC().Format("2006-01-02")))
	}
	blocks = append(blocks, section(strings.Join(rows, "\n")))
	return blocks
}

func idleIPs(rep *report.Report, n *Notifier) []map[string]any {
	if len(rep.IdlePublicIPs) == 0 {
		return nil
	}
	var blocks []map[string]any
	blocks = append(blocks, header(fmt.Sprintf("Idle public IPs (%d)", len(rep.IdlePublicIPs))))
	blocks = append(blocks, section("*Deletion candidates — idle with no NIC attachment.*"))
	var rows []string
	for _, ip := range rep.IdlePublicIPs {
		link := n.protectLink(rep, notifier.ProtectRequest{
			ID: ip.ID, Type: whitelist.TypePublicIP, Project: ip.ProjectID, Region: ip.Region,
			Deadline: derefTime(ip.MarkDeadline),
		})
		rows = append(rows, fmt.Sprintf("• %s — %s (%s), deletes after %s%s",
			ip.Address, ip.ProjectName, ip.Region, notifier.FormatDeadline(derefTime(ip.MarkDeadline)), suffix(link)))
	}
	blocks = append(blocks, section(strings.Join(rows, "\n")))
	return blocks
}

func detachedVolumes(rep *report.Report, n *Notifier) []map[string]any {
	if len(rep.DetachedVolumes) == 0 {
		return nil
	}
	var blocks []map[string]any
	blocks = append(blocks, header(fmt.Sprintf("Detached volumes (%d)", len(rep.DetachedVolumes))))
	blocks = append(blocks, section("*Deletion candidates — detached (AVAILABLE) with no server.*"))
	var rows []string
	for _, v := range rep.DetachedVolumes {
		link := n.protectLink(rep, notifier.ProtectRequest{
			ID: v.ID, Type: whitelist.TypeVolume, Project: v.ProjectID, Region: v.Region,
			Deadline: derefTime(v.MarkDeadline),
		})
		rows = append(rows, fmt.Sprintf("• _%s_ — %d GB (~%s/mo), %s (%s), deletes after %s%s",
			v.Name, v.SizeGB, notifier.FormatEUR(v.EstimatedMonthlyCostEUR), v.ProjectName, v.Region,
			notifier.FormatDeadline(derefTime(v.MarkDeadline)), suffix(link)))
	}
	blocks = append(blocks, section(strings.Join(rows, "\n")))
	return blocks
}

func anomaly(rep *report.Report) []map[string]any {
	if !rep.AnomalyDetected {
		return nil
	}
	return []map[string]any{
		header("Cost anomaly detected"),
		section(fmt.Sprintf("*Spend is up %.1f%% over the last 7 days versus the prior 7.*", rep.AnomalyPct)),
	}
}

func topProjects(rep *report.Report) []map[string]any {
	if len(rep.TopProjects) == 0 {
		return nil
	}
	var blocks []map[string]any
	blocks = append(blocks, header("Top 10 projects by 30-day spend"))
	var rows []string
	for i, p := range rep.TopProjects {
		name := p.ProjectName
		if name == "" {
			name = p.ProjectID
		}
		rows = append(rows, fmt.Sprintf("%d. _%s_ — %s / 30d", i+1, name, notifier.FormatEUR(p.Cost30dEUR)))
	}
	blocks = append(blocks, section(strings.Join(rows, "\n")))
	return blocks
}

func chart(rep *report.Report) []map[string]any {
	if rep.ChartURL == "" {
		return nil
	}
	return []map[string]any{
		header("Costs, last 30 days"),
		section(fmt.Sprintf("Total %s · average %s/day — [view chart](%s)",
			notifier.FormatEUR(rep.TotalCost30dEUR), notifier.FormatEUR(rep.AvgCostPerDayEUR), rep.ChartURL)),
	}
}

// ---- deletion summary ----

func renderDeletionSummary(summary *report.DeletionSummary) map[string]any {
	deleted, failed, skipped := summary.Counts()
	var blocks []map[string]any
	blocks = append(blocks,
		header("costguard — deletion run"),
		contextBlock(summary.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC")),
		section(fmt.Sprintf("*Deleted %d · failed %d · skipped %d.*", deleted, failed, skipped)),
	)
	if failed > 0 {
		var rows []string
		for _, r := range summary.ResultsByStatus(report.StatusFailed) {
			rows = append(rows, fmt.Sprintf("• %s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason))
		}
		blocks = append(blocks, section("*Failures:*"+strings.Join(rows, "\n")))
	}
	if skipped > 0 {
		var rows []string
		for _, r := range summary.ResultsByStatus(report.StatusSkipped) {
			rows = append(rows, fmt.Sprintf("• %s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason))
		}
		blocks = append(blocks, section("*Skipped (protected):*"+strings.Join(rows, "\n")))
	}
	if deleted > 0 {
		var rows []string
		for _, r := range summary.ResultsByStatus(report.StatusDeleted) {
			rows = append(rows, fmt.Sprintf("• %s %s (%s/%s)", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region))
		}
		blocks = append(blocks, section("*Deleted:*"+strings.Join(rows, "\n")))
	}
	return map[string]any{"blocks": blocks}
}

// ---- helpers ----

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func probeSummary(p report.StaleProject) string {
	parts := make([]string, 0, 3)
	if len(p.SKEClusters) > 0 {
		parts = append(parts, fmt.Sprintf("%d SKE", len(p.SKEClusters)))
	}
	if len(p.StorageBuckets) > 0 {
		parts = append(parts, fmt.Sprintf("%d bucket(s)", len(p.StorageBuckets)))
	}
	if p.ServerCount > 0 {
		parts = append(parts, fmt.Sprintf("%d server(s)", p.ServerCount))
	}
	if len(parts) == 0 {
		return "no content found"
	}
	return strings.Join(parts, ", ")
}

func suffix(link string) string {
	if link == "" {
		return ""
	}
	return " · " + link
}
