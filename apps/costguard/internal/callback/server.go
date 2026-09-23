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

// Package callback implements the interactive "Do not delete" endpoint
//: GET /protect (Google Chat / Slack openBrowser links)
// and POST /protect (Teams Action.Http), plus /healthz. A valid click
// appends the resource to the shared whitelist in the dedicated Secrets
// Manager instance (KV v2 CAS, idempotent). The callback is stateless and
// holds no deletion rights.
package callback

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	oapierror "github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// maxBodyBytes bounds the POST /protect body (a button body is ~200 bytes;
// anything larger is an attack or a bug).
const maxBodyBytes = 64 * 1024

// whitelistAppender is the whitelist write surface the callback needs
// (satisfied by *whitelist.Store). An interface keeps tests hermetic.
type whitelistAppender interface {
	Append(ctx context.Context, id string, e whitelist.Entry) error
}

// Server serves the protect endpoint.
type Server struct {
	Config config.Config
	RM     stackit.ResourceManager
	IaaS   stackit.IaaS
	Store  whitelistAppender
	Logger *slog.Logger

	// Now is the clock for expiry checks (tests).
	Now func() time.Time

	handlers http.Handler
}

// New builds the callback server. secret is COSTGUARD_CALLBACK_SECRET.
func New(cfg config.Config, rm stackit.ResourceManager, iaas stackit.IaaS, store whitelistAppender, secret string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		Config: cfg,
		RM:     rm,
		IaaS:   iaas,
		Store:  store,
		Logger: logger,
		Now:    time.Now,
	}
	s.handlers = s.routes(secret)
	return s
}

// Handler returns the HTTP handler (mux). Exposed for tests and for the
// app to wrap with middleware.
func (s *Server) Handler() http.Handler { return s.handlers }

func (s *Server) routes(secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /healthz", s.handleHealth)
	mux.HandleFunc("GET /protect", s.handleProtect(http.MethodGet, secret))
	mux.HandleFunc("POST /protect", s.handleProtect(http.MethodPost, secret))
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// handleProtect implements the protect flow:
// 1. parse parameters (400 on missing)
// 2. verify HMAC signature (403)
// 3. check expiry (410 Gone)
// 4. existence check (410 Gone if the resource no longer exists)
// 5. idempotent whitelist append (5xx on store failure)
//
// Security: the query string / raw body are never logged — only the resource id and type.
func (s *Server) handleProtect(method, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		req, status, msg := s.parseRequest(method, r)
		if msg != "" {
			writePlain(w, status, msg)
			return
		}

		if !notifier.VerifySig([]byte(secret), req.ID, string(req.Type), req.Project, req.Region, req.Exp, req.Sig) {
			s.Logger.Warn("protect rejected: invalid signature", "id", req.ID, "type", req.Type)
			writePlain(w, http.StatusForbidden, "invalid signature")
			return
		}
		if time.Unix(req.Exp, 0).Before(s.Now()) {
			s.Logger.Info("protect rejected: button expired", "id", req.ID, "type", req.Type)
			writePlain(w, http.StatusGone, "this button has expired — the resource was not protected")
			return
		}
		if status, msg := s.exists(ctx, req); msg != "" {
			s.Logger.Warn("protect rejected: resource gone", "id", req.ID, "type", req.Type)
			writePlain(w, status, msg)
			return
		}
		if err := s.Store.Append(ctx, req.ID, whitelist.Entry{
			Type:    req.Type,
			SavedBy: "protect-button",
			SavedAt: s.Now().UTC(),
		}); err != nil {
			s.Logger.Error("whitelist append failed", "id", req.ID, "type", req.Type, "error", err)
			writePlain(w, http.StatusInternalServerError, "could not save the protection — please try again")
			return
		}
		s.Logger.Info("resource protected", "id", req.ID, "type", req.Type)
		writePlain(w, http.StatusOK, "protected: this resource will not be deleted by costguard")
	}
}

// exists checks the resource still exists (410 Gone otherwise).
// It returns (status, message); a zero status means "exists".
func (s *Server) exists(ctx context.Context, req protectRequest) (int, string) {
	var err error
	switch req.Type {
	case whitelist.TypeProject:
		_, err = s.RM.GetProject(ctx, req.ID)
	case whitelist.TypeFolder:
		_, err = s.RM.GetFolder(ctx, req.ID)
	case whitelist.TypeVolume:
		_, err = s.IaaS.GetVolume(ctx, req.Project, req.Region, req.ID)
	case whitelist.TypePublicIP:
		_, err = s.IaaS.GetPublicIP(ctx, req.Project, req.Region, req.ID)
	default:
		return http.StatusBadRequest, "unknown resource type"
	}
	if err == nil {
		return 0, ""
	}
	if isNotFound(err) {
		return http.StatusGone, "this resource no longer exists — nothing to protect"
	}
	s.Logger.Error("existence check failed", "id", req.ID, "type", req.Type, "error", err)
	return http.StatusServiceUnavailable, "existence check temporarily unavailable — please try again"
}

func writePlain(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(msg))
}

func isNotFound(err error) bool {
	var apiErr *oapierror.GenericOpenAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode == 404
	}
	return false
}
