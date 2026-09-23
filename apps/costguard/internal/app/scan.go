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

package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/chart"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
)

// runScan is the weekly warning run: full scan (which also
// writes mark labels to new candidates), optional chart upload, then the
// report. Non-fatal scanner errors are surfaced in the report; a failed
// webhook delivery is fatal for the run.
func runScan(ctx context.Context, cfg config.Config, logger *slog.Logger) int {
	ctx, cancel := boundedRun(ctx)
	defer cancel()

	deps, err := setup(ctx, cfg, logger)
	if err != nil {
		logger.Error("setup failed", "error", err)
		return ExitFatal
	}

	scn := scanner.New(deps.Clients, deps.Whitelist, cfg, logger)
	rep, err := scn.Scan(ctx)
	if err != nil {
		logger.Error("scan failed", "error", err)
		return ExitFatal
	}

	attachChart(ctx, cfg, logger, rep)

	n, err := buildNotifier(cfg, logger)
	if err != nil {
		logger.Error("notifier setup failed", "error", err)
		return ExitFatal
	}
	if err := n.SendReport(ctx, rep); err != nil {
		logger.Error("sending the report failed", "error", err)
		return ExitFatal
	}

	if len(rep.ScanErrors) > 0 {
		logger.Warn("run completed with scanner errors (see report)", "count", len(rep.ScanErrors))
	}
	logger.Info("scan complete",
		"dryRun", rep.DryRun,
		"staleProjects", len(rep.StaleProjects),
		"emptySNAs", len(rep.EmptySNAs),
		"idlePublicIPs", len(rep.IdlePublicIPs),
		"detachedVolumes", len(rep.DetachedVolumes),
	)
	return ExitOK
}

// attachChart renders and uploads the cost chart when S3 is configured
// and stores the presigned URL on the report. Chart problems
// degrade gracefully: the report is sent without a chart.
func attachChart(ctx context.Context, cfg config.Config, logger *slog.Logger, rep *report.Report) {
	if !cfg.S3.Complete() {
		return
	}
	uploader, err := chart.NewUploader(ctx, cfg.S3)
	if err != nil {
		logger.Warn("chart upload disabled for this run: S3 client init failed", "error", err)
		return
	}
	png, err := chart.Render(rep.DailyCosts)
	if err != nil {
		logger.Warn("chart upload disabled for this run: render failed", "error", err)
		return
	}
	key := fmt.Sprintf("costguard/%s/report.png", time.Now().UTC().Format("2006-01-02T15-04-05Z"))
	url, err := uploader.UploadAndPresign(ctx, key, png)
	if err != nil {
		logger.Warn("chart upload failed, report will have no chart", "error", err)
		return
	}
	rep.ChartURL = url
	logger.Info("chart uploaded", "key", key)
}
