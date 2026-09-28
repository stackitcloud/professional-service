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

// Package deleter performs the writes of the flag and delete runs. Every
// resource is re-read right before it is deleted, and nothing is deleted
// when the re-read disagrees with the scan. Deletions are sequential; one
// failure never stops the run.
package deleter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// Production defaults.
const (
	DefaultBetweenDeletions = time.Second
	DefaultRetryBackoff     = 2 * time.Second
	DefaultMaxAttempts      = 3
	DefaultServerWait       = 5 * time.Minute
	DefaultPollInterval     = 10 * time.Second
)

// Deleter writes labels and deletes resources.
type Deleter struct {
	IaaS   stackit.IaaS
	Logger *slog.Logger

	BetweenDeletions time.Duration
	RetryBackoff     time.Duration
	MaxAttempts      int
	// ServerWait bounds how long the run waits for deleted servers to be
	// gone, so their volumes and IPs can go in the same run.
	ServerWait   time.Duration
	PollInterval time.Duration
}

// New builds a Deleter with production defaults.
func New(iaas stackit.IaaS, logger *slog.Logger) *Deleter {
	return &Deleter{
		IaaS:             iaas,
		Logger:           logger,
		BetweenDeletions: DefaultBetweenDeletions,
		RetryBackoff:     DefaultRetryBackoff,
		MaxAttempts:      DefaultMaxAttempts,
		ServerWait:       DefaultServerWait,
		PollInterval:     DefaultPollInterval,
	}
}

// FlagResult is what the label writes of a flag run did. Every write
// happens after the message went out, so failures need a follow-up.
type FlagResult struct {
	Flagged, Unflagged, Cleared int
	// NotFlagged were announced for deletion but could not be marked, so
	// they are not deleted this week. Detail holds the reason.
	NotFlagged []report.Item
	// NotCleared still carry a delete label the message said was removed.
	// They are not deleted either way (in use or protected).
	NotCleared []report.Item
}

// Failed reports whether any label write failed.
func (f FlagResult) Failed() bool {
	return len(f.NotFlagged)+len(f.NotCleared) > 0
}

// Flag sets delete=true on the new cleanup candidates and removes it from
// volumes and public IPs that are in use again or protected by
// do-not-delete. The caller has already delivered the message and checked
// that the run is not blocked.
func (d *Deleter) Flag(ctx context.Context, res *scanner.Result) FlagResult {
	var out FlagResult
	item := itemFor(res)
	failed := func(r stackit.Resource, what string, err error) report.Item {
		d.Logger.Error(what+" failed", "kind", r.Kind, "id", r.ID, "project", r.ProjectID, "error", err)
		it := item(r)
		it.Detail = writeError(err)
		return it
	}
	value := stackit.LabelTrue
	for _, r := range res.Flag {
		if err := d.IaaS.SetLabel(ctx, r, stackit.LabelDelete, &value); err != nil {
			out.NotFlagged = append(out.NotFlagged, failed(r, "flagging", err))
			continue
		}
		d.Logger.Info("flagged", "kind", r.Kind, "id", r.ID, "name", r.Name, "project", r.ProjectID, "region", r.Region)
		out.Flagged++
	}
	for _, r := range res.Unflag {
		if err := d.IaaS.SetLabel(ctx, r, stackit.LabelDelete, nil); err != nil {
			out.NotCleared = append(out.NotCleared, failed(r, "removing the delete label", err))
			continue
		}
		d.Logger.Info("delete label removed, in use again", "kind", r.Kind, "id", r.ID, "project", r.ProjectID)
		out.Unflagged++
	}
	for _, r := range res.ClearStale {
		if err := d.IaaS.SetLabel(ctx, r, stackit.LabelDelete, nil); err != nil {
			out.NotCleared = append(out.NotCleared, failed(r, "removing the stale delete label", err))
			continue
		}
		d.Logger.Info("stale delete label removed, do-not-delete wins", "kind", r.Kind, "id", r.ID, "project", r.ProjectID)
		out.Cleared++
	}
	return out
}

// writeError is the reason shown for a failed label write.
func writeError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "the run was stopped before this label was written"
	}
	return stackit.Describe(err)
}

// itemFor returns a function that finds the message entry of a resource,
// so failures are shown with the same name and project as in the message.
func itemFor(res *scanner.Result) func(stackit.Resource) report.Item {
	items := itemIndex(res.Report)
	return func(r stackit.Resource) report.Item {
		if it, ok := items[r.ID]; ok {
			it.Detail = ""
			return it
		}
		return report.Item{Kind: string(r.Kind), ID: r.ID, Name: r.Name, ProjectID: r.ProjectID,
			ProjectName: res.Context.ProjectNames[r.ProjectID], Region: r.Region, NetworkID: r.NetworkID}
	}
}

// Delete removes the delete label from volumes and IPs that are in use
// again, then deletes what the scan found, in its order. A blocked run
// does nothing.
func (d *Deleter) Delete(ctx context.Context, res *scanner.Result, now time.Time) *report.DeletionSummary {
	rep := res.Report
	sum := &report.DeletionSummary{GeneratedAt: now, Scope: rep.Scope, Blocked: rep.Blocked, ScanErrors: rep.ScanErrors}
	if res.Blocked() {
		return sum
	}
	items := itemIndex(rep)
	withoutDetail := itemFor(res)
	item := func(r stackit.Resource) report.Item {
		if it, ok := items[r.ID]; ok {
			return it
		}
		return withoutDetail(r)
	}

	for _, r := range res.Unflag {
		if ctx.Err() != nil {
			sum.Interrupted = true
			return sum
		}
		sum.Results = append(sum.Results, d.unflag(ctx, r, item(r)))
	}

	deletedServers := map[string]stackit.Resource{}
	waited := false
	for i, r := range res.Delete {
		if ctx.Err() != nil {
			sum.Interrupted = true
			break
		}
		if r.Kind != stackit.KindServer && !waited {
			d.waitGone(ctx, deletedServers)
			waited = true
		}
		if i > 0 {
			d.sleep(ctx, d.BetweenDeletions)
		}
		result := d.deleteOne(ctx, r, item(r), deletedServers, res.Context)
		if r.Kind == stackit.KindServer && result.Status == report.StatusDeleted {
			deletedServers[r.ID] = r
		}
		sum.Results = append(sum.Results, result)
	}
	return sum
}

func itemIndex(rep *report.Report) map[string]report.Item {
	out := map[string]report.Item{}
	for _, list := range [][]report.Item{rep.IdlePublicIPs, rep.DetachedVolumes, rep.Requested, rep.BackInUse, rep.ProtectedMarked} {
		for _, it := range list {
			out[it.ID] = it
		}
	}
	return out
}

// unflag removes the delete label from a volume or IP that the scan saw
// in use, after checking it still is.
func (d *Deleter) unflag(ctx context.Context, r stackit.Resource, it report.Item) report.Result {
	cur, err := d.get(ctx, r)
	switch {
	case stackit.IsNotFound(err):
		return report.Result{Item: it, Status: report.StatusSkipped, Reason: "gone"}
	case err != nil:
		return report.Result{Item: it, Status: report.StatusFailed, Reason: "re-check: " + stackit.Describe(err)}
	case !stackit.Requested(cur.Labels):
		return report.Result{Item: it, Status: report.StatusSkipped, Reason: "the delete label was already removed"}
	case !inUse(*cur):
		return report.Result{Item: it, Status: report.StatusDeferred, Reason: "unused again; it stays labelled"}
	}
	return d.removeLabel(ctx, r, it, it.Detail)
}

func inUse(r stackit.Resource) bool {
	if r.Kind == stackit.KindVolume {
		return r.Status != stackit.VolumeStatusAvailable || r.ServerID != ""
	}
	return r.NICID != ""
}

func (d *Deleter) removeLabel(ctx context.Context, r stackit.Resource, it report.Item, why string) report.Result {
	if err := d.IaaS.SetLabel(ctx, r, stackit.LabelDelete, nil); err != nil {
		d.Logger.Error("removing the delete label failed", "kind", r.Kind, "id", r.ID, "error", err)
		return report.Result{Item: it, Status: report.StatusFailed, Reason: "removing the delete label: " + stackit.Describe(err)}
	}
	d.Logger.Info("delete label removed, in use again", "kind", r.Kind, "id", r.ID, "reason", why)
	return report.Result{Item: it, Status: report.StatusUnflagged, Reason: why}
}

// deleteOne re-reads the resource and deletes it if every rule still
// holds.
func (d *Deleter) deleteOne(ctx context.Context, r stackit.Resource, it report.Item, deletedServers map[string]stackit.Resource, dc scanner.DeleteContext) report.Result {
	cur, err := d.get(ctx, r)
	switch {
	case stackit.IsNotFound(err):
		return report.Result{Item: it, Status: report.StatusDeleted, Reason: "already gone", AlreadyGone: true}
	case err != nil:
		return report.Result{Item: it, Status: report.StatusFailed, Reason: "re-check: " + stackit.Describe(err)}
	case stackit.Protected(cur.Labels):
		return report.Result{Item: it, Status: report.StatusSkipped, Reason: "do-not-delete is set"}
	case !stackit.Requested(cur.Labels):
		return report.Result{Item: it, Status: report.StatusSkipped, Reason: "the delete label was removed"}
	}

	switch cur.Kind {
	case stackit.KindVolume:
		if inUse(*cur) {
			if keep, why := d.keepForServer(ctx, cur.ServerID, *cur, deletedServers); keep {
				return report.Result{Item: it, Status: report.StatusDeferred, Reason: why}
			}
			return d.removeLabel(ctx, r, it, "attached to a server again")
		}
	case stackit.KindPublicIP:
		if cur.NICID != "" {
			if keep, why := d.keepForServer(ctx, dc.NICServers[cur.NICID], *cur, deletedServers); keep {
				return report.Result{Item: it, Status: report.StatusDeferred, Reason: why}
			}
			return d.removeLabel(ctx, r, it, "attached to a network interface again")
		}
		if lb, used := dc.LoadBalancerAddresses[cur.ProjectID+"/"+cur.Region][cur.Address]; used {
			return d.removeLabel(ctx, r, it, "used by "+lb)
		}
	}

	d.Logger.Info("deleting", "kind", r.Kind, "id", r.ID, "name", it.Name, "project", it.ProjectName, "project_id", r.ProjectID, "region", r.Region)
	err = d.withRetry(ctx, func(ctx context.Context) error { return d.IaaS.Delete(ctx, r) })
	switch {
	case err == nil || stackit.IsNotFound(err):
		d.Logger.Info("deleted", "kind", r.Kind, "id", r.ID)
		return report.Result{Item: it, Status: report.StatusDeleted}
	case isConflict(err):
		d.Logger.Warn("delete refused, still in use", "kind", r.Kind, "id", r.ID, "error", err)
		return report.Result{Item: it, Status: report.StatusFailed, Reason: "still in use (conflict); tried again next run"}
	default:
		d.Logger.Error("delete failed", "kind", r.Kind, "id", r.ID, "error", err)
		return report.Result{Item: it, Status: report.StatusFailed, Reason: stackit.Describe(err)}
	}
}

const serverGoingReason = "still attached to a server that is being deleted or is marked delete=true; next run"

// keepForServer decides whether a volume or public IP that is still
// attached keeps its delete label and waits for the next run, and why. It
// does when its server is on its way out (deleted in this run, being
// deleted, already gone, or itself marked delete=true, e.g. because its
// delete failed in this run) and when the server cannot be read: a label is
// only removed when the server is known to stay.
func (d *Deleter) keepForServer(ctx context.Context, serverID string, attached stackit.Resource, deletedServers map[string]stackit.Resource) (bool, string) {
	if serverID == "" {
		return false, ""
	}
	if _, deleted := deletedServers[serverID]; deleted {
		return true, serverGoingReason
	}
	srv, err := d.get(ctx, stackit.Resource{Kind: stackit.KindServer, ID: serverID, ProjectID: attached.ProjectID, Region: attached.Region})
	switch {
	case stackit.IsNotFound(err):
		return true, serverGoingReason
	case err != nil:
		return true, "its server could not be checked (" + stackit.Describe(err) + "); next run"
	case srv.Status == stackit.ServerStatusDeleting || (stackit.Requested(srv.Labels) && !stackit.Protected(srv.Labels)):
		return true, serverGoingReason
	}
	return false, ""
}

// get re-reads a resource, retrying rate limiting and server errors like a
// delete: one hiccup must not cost a week.
func (d *Deleter) get(ctx context.Context, r stackit.Resource) (*stackit.Resource, error) {
	var cur *stackit.Resource
	err := d.withRetry(ctx, func(ctx context.Context) error {
		var err error
		cur, err = d.IaaS.Get(ctx, r)
		return err
	})
	return cur, err
}

// waitGone polls the deleted servers until they are gone or ServerWait
// has passed.
func (d *Deleter) waitGone(ctx context.Context, servers map[string]stackit.Resource) {
	if len(servers) == 0 {
		return
	}
	deadline := time.Now().Add(d.ServerWait)
	left := map[string]stackit.Resource{}
	for id, ref := range servers {
		left[id] = ref
	}
	for len(left) > 0 && ctx.Err() == nil {
		for id, ref := range left {
			if _, err := d.IaaS.Get(ctx, ref); stackit.IsNotFound(err) {
				delete(left, id)
			}
		}
		if len(left) == 0 || time.Now().After(deadline) {
			break
		}
		d.sleep(ctx, d.PollInterval)
	}
	if len(left) > 0 {
		d.Logger.Warn("servers still being deleted; their volumes and IPs go in the next run", "count", len(left))
	}
}

func (d *Deleter) withRetry(ctx context.Context, fn func(context.Context) error) error {
	var err error
	for attempt := 1; attempt <= d.MaxAttempts; attempt++ {
		if err = fn(ctx); err == nil || !isTransient(err) || attempt == d.MaxAttempts {
			return err
		}
		d.Logger.Debug("transient error, retrying", "attempt", attempt, "error", err)
		d.sleep(ctx, d.RetryBackoff<<uint(attempt-1))
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return err
}

func (d *Deleter) sleep(ctx context.Context, dur time.Duration) {
	if dur <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(dur):
	}
}

// isTransient: rate limiting and server errors are retried.
func isTransient(err error) bool {
	code, ok := stackit.StatusCode(err)
	return ok && (code == 429 || code >= 500)
}

func isConflict(err error) bool {
	code, ok := stackit.StatusCode(err)
	return ok && code == 409
}
