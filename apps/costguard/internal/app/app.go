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

// Package app runs one costguard subcommand:
//
//	report  the report run while delete is off: scan and post; never
//	        writes.
//	flag    the report run while delete is on: scan, post, then label new
//	        candidates. No message, no labels.
//	delete  the delete run: scan, delete what is labelled, post when
//	        something happened.
//	boot    once after the server was (re)created: wait for the login,
//	        scan and post like report; never writes.
//
// flag and delete refuse to run while delete is off in the configuration.
// A blocked run (a skip entry matches nothing) neither flags nor deletes.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/deleter"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/googlechat"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/slack"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/teams"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitFatal = 1
	ExitUsage = 2
)

// runTimeout bounds a run; the systemd unit's TimeoutStartSec is the outer
// bound.
const runTimeout = 60 * time.Minute

// sendTimeout is the budget for posting one message, retries included. It
// does not depend on the run: when a run times out or systemd stops it
// (SIGTERM), the message about what already happened must still go out.
// The units give costguard 60 s after SIGTERM (TimeoutStopSec), which
// covers it.
const sendTimeout = 45 * time.Second

// bootLoginWait is how long the boot run waits for the service account:
// Terraform attaches it only after the server was created.
const bootLoginWait = 5 * time.Minute

// Usage is the command line synopsis.
const Usage = "costguard [--config path] <report|flag|delete|boot>"

var modes = map[string]notifier.Mode{
	"report": notifier.ModeReport,
	"flag":   notifier.ModeFlag,
	"delete": notifier.ModeDelete,
	"boot":   notifier.ModeBoot,
}

// Replaced in tests.
var (
	newClients           = func() (*stackit.Set, error) { return stackit.New() }
	newDeleter           = deleter.New
	newScanner           = scanner.New
	now                  = time.Now
	logOutput  io.Writer = os.Stdout
)

// Options are the command line inputs.
type Options struct {
	Subcommand string
	ConfigPath string
	LogLevel   string
	Version    string
}

// Run executes the subcommand and returns the exit code.
func Run(ctx context.Context, opts Options) int {
	logger := newLogger(opts.LogLevel)
	mode, ok := modes[opts.Subcommand]
	if !ok {
		logger.Error("unknown subcommand", "subcommand", opts.Subcommand, "usage", Usage)
		return ExitUsage
	}
	cfg, err := config.LoadFromOS(opts.ConfigPath)
	if err == nil {
		err = cfg.Validate()
	}
	if err != nil {
		logger.Error("the configuration cannot be used", "error", err.Error())
		reportConfigProblem(ctx, logger, mode, opts, err)
		return ExitFatal
	}

	r := &run{
		mode:   mode,
		cfg:    *cfg,
		logger: logger,
		notify: buildNotifier(*cfg),
		compose: notifier.Composer{
			PortalURL:          cfg.PortalURL,
			DeleteEnabled:      cfg.DeleteEnabled,
			DeleteRunAt:        cfg.DeleteRunAt,
			ReportRunAt:        cfg.ReportRunAt,
			Location:           cfg.Location(),
			Prices:             cfg.Prices.Report(),
			WarnEmptyAfterDays: cfg.WarnEmptyAfterDays,
			Version:            opts.Version,
		},
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	return r.execute(ctx)
}

type run struct {
	mode    notifier.Mode
	cfg     config.Config
	logger  *slog.Logger
	notify  notifier.Notifier
	compose notifier.Composer
}

func (r *run) execute(ctx context.Context) int {
	if (r.mode == notifier.ModeFlag || r.mode == notifier.ModeDelete) && !r.cfg.DeleteEnabled {
		return r.fail(ctx, fmt.Errorf("the %s run only works with delete on, and the configuration has deleteEnabled: false. Nothing was changed; run terraform apply to bring the timers in line with the configuration", r.mode))
	}
	clients, err := newClients()
	if err == nil && clients.Login != nil {
		wait := time.Duration(0)
		if r.mode == notifier.ModeBoot {
			wait = bootLoginWait
		}
		err = clients.Login.Ready(ctx, wait)
	}
	if err != nil {
		return r.fail(ctx, fmt.Errorf("costguard cannot log in to STACKIT.\n  - %w", err))
	}
	sc := newScanner(clients, r.cfg, r.logger)
	sc.DeleteRun = r.mode == notifier.ModeDelete
	res, err := sc.Scan(ctx)
	if err != nil {
		return r.fail(ctx, err)
	}
	logReport(r.logger, res.Report)

	switch r.mode {
	case notifier.ModeReport, notifier.ModeBoot:
		if err := r.send(ctx, r.compose.Report(res.Report, r.mode)); err != nil {
			r.logger.Error("sending the report failed", "error", err)
			return ExitFatal
		}
		return ExitOK

	case notifier.ModeFlag:
		if err := r.send(ctx, r.compose.Report(res.Report, r.mode)); err != nil {
			r.logger.Error("sending the report failed; no labels were set, so nothing new gets deleted", "error", err)
			return ExitFatal
		}
		if res.Blocked() {
			r.logger.Error("a skip entry matches nothing; no labels were set", "entries", res.Report.Blocked)
			return ExitFatal
		}
		fr := newDeleter(clients.IaaS, r.logger).Flag(ctx, res)
		r.logger.Info("flag run complete", "flagged", fr.Flagged, "unflagged", fr.Unflagged, "cleared", fr.Cleared,
			"not_flagged", len(fr.NotFlagged), "not_cleared", len(fr.NotCleared))
		if fr.Failed() {
			// The message already went out: correct it in the chat.
			if err := r.send(ctx, r.compose.FlagProblems(fr.NotFlagged, fr.NotCleared)); err != nil {
				r.logger.Error("sending the correction failed", "error", err)
			}
			return ExitFatal
		}
		return ExitOK

	default:
		sum := newDeleter(clients.IaaS, r.logger).Delete(ctx, res, now().UTC())
		saved := report.Estimate(sum.DeletedByThisRun(), r.cfg.Prices.Report())
		r.logger.Info("delete run complete",
			"deleted", sum.Count(report.StatusDeleted), "failed", sum.Count(report.StatusFailed),
			"unflagged", sum.Count(report.StatusUnflagged), "deferred", sum.Count(report.StatusDeferred),
			"skipped", sum.Count(report.StatusSkipped), "saves_eur_per_month", saved.TotalEUR())
		if sum.HasNews() {
			if err := r.send(ctx, r.compose.Summary(sum)); err != nil {
				r.logger.Error("sending the deletion summary failed", "error", err)
				return ExitFatal
			}
		}
		if sum.Interrupted {
			r.logger.Error("the delete run was interrupted; the rest follows in the next run", "error", ctx.Err())
			return ExitFatal
		}
		if res.Blocked() {
			r.logger.Error("a skip entry matches nothing; nothing was deleted", "entries", res.Report.Blocked)
			return ExitFatal
		}
		return ExitOK
	}
}

// fail reports a run that stopped before changing anything.
func (r *run) fail(ctx context.Context, err error) int {
	err = readableStop(err)
	r.logger.Error("run failed before changing anything", "error", err.Error())
	if sendErr := r.send(ctx, r.compose.Failure(r.mode, err)); sendErr != nil {
		r.logger.Error("sending the failure message failed", "error", sendErr)
	}
	return ExitFatal
}

// send posts a message with its own time budget, so it still goes out when
// the run itself was cancelled.
func (r *run) send(ctx context.Context, msg notifier.Message) error {
	return sendWithBudget(ctx, r.notify, msg)
}

func sendWithBudget(ctx context.Context, n notifier.Notifier, msg notifier.Message) error {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()
	return n.Send(sendCtx, msg)
}

// reportConfigProblem posts a configuration problem to the chat when the
// chat settings themselves are usable, so a broken config does not make
// costguard fall silent. Otherwise the log is the only place.
func reportConfigProblem(ctx context.Context, logger *slog.Logger, mode notifier.Mode, opts Options, problem error) {
	output, webhookURL, ok := config.ChatFromOS(opts.ConfigPath)
	if !ok {
		logger.Error("the problem cannot be posted: the output or the webhook URL is not usable")
		return
	}
	n := buildNotifier(config.Config{Output: output, WebhookURL: webhookURL})
	msg := notifier.Composer{Version: opts.Version}.Failure(mode, problem)
	if err := sendWithBudget(ctx, n, msg); err != nil {
		logger.Error("posting the configuration problem failed", "error", err)
	}
}

// readableStop replaces Go's context errors with what happened.
func readableStop(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("the run took longer than %d minutes and was stopped", int(runTimeout.Minutes()))
	case errors.Is(err, context.Canceled):
		return errors.New("the run was stopped from outside, for example because the server was stopped or is being replaced")
	}
	return err
}

func buildNotifier(cfg config.Config) notifier.Notifier {
	switch cfg.Output {
	case config.OutputSlack:
		return slack.New(cfg.WebhookURL)
	case config.OutputTeams:
		return teams.New(cfg.WebhookURL)
	default:
		return googlechat.New(cfg.WebhookURL)
	}
}

// logReport writes every listed item to the log: messages are capped, the
// log is the full list.
func logReport(logger *slog.Logger, rep *report.Report) {
	for _, g := range []struct {
		category string
		items    []report.Item
	}{
		{"idle-public-ip", rep.IdlePublicIPs},
		{"detached-volume", rep.DetachedVolumes},
		{"requested", rep.Requested},
		{"volume-with-snapshots", rep.WithSnapshots},
		{"back-in-use", rep.BackInUse},
		{"protected-but-marked", rep.ProtectedMarked},
		{"empty-project", rep.EmptyProjects},
		{"empty-network-area", rep.EmptyNetworkAreas},
	} {
		for _, it := range g.items {
			logger.Info("finding", "category", g.category, "kind", it.Kind, "id", it.ID, "name", it.Name,
				"project", it.ProjectName, "project_id", it.ProjectID, "region", it.Region, "detail", it.Detail, "new", it.New)
		}
	}
	logger.Info("scan complete", "scope", rep.Scope, "to_delete", rep.ToDelete(),
		"skipped_folders", rep.SkippedFolders, "skipped_projects", rep.SkippedProjects,
		"scan_errors", len(rep.ScanErrors), "blocked", len(rep.Blocked))
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: lvl}))
}
