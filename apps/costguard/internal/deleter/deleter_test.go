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

package deleter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/fake"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const org = "00000000-0000-0000-0000-00000000000a"

var (
	ctx = context.Background()
	now = time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
)

func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testDeleter(st *fake.Store) *Deleter {
	return &Deleter{IaaS: st, Logger: logger(), MaxAttempts: 3, ServerWait: time.Second}
}

func store() *fake.Store {
	st := fake.New()
	st.AddProject("p1", "proj", org, now.AddDate(0, 0, -1), nil)
	return st
}

func scanStore(t *testing.T, st *fake.Store, mod func(*config.Config)) *scanner.Result {
	t.Helper()
	cfg := config.Config{OrganizationID: org, Regions: []string{"eu01"}, WarnEmptyAfterDays: 30}
	if mod != nil {
		mod(&cfg)
	}
	s := &scanner.Scanner{Clients: st.Set(), Config: cfg, Logger: logger(), Pacer: scanner.NopPacer{}, Now: func() time.Time { return now }, Workers: 2}
	res, err := s.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func res(kind stackit.Kind, id string, mods ...func(*stackit.Resource)) stackit.Resource {
	r := stackit.Resource{Kind: kind, ID: id, Name: id, ProjectID: "p1", Region: "eu01"}
	for _, m := range mods {
		m(&r)
	}
	return r
}

func labelled(kv ...string) func(*stackit.Resource) {
	return func(r *stackit.Resource) {
		r.Labels = map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			r.Labels[kv[i]] = kv[i+1]
		}
	}
}

var del = labelled("delete", "true")

func detached(r *stackit.Resource) { r.Status = stackit.VolumeStatusAvailable }

func on(server string) func(*stackit.Resource) {
	return func(r *stackit.Resource) { r.Status, r.ServerID = "IN-USE", server }
}

func outcomes(sum *report.DeletionSummary) map[string]report.Status {
	out := map[string]report.Status{}
	for _, r := range sum.Results {
		out[r.Item.ID] = r.Status
	}
	return out
}

func TestDeleteWaitsForServersSoTheirVolumesAndIPsGoToo(t *testing.T) {
	st := store()
	st.DeletingReads = 2
	st.Add(
		res(stackit.KindServer, "s1", del),
		res(stackit.KindVolume, "v1", del, on("s1")),
		res(stackit.KindNIC, "n1", on("s1")),
		res(stackit.KindPublicIP, "ip1", del, func(r *stackit.Resource) { r.NICID, r.Address = "n1", "192.0.2.1" }),
		res(stackit.KindPublicIP, "ip2", del, func(r *stackit.Resource) { r.Address = "192.0.2.2" }),
		res(stackit.KindSnapshot, "sn1", del),
		res(stackit.KindImage, "i1", del),
		res(stackit.KindSecurityGroup, "sg1", del),
	)
	result := scanStore(t, st, nil)
	sum := testDeleter(st).Delete(ctx, result, now)

	want := map[string]report.Status{
		"s1": report.StatusDeleted, "v1": report.StatusDeleted, "ip1": report.StatusDeleted, "ip2": report.StatusDeleted,
		"sn1": report.StatusDeleted, "i1": report.StatusDeleted, "sg1": report.StatusDeleted,
	}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	if len(st.Resources) != 0 {
		t.Errorf("left over: %+v", st.Resources)
	}
	deletes := st.CallsWith("delete ")
	if deletes[0] != "delete server s1" || deletes[len(deletes)-1] != "delete securitygroup sg1" {
		t.Errorf("order = %v", deletes)
	}
	if sum.Scope != "whole organization" || !sum.GeneratedAt.Equal(now) {
		t.Errorf("summary header = %+v", sum)
	}
	for _, r := range sum.Results {
		if r.Item.ProjectName != "proj" {
			t.Errorf("item lacks context: %+v", r.Item)
		}
	}
}

func TestDeleteDefersWhenServerIsSlow(t *testing.T) {
	st := store()
	st.DeletingReads = 1000
	st.Add(res(stackit.KindServer, "s1", del), res(stackit.KindVolume, "v1", del, on("s1")))
	result := scanStore(t, st, nil)
	d := testDeleter(st)
	d.ServerWait, d.PollInterval = 5*time.Millisecond, time.Millisecond
	sum := d.Delete(ctx, result, now)
	if got := outcomes(sum); got["s1"] != report.StatusDeleted || got["v1"] != report.StatusDeferred {
		t.Errorf("outcomes = %v", got)
	}
	if v := st.Find("v1"); v == nil || !stackit.Requested(v.Labels) {
		t.Error("a deferred volume keeps its label")
	}
}

func TestDeleteKeepsLabelsWhenTheServerDeleteFails(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindServer, "s1", del),
		res(stackit.KindVolume, "v1", del, on("s1")),
		res(stackit.KindNIC, "n1", on("s1")),
		res(stackit.KindPublicIP, "ip1", del, func(r *stackit.Resource) { r.NICID, r.Address = "n1", "192.0.2.1" }),
	)
	result := scanStore(t, st, nil)
	st.Errs["delete:s1"] = fake.Status(500)

	sum := testDeleter(st).Delete(ctx, result, now)
	want := map[string]report.Status{"s1": report.StatusFailed, "v1": report.StatusDeferred, "ip1": report.StatusDeferred}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	if !stackit.Requested(st.Find("v1").Labels) || !stackit.Requested(st.Find("ip1").Labels) {
		t.Error("the disk and IP must keep their labels and go together with the server next run")
	}

	// Next run: the server goes, and so do the disk and IP.
	delete(st.Errs, "delete:s1")
	sum = testDeleter(st).Delete(ctx, scanStore(t, st, nil), now)
	want = map[string]report.Status{"s1": report.StatusDeleted, "v1": report.StatusDeleted, "ip1": report.StatusDeleted}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("next run outcomes = %v", got)
	}
}

func TestDeleteSurvivesReadHiccups(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindServer, "s1", del),
		res(stackit.KindServer, "s2"),
		res(stackit.KindVolume, "v-hiccup", del, detached),
		res(stackit.KindServer, "s3"),
		res(stackit.KindVolume, "v-server-unreadable", del, detached),
	)
	result := scanStore(t, st, nil)
	// Both volumes were unused at the scan and are attached before the
	// delete run reaches them, so it has to check their servers.
	on("s2")(st.Find("v-hiccup"))
	on("s3")(st.Find("v-server-unreadable"))
	st.ErrsOnce["get:s1"] = fake.Status(503) // the re-read before deleting
	st.ErrsOnce["get:s2"] = fake.Status(429) // the check of an attached volume's server
	st.Errs["get:s3"] = fake.Status(500)     // a server that stays unreadable

	sum := testDeleter(st).Delete(ctx, result, now)
	got := outcomes(sum)
	if got["s1"] != report.StatusDeleted {
		t.Errorf("one 503 on the re-read must not cost a week: %v", got)
	}
	// v-hiccup: its server stays (read on the second try), so the label goes.
	if got["v-hiccup"] != report.StatusUnflagged {
		t.Errorf("v-hiccup = %v", got["v-hiccup"])
	}
	// v-server-unreadable: nothing is known about the server, so the label stays.
	if got["v-server-unreadable"] != report.StatusDeferred || !stackit.Requested(st.Find("v-server-unreadable").Labels) {
		t.Errorf("an unreadable server must keep the volume's label: %v", got)
	}
	for _, r := range sum.Results {
		if r.Item.ID == "v-server-unreadable" && !strings.Contains(r.Reason, "could not be checked (HTTP 500)") {
			t.Errorf("reason = %q", r.Reason)
		}
	}
}

func TestDeleteRechecksBeforeDeleting(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindServer, "unlabelled", del),
		res(stackit.KindServer, "protected", del),
		res(stackit.KindServer, "gone", del),
		res(stackit.KindServer, "unreadable", del),
		res(stackit.KindVolume, "reattached", del, detached),
		res(stackit.KindServer, "s9"),
		res(stackit.KindPublicIP, "ip-attached", del, func(r *stackit.Resource) { r.Address = "192.0.2.9" }),
	)
	result := scanStore(t, st, nil)

	// Things change between the scan and the delete run.
	delete(st.Find("unlabelled").Labels, "delete")
	st.Find("protected").Labels["do-not-delete"] = "true"
	st.Resources = removeID(st.Resources, "gone")
	st.Errs["get:unreadable"] = fake.Status(500)
	on("s9")(st.Find("reattached"))
	st.Find("ip-attached").NICID = "n9"

	sum := testDeleter(st).Delete(ctx, result, now)
	want := map[string]report.Status{
		"unlabelled":  report.StatusSkipped,
		"protected":   report.StatusSkipped,
		"gone":        report.StatusDeleted,
		"unreadable":  report.StatusFailed,
		"reattached":  report.StatusUnflagged,
		"ip-attached": report.StatusUnflagged,
	}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	if got := st.CallsWith("delete "); len(got) != 0 {
		t.Errorf("nothing may be deleted: %v", got)
	}
	for _, r := range sum.Results {
		if r.AlreadyGone != (r.Item.ID == "gone") {
			t.Errorf("%s: AlreadyGone = %v", r.Item.ID, r.AlreadyGone)
		}
	}
	if stackit.Requested(st.Find("reattached").Labels) || stackit.Requested(st.Find("ip-attached").Labels) {
		t.Error("labels of re-used resources must be removed")
	}
}

func removeID(rs []*stackit.Resource, id string) []*stackit.Resource {
	var out []*stackit.Resource
	for _, r := range rs {
		if r.ID != id {
			out = append(out, r)
		}
	}
	return out
}

func TestDeleteUnflagsIPsUsedByLoadBalancers(t *testing.T) {
	st := store()
	st.Add(res(stackit.KindPublicIP, "ip1", del, func(r *stackit.Resource) { r.Address = "192.0.2.1" }))
	result := scanStore(t, st, nil)
	// A load balancer took the address after the scan listed them.
	result.Context.LoadBalancerAddresses["p1/eu01"] = map[string]string{"192.0.2.1": "network load balancer web"}
	sum := testDeleter(st).Delete(ctx, result, now)
	if len(sum.Results) != 1 || sum.Results[0].Status != report.StatusUnflagged || sum.Results[0].Reason != "used by network load balancer web" {
		t.Errorf("results = %+v", sum.Results)
	}
}

func TestDeleteUnflagsBackInUse(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindServer, "s1"),
		res(stackit.KindVolume, "v-used", del, on("s1")),
		res(stackit.KindVolume, "v-free-again", del, on("s1")),
		res(stackit.KindVolume, "v-gone", del, on("s1")),
		res(stackit.KindVolume, "v-cleared", del, on("s1")),
		res(stackit.KindVolume, "v-broken", del, on("s1")),
		res(stackit.KindVolume, "v-nolabel", del, on("s1")),
	)
	result := scanStore(t, st, nil)
	detached(st.Find("v-free-again"))
	st.Find("v-free-again").ServerID = ""
	st.Resources = removeID(st.Resources, "v-gone")
	delete(st.Find("v-cleared").Labels, "delete")
	st.Errs["get:v-broken"] = fake.Status(500)
	st.Errs["label:v-nolabel"] = fake.Status(500)

	sum := testDeleter(st).Delete(ctx, result, now)
	want := map[string]report.Status{
		"v-used":       report.StatusUnflagged,
		"v-free-again": report.StatusDeferred,
		"v-gone":       report.StatusSkipped,
		"v-cleared":    report.StatusSkipped,
		"v-broken":     report.StatusFailed,
		"v-nolabel":    report.StatusFailed,
	}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	for _, r := range sum.Results {
		if r.Item.ID == "v-used" && r.Reason != "attached to a server" {
			t.Errorf("reason = %q", r.Reason)
		}
	}
}

func TestDeleteErrors(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindSecurityGroup, "sg-in-use", del),
		res(stackit.KindImage, "i-flaky", del),
		res(stackit.KindSnapshot, "sn-forbidden", del),
		res(stackit.KindServer, "s-vanished", del),
	)
	result := scanStore(t, st, nil)
	st.Errs["delete:sg-in-use"] = fake.Status(409)
	st.Errs["delete:i-flaky"] = fake.Status(503)
	st.Errs["delete:sn-forbidden"] = fake.Status(403)
	st.Errs["delete:s-vanished"] = fake.Status(404)

	d := testDeleter(st)
	d.ServerWait = 10 * time.Millisecond // s-vanished never really goes away in the fake
	sum := d.Delete(ctx, result, now)
	want := map[string]report.Status{
		"sg-in-use":    report.StatusFailed,
		"i-flaky":      report.StatusFailed,
		"sn-forbidden": report.StatusFailed,
		"s-vanished":   report.StatusDeleted,
	}
	if got := outcomes(sum); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	if n := len(st.CallsWith("delete image i-flaky")); n != 3 {
		t.Errorf("transient errors are retried: %d attempts", n)
	}
	if n := len(st.CallsWith("delete securitygroup")); n != 1 {
		t.Errorf("conflicts are not retried: %d attempts", n)
	}
	for _, r := range sum.Results {
		if r.Item.ID == "sg-in-use" && !strings.Contains(r.Reason, "still in use") {
			t.Errorf("reason = %q", r.Reason)
		}
	}
}

func TestDeleteDoesNothingWhenBlocked(t *testing.T) {
	st := store()
	st.Add(res(stackit.KindServer, "s1", del))
	result := scanStore(t, st, func(c *config.Config) { c.Skip.Projects = []string{"missing"} })
	sum := testDeleter(st).Delete(ctx, result, now)
	if len(sum.Results) != 0 || len(sum.Blocked) != 1 || !sum.HasNews() {
		t.Errorf("summary = %+v", sum)
	}
	if len(st.CallsWith("delete ")) != 0 || len(st.CallsWith("get ")) != 0 {
		t.Error("a blocked run must not touch anything")
	}
}

func TestDeleteStopsOnCancelledContext(t *testing.T) {
	st := store()
	st.Add(res(stackit.KindServer, "s1"), res(stackit.KindVolume, "v1", del, on("s1")), res(stackit.KindServer, "s2", del))
	result := scanStore(t, st, nil)
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	sum := testDeleter(st).Delete(cctx, result, now)
	if len(sum.Results) != 0 || len(st.CallsWith("delete ")) != 0 {
		t.Errorf("results = %+v", sum.Results)
	}
	if !sum.Interrupted || !sum.HasNews() {
		t.Error("an interrupted run must say so")
	}
}

func TestFlag(t *testing.T) {
	st := store()
	st.Add(
		res(stackit.KindServer, "s1"),
		res(stackit.KindVolume, "v-new", detached),
		res(stackit.KindVolume, "v-broken", detached),
		res(stackit.KindPublicIP, "ip-new", func(r *stackit.Resource) { r.Address = "192.0.2.1" }),
		res(stackit.KindVolume, "v-used", del, on("s1")),
		res(stackit.KindVolume, "v-used-broken", del, on("s1")),
		res(stackit.KindVolume, "v-kept", labelled("delete", "true", "do-not-delete", "true"), detached),
		res(stackit.KindPublicIP, "ip-kept-broken", labelled("delete", "true", "do-not-delete", "true")),
	)
	result := scanStore(t, st, nil)
	st.Errs["label:v-broken"] = fake.Status(500)
	st.Errs["label:v-used-broken"] = fake.Status(500)
	st.Errs["label:ip-kept-broken"] = fake.Status(500)

	got := testDeleter(st).Flag(ctx, result)
	if got.Flagged != 2 || got.Unflagged != 1 || got.Cleared != 1 || !got.Failed() {
		t.Errorf("flag result = %+v", got)
	}
	if len(got.NotFlagged) != 1 || got.NotFlagged[0].ID != "v-broken" || got.NotFlagged[0].ProjectName != "proj" ||
		!strings.Contains(got.NotFlagged[0].Detail, "HTTP 500") {
		t.Errorf("not flagged = %+v", got.NotFlagged)
	}
	if len(got.NotCleared) != 2 || got.NotCleared[0].ID != "v-used-broken" || got.NotCleared[1].ID != "ip-kept-broken" {
		t.Errorf("not cleared = %+v", got.NotCleared)
	}
	if got.NotCleared[0].Detail != "HTTP 500" {
		t.Errorf("the message detail must be replaced by the reason: %q", got.NotCleared[0].Detail)
	}
	kept := st.Find("v-kept").Labels
	if stackit.Requested(kept) || !stackit.Protected(kept) {
		t.Errorf("the stale delete label must go, do-not-delete must stay: %v", kept)
	}
	if !stackit.Requested(st.Find("v-new").Labels) || !stackit.Requested(st.Find("ip-new").Labels) {
		t.Error("new candidates must be labelled")
	}
	if stackit.Requested(st.Find("v-used").Labels) {
		t.Error("the label of a volume in use must be removed")
	}
}

func TestWriteErrorNamesStops(t *testing.T) {
	for _, err := range []error{context.Canceled, fmt.Errorf("x: %w", context.DeadlineExceeded)} {
		if got := writeError(err); got != "the run was stopped before this label was written" {
			t.Errorf("%v: %q", err, got)
		}
	}
	if got := writeError(fake.Status(403)); got != "HTTP 403" {
		t.Errorf("got %q", got)
	}
	if (FlagResult{}).Failed() {
		t.Error("an empty result has no failures")
	}
}

func TestRetryAndSleepHelpers(t *testing.T) {
	d := New(nil, logger())
	if d.MaxAttempts != DefaultMaxAttempts || d.ServerWait != DefaultServerWait {
		t.Errorf("defaults = %+v", d)
	}
	d.RetryBackoff = time.Hour
	cctx, cancel := context.WithCancel(ctx)
	calls := 0
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	err := d.withRetry(cctx, func(context.Context) error { calls++; return fake.Status(500) })
	if err == nil || calls != 1 {
		t.Errorf("err=%v calls=%d", err, calls)
	}
	d.sleep(ctx, 0)
}
