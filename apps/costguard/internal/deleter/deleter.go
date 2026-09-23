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

// Package deleter implements the execution phase: it deletes
// only candidates whose mark deadline has passed, re-validating every
// criterion immediately before each DELETE (fail-closed). v1 deletes
// public IPs and detached volumes only — there is no project or SNA
// deletion path on purpose. Deletions are strictly sequential with a
// 1-second delay; one resource's failure never aborts the run.
package deleter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	oapierror "github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// Default between-deletion delay and retry parameters.
const (
	DefaultBetweenDeletions = time.Second
	DefaultRetryBackoff     = 200 * time.Millisecond
	DefaultMaxAttempts      = 3
)

// Deleter deletes due candidates from a scan report.
type Deleter struct {
	IaaS      stackit.IaaS
	Whitelist *whitelist.Whitelist
	Config    config.Config
	Logger    *slog.Logger

	// BetweenDeletions is the pause between individual deletions
	// (1 s against rate limiting).
	BetweenDeletions time.Duration
	// RetryBackoff is the base backoff for transient retries; attempt n
	// waits RetryBackoff << (n-1).
	RetryBackoff time.Duration
	// MaxAttempts bounds transient-error retries (max 3).
	MaxAttempts int
	// Now is the clock used for deadline checks.
	Now func() time.Time
}

// New builds a Deleter with production defaults.
func New(iaas stackit.IaaS, wl *whitelist.Whitelist, cfg config.Config, logger *slog.Logger) *Deleter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Deleter{
		IaaS:             iaas,
		Whitelist:        wl,
		Config:           cfg,
		Logger:           logger,
		BetweenDeletions: DefaultBetweenDeletions,
		RetryBackoff:     DefaultRetryBackoff,
		MaxAttempts:      DefaultMaxAttempts,
		Now:              time.Now,
	}
}

// DeleteAll deletes the due candidates of the report in the fixed order
// public IPs → volumes. It returns the per-item summary
//; a context cancellation stops the run and returns the
// partial summary (re-running after a crash is safe).
func (d *Deleter) DeleteAll(ctx context.Context, rep *report.Report) *report.DeletionSummary {
	summary := &report.DeletionSummary{GeneratedAt: d.Now().UTC()}
	first := true
	for _, ip := range rep.IdlePublicIPs {
		if ctx.Err() != nil {
			break
		}
		if !first {
			d.delay(ctx)
		}
		first = false
		summary.Results = append(summary.Results, d.deletePublicIP(ctx, ip))
	}
	for _, v := range rep.DetachedVolumes {
		if ctx.Err() != nil {
			break
		}
		if !first {
			d.delay(ctx)
		}
		first = false
		summary.Results = append(summary.Results, d.deleteVolume(ctx, v))
	}
	return summary
}

// delay pauses BetweenDeletions between deletions.
func (d *Deleter) delay(ctx context.Context) {
	if d.BetweenDeletions <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d.BetweenDeletions):
	}
}

// deleteWithRetry runs fn with exponential backoff on transient errors
// (429, 409, 5xx) up to MaxAttempts. A 404 from a delete call
// is success — "already gone".
func (d *Deleter) deleteWithRetry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= d.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if isNotFound(lastErr) {
			return nil
		}
		if !isTransient(lastErr) {
			return lastErr
		}
		if attempt == d.MaxAttempts {
			break
		}
		d.Logger.Debug("transient delete error, retrying", "attempt", attempt, "error", lastErr)
		backoff := d.RetryBackoff << uint(attempt-1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

// safeLabeled reports whether the resource carries the configured safe
// label.
func (d *Deleter) safeLabeled(labels map[string]string) bool {
	return labels[d.Config.SafeLabelKey] == d.Config.SafeLabelValue
}

// logDeletion records the mandatory deletion audit line: name, ID,
// project and region.
func (d *Deleter) logDeletion(kind string, item report.DeletionItem) {
	d.Logger.Info("deleting "+kind,
		"kind", kind,
		"name", item.Name,
		"id", item.ID,
		"project", item.ProjectName,
		"project_id", item.ProjectID,
		"region", item.Region,
	)
}

func deadlineString(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}

// statusCode extracts the HTTP status from an SDK error, if any.
func statusCode(err error) (int, bool) {
	var apiErr *oapierror.GenericOpenAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode, true
	}
	return 0, false
}

// isNotFound reports whether err is an HTTP 404 ("already gone").
func isNotFound(err error) bool {
	code, ok := statusCode(err)
	return ok && code == 404
}

// isTransient reports whether err is retryable: 429 (rate limit), 409
// (conflict / dependent-resource release race) or a 5xx.
func isTransient(err error) bool {
	code, ok := statusCode(err)
	if !ok {
		return false
	}
	return code == 429 || code == 409 || (code >= 500 && code < 600)
}
