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

// Package app wires configuration, STACKIT clients, the merged whitelist
// and the selected notifier, then dispatches the subcommand. Fatal setup
// errors exit non-zero; non-fatal scan errors are carried in the report and
// never abort the run.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/googlechat"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/slack"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier/teams"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// Exit codes: 0 success, 1 fatal run error, 2 usage error.
const (
	ExitOK    = 0
	ExitFatal = 1
	ExitUsage = 2
)

// runTimeout bounds a scan/delete run. The CronJob's activeDeadlineSeconds is the outer bound.
const runTimeout = 30 * time.Minute

// Subcommand names.
const (
	SubcommandScan     = "scan"
	SubcommandDelete   = "delete"
	SubcommandCallback = "callback"
)

// deps is the shared dependency bundle built once per run.
type deps struct {
	Clients   *stackit.Set
	Whitelist *whitelist.Whitelist
}

// Constructors are variables so tests can inject fakes; production uses
// the real SDK/Vault clients.
var (
	newStackitClients = stackit.New
	newSMStore        = whitelist.NewStore
)

// Run loads and validates the configuration, wires dependencies and
// dispatches the subcommand. It returns the process exit code.
func Run(ctx context.Context, subcommand, configPath, logLevel string) int {
	logger := newLogger(logLevel)
	loaded, err := config.LoadFromOS(configPath)
	if err != nil {
		logger.Error("loading configuration failed", "error", err)
		return ExitFatal
	}
	cfg := *loaded
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err.Error())
		return ExitFatal
	}
	switch subcommand {
	case SubcommandScan:
		return runScan(ctx, cfg, logger)
	case SubcommandDelete:
		return runDelete(ctx, cfg, logger)
	case SubcommandCallback:
		return runCallback(ctx, cfg, logger)
	default:
		logger.Error("unknown subcommand", "subcommand", subcommand,
			"usage", "costguard [--config path] <scan|delete|callback>")
		return ExitUsage
	}
}

// newLogger builds the structured logger from LOG_LEVEL
// (debug|info|warn|error; default info).
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
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// setup builds the STACKIT client set and the merged whitelist. A
// whitelist store failure is fatal: running without the shared whitelist
// would break the protection model (fail-closed).
func setup(ctx context.Context, cfg config.Config, logger *slog.Logger) (*deps, error) {
	clients, err := newStackitClients()
	if err != nil {
		return nil, fmt.Errorf("initializing STACKIT clients: %w", err)
	}
	wl, err := loadWhitelist(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	return &deps{Clients: clients, Whitelist: wl}, nil
}

// loadWhitelist merges the static (env+YAML) whitelist with the Secrets
// Manager entry when one is configured.
func loadWhitelist(ctx context.Context, cfg config.Config, logger *slog.Logger) (*whitelist.Whitelist, error) {
	entries := map[string]whitelist.Entry{}
	if cfg.WhitelistSecretPath != "" {
		if cfg.SecretsManager.URL == "" {
			return nil, errors.New("whitelistSecretPath is set but secretsManager.url is missing")
		}
		store, err := newSMStore(cfg.SecretsManager.URL, cfg.SecretsManager.Username, cfg.SecretsManager.Password, cfg.WhitelistSecretPath, logger)
		if err != nil {
			return nil, fmt.Errorf("connecting to the whitelist Secrets Manager: %w", err)
		}
		entries, err = store.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading the shared whitelist (failing closed): %w", err)
		}
	}
	return whitelist.New(cfg.Whitelist, entries, logger), nil
}

// buildNotifier selects the output adapter from COSTGUARD_OUTPUT. Buttons
// are enabled when the callback URL + secret are set.
func buildNotifier(cfg config.Config, logger *slog.Logger) (notifier.Notifier, error) {
	btn := notifier.NewButton(cfg.CallbackURL, cfg.CallbackSecret)
	switch cfg.Output {
	case config.OutputGoogleChat:
		return googlechat.New(cfg.WebhookURL, btn, logger), nil
	case config.OutputSlack:
		return slack.New(cfg.WebhookURL, btn, logger), nil
	case config.OutputTeams:
		return teams.New(cfg.WebhookURL, btn, logger), nil
	default:
		return nil, fmt.Errorf("unsupported output mode %q (validated at startup — internal error)", cfg.Output)
	}
}

// boundedRun returns the run context: the caller's context (signal-aware)
// capped at runTimeout.
func boundedRun(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, runTimeout)
}
