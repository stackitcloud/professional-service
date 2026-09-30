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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/budget"
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

const (
	ExitOK    = 0
	ExitFatal = 1
	ExitUsage = 2
)

const runTimeout = 60 * time.Minute

const sendTimeout = 45 * time.Second

const bootLoginWait = 5 * time.Minute

const Usage = "costguard [--config path] <report|flag|delete|budgets|boot>"

var modes = map[string]notifier.Mode{
	"report":  notifier.ModeReport,
	"flag":    notifier.ModeFlag,
	"delete":  notifier.ModeDelete,
	"budgets": notifier.ModeBudgets,
	"boot":    notifier.ModeBoot,
}

var (
	newClients           = func() (*stackit.Set, error) { return stackit.New() }
	newDeleter           = deleter.New
	newScanner           = scanner.New
	newChecker           = budget.New
	now                  = time.Now
	logOutput  io.Writer = os.Stdout
)

type Options struct {
	Subcommand string
	ConfigPath string
	LogLevel   string
	Version    string
}

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
			Organization:       cfg.OrganizationID,
			OrganizationID:     cfg.OrganizationID,
			PortalURL:          cfg.PortalURL,
			DeleteEnabled:      cfg.DeleteEnabled,
			DeleteRunAt:        cfg.DeleteRunAt,
			ReportRunAt:        cfg.ReportRunAt,
			BudgetsRunAt:       cfg.BudgetsRunAt(),
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
	if r.mode == notifier.ModeBudgets && r.cfg.Budgets == nil {
		return r.fail(ctx, errors.New("the budgets run only works with budgets on, and the configuration has no budgets. Run terraform apply to bring the timers in line with the configuration"))
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
	r.nameOrganization(ctx, clients)
	switch r.mode {
	case notifier.ModeBudgets:
		return r.budgets(ctx, clients)
	case notifier.ModeBoot:
		return r.boot(ctx, clients)
	}
	sc := newScanner(clients, r.cfg, r.logger)
	sc.DeleteRun = r.mode == notifier.ModeDelete
	res, err := sc.Scan(ctx)
	if err != nil {
		return r.fail(ctx, err)
	}
	logReport(r.logger, res.Report)

	switch r.mode {
	case notifier.ModeReport:
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

func (r *run) nameOrganization(ctx context.Context, clients *stackit.Set) {
	name, err := clients.ResourceManager.OrganizationName(ctx, r.cfg.OrganizationID)
	if name = strings.TrimSpace(name); err != nil || name == "" {
		r.logger.Warn("the organization's name could not be read; the messages show its ID", "error", err)
		return
	}
	r.compose.Organization = name
}

func (r *run) budgets(ctx context.Context, clients *stackit.Set) int {
	chk, err := newChecker(clients, r.cfg, r.logger).Check(ctx)
	if err != nil {
		return r.fail(ctx, err)
	}
	logBudgets(r.logger, chk)
	if chk.NothingIn() {
		r.logger.Info("no cost of this month is in yet; nothing to check", "month", chk.Month.Format("2006-01"),
			"last_modified", chk.LastModified)
		return ExitOK
	}
	if msg, post := r.compose.Budgets(chk); post {
		if err := r.send(ctx, msg); err != nil {
			r.logger.Error("sending the budgets message failed", "error", err)
			return ExitFatal
		}
	}
	if len(chk.Problems) > 0 {
		return ExitFatal
	}
	return ExitOK
}

func (r *run) boot(ctx context.Context, clients *stackit.Set) int {
	var rep *report.Report
	if r.cfg.ReportEnabled {
		res, err := newScanner(clients, r.cfg, r.logger).Scan(ctx)
		if err != nil {
			return r.fail(ctx, err)
		}
		logReport(r.logger, res.Report)
		rep = res.Report
	}
	var chk *report.BudgetCheck
	var budgetErr error
	if r.cfg.Budgets != nil {
		if chk, budgetErr = newChecker(clients, r.cfg, r.logger).Check(ctx); budgetErr == nil {
			logBudgets(r.logger, chk)
		} else {
			budgetErr = readableStop(budgetErr)
			r.logger.Error("checking the budgets failed", "error", budgetErr.Error())
		}
	}
	if err := r.send(ctx, r.compose.Boot(rep, chk, budgetErr, now())); err != nil {
		r.logger.Error("sending the boot message failed", "error", err)
		return ExitFatal
	}
	if budgetErr != nil || (chk != nil && len(chk.Problems) > 0) {
		return ExitFatal
	}
	return ExitOK
}

func (r *run) fail(ctx context.Context, err error) int {
	err = readableStop(err)
	r.logger.Error("run failed before changing anything", "error", err.Error())
	if sendErr := r.send(ctx, r.compose.Failure(r.mode, err)); sendErr != nil {
		r.logger.Error("sending the failure message failed", "error", sendErr)
	}
	return ExitFatal
}

func (r *run) send(ctx context.Context, msg notifier.Message) error {
	return sendWithBudget(ctx, r.notify, msg)
}

func sendWithBudget(ctx context.Context, n notifier.Notifier, msg notifier.Message) error {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()
	return n.Send(sendCtx, msg)
}

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

func logBudgets(logger *slog.Logger, chk *report.BudgetCheck) {
	for _, b := range chk.Budgets {
		logger.Info("budget", "name", b.Name, "target", b.Target, "month_eur", b.MonthEUR, "day_eur", b.DayEUR,
			"limit_eur", b.LimitEUR, "reached", b.Reached, "forecast_eur", b.ForecastEUR)
	}
	for _, p := range chk.Problems {
		logger.Error("budget not checked", "problem", p)
	}
	logger.Info("budgets checked", "month", chk.Month.Format("2006-01"), "checked_up_to", chk.Checked.Format("2006-01-02"),
		"last_modified", chk.LastModified, "reached", len(chk.Reached()), "problems", len(chk.Problems))
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
