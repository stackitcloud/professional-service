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
	"log/slog"
	"net/http"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/callback"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
)

// shutdownTimeout bounds the graceful-shutdown drain (in-flight protect
// writes finish or are abandoned — never started).
const shutdownTimeout = 15 * time.Second

// runCallback serves the protect endpoint until the context is cancelled
// (SIGTERM/SIGINT), then shuts down gracefully. The callback SA holds no deletion rights.
func runCallback(ctx context.Context, cfg config.Config, logger *slog.Logger) int {
	clients, err := newStackitClients()
	if err != nil {
		logger.Error("initializing STACKIT clients failed", "error", err)
		return ExitFatal
	}
	if cfg.SecretsManager.URL == "" {
		logger.Error("Secrets Manager URL is required for the callback (whitelist write target)",
			"env", config.EnvSMURL)
		return ExitFatal
	}
	store, err := newSMStore(cfg.SecretsManager.URL, cfg.SecretsManager.Username, cfg.SecretsManager.Password, cfg.WhitelistSecretPath, logger)
	if err != nil {
		logger.Error("connecting to the whitelist Secrets Manager failed", "error", err)
		return ExitFatal
	}

	srv := callback.New(cfg, clients.ResourceManager, clients.IaaS, store, cfg.CallbackSecret, logger)
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.CallbackPort),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("callback server listening", "port", cfg.CallbackPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("callback server failed", "error", err)
			return ExitFatal
		}
		return ExitOK
	case <-ctx.Done():
	}

	logger.Info("shutting down callback server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown did not complete in time", "error", err)
	}
	return ExitOK
}
