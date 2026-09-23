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

// Package teams renders the costguard report as a Microsoft Teams
// Adaptive Card and POSTs it to the channel webhook. Teams
// incoming webhooks support Action.Http natively, so the "Do not delete"
// button is a real Action.Http that POSTs to /protect — no app
// registration required.
package teams

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

// Notifier renders and delivers the report to a Teams incoming webhook.
type Notifier struct {
	webhookURL string
	buttons    *notifier.Button
	client     *http.Client
	logger     *slog.Logger
}

// New builds a Teams notifier. buttons may be nil (static text only).
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

// SendReport renders the Adaptive Card and posts it.
func (n *Notifier) SendReport(ctx context.Context, rep *report.Report) error {
	return n.post(ctx, renderReport(n, rep))
}

// SendDeletionSummary renders and posts the post-deletion confirmation.
func (n *Notifier) SendDeletionSummary(ctx context.Context, summary *report.DeletionSummary) error {
	return n.post(ctx, renderDeletionSummary(summary))
}

func (n *Notifier) post(ctx context.Context, envelope map[string]any) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encoding Teams payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to Teams webhook: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Teams webhook returned %s: %s", resp.Status, string(body))
	}
	n.logger.Debug("teams: report delivered", "status", resp.Status)
	return nil
}

// ---- Adaptive Card builders ----

func textBlock(text string, opts ...string) map[string]any {
	tb := map[string]any{"type": "TextBlock", "text": text, "wrap": true}
	for i := 0; i+1 < len(opts); i += 2 {
		tb[opts[i]] = opts[i+1]
	}
	return tb
}

func fact(title, value string) map[string]any {
	return map[string]any{"title": title, "value": value}
}

func factSet(facts []map[string]any) map[string]any {
	return map[string]any{"type": "FactSet", "facts": facts}
}

func containerAccent(items []map[string]any) map[string]any {
	return map[string]any{"type": "Container", "style": "accent", "items": items}
}

// actionHTTP builds a native Action.Http button that POSTs the
// signed body to the callback. A zero request deadline falls back to one
// reporting cycle (report-only items).
func (n *Notifier) actionHTTP(rep *report.Report, label string, req notifier.ProtectRequest) map[string]any {
	if n.buttons == nil {
		return nil
	}
	if req.Deadline.IsZero() {
		req.Deadline = notifier.ButtonDeadline(rep.GeneratedAt, nil)
	}
	body, err := n.buttons.Body(req)
	if err != nil {
		return nil
	}
	return map[string]any{
		"type":       "Action.Http",
		"method":     "POST",
		"url":        n.buttons.Endpoint(),
		"name":       label,
		"headers":    map[string]string{"Content-Type": "application/json"},
		"body":       body,
		"dataFormat": "json",
		"result": []map[string]any{
			{"type": "Action.ShowCard", "card": map[string]any{
				"type":    "AdaptiveCard",
				"version": "1.4",
				"body": []map[string]any{
					textBlock("Protected", "weight", "Bolder"),
					textBlock("This resource has been added to the costguard whitelist and will not be deleted.", "isSubtle", "true"),
				},
			}},
		},
	}
}

// ---- report rendering ----

func renderReport(n *Notifier, rep *report.Report) map[string]any {
	var body []map[string]any
	var actions []map[string]any

	body = append(body,
		textBlock("costguard — cost report", "weight", "Bolder", "size", "Large"),
		textBlock(fmt.Sprintf("scope=%s · %s · %s", rep.Scope, rep.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"), dryRunLabel(rep.DryRun)), "isSubtle", "true"),
		containerAccent([]map[string]any{
			textBlock(fmt.Sprintf("Estimated savings: %s/month (%s/day)",
				notifier.FormatEUR(rep.EstimatedMonthlySavingsEUR), notifier.FormatEUR(rep.EstimatedDailySavingsEUR)), "weight", "Bolder"),
		}),
	)
	if len(rep.ScanErrors) > 0 {
		body = append(body, containerAccent([]map[string]any{
			textBlock(fmt.Sprintf("%d scanner error(s) this run — the report may be partial.", len(rep.ScanErrors)), "color", "attention"),
		}))
	}

	addGroup := func(g group) {
		if g.title == "" {
			return
		}
		body = append(body, textBlock(g.title, "weight", "Bolder"))
		if g.sub != "" {
			body = append(body, textBlock(g.sub, "isSubtle", "true"))
		}
		if len(g.items) > 0 {
			body = append(body, factSet(g.items))
		}
	}

	addGroup(staleProjectsGroup(n, rep, &actions))
	addGroup(emptySNAGroup(rep))
	addGroup(idleIPGroup(n, rep, &actions))
	addGroup(detachedVolumeGroup(n, rep, &actions))
	addGroup(anomalyGroup(rep))
	addGroup(topProjectsGroup(rep))
	addGroup(chartGroup(rep))

	card := map[string]any{
		"type":    "AdaptiveCard",
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"version": "1.4",
		"body":    body,
	}
	if len(actions) > 0 {
		card["actions"] = actions
	}
	return map[string]any{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": "0076D4",
		"summary":    "costguard — cost report",
		"attachments": []map[string]any{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content":     card,
		}},
	}
}

// group is one logical report section.
type group struct {
	title string
	sub   string
	items []map[string]any
}

func staleProjectsGroup(n *Notifier, rep *report.Report, actions *[]map[string]any) group {
	if len(rep.StaleProjects) == 0 {
		return group{}
	}
	g := group{title: fmt.Sprintf("Stale projects (%d)", len(rep.StaleProjects)), sub: "Report only — not auto-deleted in v1"}
	for _, grp := range notifier.GroupProjectsByFolder(rep.StaleProjects) {
		for _, p := range grp.Projects {
			g.items = append(g.items, fact(
				fmt.Sprintf("%s — %d days", p.Name, p.AgeDays),
				fmt.Sprintf("%s · %s", folderName(grp), probeSummary(p)),
			))
			if a := n.actionHTTP(rep, "Do not delete: "+p.Name, notifier.ProtectRequest{ID: p.ID, Type: whitelist.TypeProject}); a != nil {
				*actions = append(*actions, a)
			}
		}
	}
	return g
}

func emptySNAGroup(rep *report.Report) group {
	if len(rep.EmptySNAs) == 0 {
		return group{}
	}
	g := group{title: fmt.Sprintf("Empty network areas (%d)", len(rep.EmptySNAs)), sub: "Report only — not auto-deleted in v1"}
	for _, s := range rep.EmptySNAs {
		g.items = append(g.items, fact(fmt.Sprintf("%s — %d days", s.Name, s.AgeDays), "created "+s.CreatedAt.UTC().Format("2006-01-02")))
	}
	return g
}

func idleIPGroup(n *Notifier, rep *report.Report, actions *[]map[string]any) group {
	if len(rep.IdlePublicIPs) == 0 {
		return group{}
	}
	g := group{title: fmt.Sprintf("Idle public IPs (%d)", len(rep.IdlePublicIPs)), sub: "Deletion candidates — idle with no NIC attachment"}
	for _, ip := range rep.IdlePublicIPs {
		g.items = append(g.items, fact(
			fmt.Sprintf("%s — %s (%s)", ip.Address, ip.ProjectName, ip.Region),
			"deletes after "+notifier.FormatDeadline(derefTime(ip.MarkDeadline)),
		))
		if a := n.actionHTTP(rep, "Do not delete: "+ip.Address, notifier.ProtectRequest{
			ID: ip.ID, Type: whitelist.TypePublicIP, Project: ip.ProjectID, Region: ip.Region,
			Deadline: derefTime(ip.MarkDeadline),
		}); a != nil {
			*actions = append(*actions, a)
		}
	}
	return g
}

func detachedVolumeGroup(n *Notifier, rep *report.Report, actions *[]map[string]any) group {
	if len(rep.DetachedVolumes) == 0 {
		return group{}
	}
	g := group{title: fmt.Sprintf("Detached volumes (%d)", len(rep.DetachedVolumes)), sub: "Deletion candidates — detached with no server"}
	for _, v := range rep.DetachedVolumes {
		g.items = append(g.items, fact(
			fmt.Sprintf("%s — %d GB (~%s/mo)", v.Name, v.SizeGB, notifier.FormatEUR(v.EstimatedMonthlyCostEUR)),
			fmt.Sprintf("%s (%s) · deletes after %s", v.ProjectName, v.Region, notifier.FormatDeadline(derefTime(v.MarkDeadline))),
		))
		if a := n.actionHTTP(rep, "Do not delete: "+v.Name, notifier.ProtectRequest{
			ID: v.ID, Type: whitelist.TypeVolume, Project: v.ProjectID, Region: v.Region,
			Deadline: derefTime(v.MarkDeadline),
		}); a != nil {
			*actions = append(*actions, a)
		}
	}
	return g
}

func anomalyGroup(rep *report.Report) group {
	if !rep.AnomalyDetected {
		return group{}
	}
	return group{
		title: "Cost anomaly detected",
		sub:   fmt.Sprintf("Spend is up %.1f%% over the last 7 days versus the prior 7.", rep.AnomalyPct),
	}
}

func topProjectsGroup(rep *report.Report) group {
	if len(rep.TopProjects) == 0 {
		return group{}
	}
	g := group{title: "Top 10 projects by 30-day spend"}
	for i, p := range rep.TopProjects {
		name := p.ProjectName
		if name == "" {
			name = p.ProjectID
		}
		g.items = append(g.items, fact(fmt.Sprintf("%d. %s", i+1, name), notifier.FormatEUR(p.Cost30dEUR)+" / 30d"))
	}
	return g
}

func chartGroup(rep *report.Report) group {
	if rep.ChartURL == "" {
		return group{}
	}
	return group{
		title: "Costs, last 30 days",
		sub:   fmt.Sprintf("Total %s · average %s/day · chart: %s", notifier.FormatEUR(rep.TotalCost30dEUR), notifier.FormatEUR(rep.AvgCostPerDayEUR), rep.ChartURL),
	}
}

// ---- deletion summary ----

func renderDeletionSummary(summary *report.DeletionSummary) map[string]any {
	deleted, failed, skipped := summary.Counts()
	var body []map[string]any
	body = append(body,
		textBlock("costguard — deletion run", "weight", "Bolder", "size", "Large"),
		textBlock(summary.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"), "isSubtle", "true"),
		textBlock(fmt.Sprintf("Deleted %d · failed %d · skipped %d", deleted, failed, skipped), "weight", "Bolder"),
	)
	addSection := func(title string, rows []string) {
		if len(rows) == 0 {
			return
		}
		body = append(body, textBlock(title, "weight", "Bolder"))
		for _, r := range rows {
			body = append(body, textBlock("• "+r))
		}
	}
	var failedRows, skippedRows, deletedRows []string
	for _, r := range summary.ResultsByStatus(report.StatusFailed) {
		failedRows = append(failedRows, fmt.Sprintf("%s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason))
	}
	for _, r := range summary.ResultsByStatus(report.StatusSkipped) {
		skippedRows = append(skippedRows, fmt.Sprintf("%s %s (%s/%s): %s", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region, r.Reason))
	}
	for _, r := range summary.ResultsByStatus(report.StatusDeleted) {
		deletedRows = append(deletedRows, fmt.Sprintf("%s %s (%s/%s)", r.Item.Kind, r.Item.Name, r.Item.ProjectName, r.Item.Region))
	}
	addSection("Failures", failedRows)
	addSection("Skipped (protected)", skippedRows)
	addSection("Deleted", deletedRows)

	return map[string]any{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": "0076D4",
		"summary":    "costguard — deletion run",
		"attachments": []map[string]any{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content": map[string]any{
				"type":    "AdaptiveCard",
				"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
				"version": "1.4",
				"body":    body,
			},
		}},
	}
}

// ---- helpers ----

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func dryRunLabel(dry bool) string {
	if dry {
		return "DRY RUN"
	}
	return "live"
}

func folderName(g notifier.FolderGroup) string {
	if g.FolderName == "" {
		return "Organization (root)"
	}
	return g.FolderName
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
