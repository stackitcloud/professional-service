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
	"log/slog"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/deleter"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
)

// runDelete is the weekly execution run: full
// re-scan → delete the due candidates (volumes and public IPs only)
// → send a separate confirmation. The scan→delete offset and the mark
// deadline guarantee that nothing first seen by this week's scan is
// deleted by this week's delete run.
//
// Dry run is the global kill switch: with COSTGUARD_DRY_RUN=true (the
// default) this subcommand performs the scan and sends the report but
// never issues a DELETE call.
func runDelete(ctx context.Context, cfg config.Config, logger *slog.Logger) int {
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

	n, err := buildNotifier(cfg, logger)
	if err != nil {
		logger.Error("notifier setup failed", "error", err)
		return ExitFatal
	}

	if cfg.DryRun {
		logger.Warn("dry run enabled — scanning and reporting only, no deletions",
			"idlePublicIPs", len(rep.IdlePublicIPs), "detachedVolumes", len(rep.DetachedVolumes))
		if err := n.SendReport(ctx, rep); err != nil {
			logger.Error("sending the report failed", "error", err)
			return ExitFatal
		}
		return ExitOK
	}

	del := deleter.New(deps.Clients.IaaS, deps.Whitelist, cfg, logger)
	summary := del.DeleteAll(ctx, rep)
	deleted, failed, skipped := summary.Counts()

	// The confirmation is sent even when every deletion failed
	//. A confirmation-delivery failure is fatal for the run.
	if err := n.SendDeletionSummary(ctx, summary); err != nil {
		logger.Error("sending the deletion confirmation failed", "error", err)
		return ExitFatal
	}

	if failed > 0 {
		logger.Error("delete run completed with failures (see confirmation)",
			"deleted", deleted, "failed", failed, "skipped", skipped)
	} else {
		logger.Info("delete run complete", "deleted", deleted, "skipped", skipped)
	}
	return ExitOK
}
