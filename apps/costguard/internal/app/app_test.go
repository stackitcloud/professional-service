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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/deleter"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/fake"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/scanner"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

const org = "00000000-0000-0000-0000-00000000000a"

var monday = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)

// webhook records the messages it receives.
type webhook struct {
	mu     sync.Mutex
	bodies []string
	status int
}

func (w *webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bodies = append(w.bodies, string(b))
	rw.WriteHeader(w.status)
}

func (w *webhook) messages() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.bodies...)
}

type env struct {
	store   *fake.Store
	hook    *webhook
	config  string
	logs    *bytes.Buffer
	clients error
}

// setup wires the fake STACKIT, a webhook and a config file into Run.
func setup(t *testing.T, yaml string) *env {
	t.Helper()
	e := &env{store: fake.New(), hook: &webhook{status: http.StatusOK}, logs: &bytes.Buffer{}}
	e.store.AddProject("p1", "shop", org, monday.AddDate(0, 0, -1), nil)

	ts := httptest.NewServer(e.hook)
	t.Cleanup(ts.Close)
	t.Setenv(config.EnvWebhookURL, ts.URL)

	e.config = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(e.config, []byte("organizationId: "+org+"\noutput: slack\nregions: [eu01]\n"+yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	restore := []func(){}
	oldClients, oldScanner, oldDeleter, oldNow, oldLog := newClients, newScanner, newDeleter, now, logOutput
	restore = append(restore, func() {
		newClients, newScanner, newDeleter, now, logOutput = oldClients, oldScanner, oldDeleter, oldNow, oldLog
	})
	t.Cleanup(func() {
		for _, f := range restore {
			f()
		}
	})
	newClients = func() (*stackit.Set, error) {
		if e.clients != nil {
			return nil, e.clients
		}
		return e.store.Set(), nil
	}
	newScanner = func(set *stackit.Set, cfg config.Config, logger *slog.Logger) *scanner.Scanner {
		s := scanner.New(set, cfg, logger)
		s.Pacer, s.Now = scanner.NopPacer{}, func() time.Time { return monday }
		return s
	}
	newDeleter = func(iaas stackit.IaaS, logger *slog.Logger) *deleter.Deleter {
		d := deleter.New(iaas, logger)
		d.BetweenDeletions, d.RetryBackoff, d.ServerWait, d.PollInterval = 0, 0, 10*time.Millisecond, time.Millisecond
		return d
	}
	now = func() time.Time { return monday.AddDate(0, 0, 1) }
	logOutput = e.logs
	oldPauses := notifier.RetryPauses
	notifier.RetryPauses = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { notifier.RetryPauses = oldPauses })
	return e
}

func (e *env) run(sub string) int {
	return e.runCtx(context.Background(), sub)
}

func (e *env) runCtx(ctx context.Context, sub string) int {
	return Run(ctx, Options{Subcommand: sub, ConfigPath: e.config, LogLevel: "debug", Version: "test"})
}

func (e *env) addCandidates() {
	e.store.Add(
		stackit.Resource{Kind: stackit.KindVolume, ID: "v1", Name: "data", ProjectID: "p1", Region: "eu01", Status: stackit.VolumeStatusAvailable, SizeGB: 10},
		stackit.Resource{Kind: stackit.KindPublicIP, ID: "ip1", Name: "192.0.2.1", Address: "192.0.2.1", ProjectID: "p1", Region: "eu01"},
	)
}

func TestReportChangesNothing(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	if code := e.run("report"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard report (stage 1)") || !strings.Contains(msgs[0], "2 resource(s) would be deleted, saving about €5.44 per month") {
		t.Errorf("messages = %v", msgs)
	}
	if calls := append(e.store.CallsWith("label"), e.store.CallsWith("delete")...); len(calls) != 0 {
		t.Errorf("report must not write: %v", calls)
	}
	if !strings.Contains(e.logs.String(), `"category":"detached-volume"`) {
		t.Errorf("findings must be logged:\n%s", e.logs)
	}
}

func TestFlagPostsThenLabels(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	if code := e.run("flag"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "2 resource(s) will be deleted Tuesday 08:00") {
		t.Errorf("messages = %v", msgs)
	}
	if !stackit.Requested(e.store.Find("v1").Labels) || !stackit.Requested(e.store.Find("ip1").Labels) {
		t.Error("candidates must be labelled")
	}
}

func TestFlagOnlyFlagsWhatTheMessageLists(t *testing.T) {
	e := setup(t, "")
	for i := 0; i < 12; i++ {
		e.store.Add(stackit.Resource{Kind: stackit.KindVolume, ID: fmt.Sprintf("v%02d", i), ProjectID: "p1", Region: "eu01",
			Status: stackit.VolumeStatusAvailable, SizeGB: int64(10 + i)})
	}
	if code := e.run("flag"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "… and 2 more; they will be listed in the next runs.") {
		t.Errorf("messages = %v", msgs)
	}
	flagged := 0
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("v%02d", i)
		if stackit.Requested(e.store.Find(id).Labels) {
			flagged++
			if !strings.Contains(msgs[0], id) {
				t.Errorf("%s is flagged but not in the message", id)
			}
		}
	}
	if flagged != 10 {
		t.Errorf("flagged %d, want 10", flagged)
	}
}

func TestFlagClearsStaleDeleteLabelOnProtectedDisk(t *testing.T) {
	e := setup(t, "")
	e.store.Add(stackit.Resource{Kind: stackit.KindVolume, ID: "v1", Name: "data", ProjectID: "p1", Region: "eu01",
		Status: stackit.VolumeStatusAvailable, SizeGB: 10, Labels: map[string]string{"delete": "true", "do-not-delete": "true"}})

	if code := e.run("report"); code != ExitOK || len(e.store.CallsWith("label")) != 0 {
		t.Fatalf("report must not write (exit %d, calls %v)", code, e.store.CallsWith("label"))
	}
	if code := e.run("flag"); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	labels := e.store.Find("v1").Labels
	if stackit.Requested(labels) || !stackit.Protected(labels) {
		t.Errorf("labels = %v", labels)
	}
	msgs := e.hook.messages()
	if len(msgs) != 2 || !strings.Contains(msgs[0], "stage 2 removes the delete label") ||
		!strings.Contains(msgs[1], "Protected by do-not-delete but still marked delete=true: 1") ||
		!strings.Contains(msgs[1], "delete label removed") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestFlagWithoutDeliveredMessageSetsNoLabels(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	e.hook.status = http.StatusInternalServerError
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if calls := e.store.CallsWith("label"); len(calls) != 0 {
		t.Errorf("no message, no labels: %v", calls)
	}
	if n := len(e.hook.messages()); n != 3 {
		t.Errorf("a 500 is tried 3 times, got %d", n)
	}
}

func TestFlagBlockedSetsNoLabels(t *testing.T) {
	e := setup(t, "skip:\n  projects: [renamed]\n")
	e.addCandidates()
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Nothing is flagged or deleted until the skip list is fixed") ||
		!strings.Contains(msgs[0], "costguard: deletions blocked") {
		t.Errorf("messages = %v", msgs)
	}
	if calls := e.store.CallsWith("label"); len(calls) != 0 {
		t.Errorf("blocked runs set no labels: %v", calls)
	}
}

func TestFlagLabelFailureIsFatal(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	e.store.Errs["label:v1"] = fake.Status(500)
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
}

func TestDeleteDeletesLabelledAndPosts(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	if code := e.run("flag"); code != ExitOK {
		t.Fatal("flag failed")
	}
	if code := e.run("delete"); code != ExitOK {
		t.Fatalf("exit %d\n%s", code, e.logs)
	}
	if e.store.Find("v1") != nil || e.store.Find("ip1") != nil {
		t.Error("labelled candidates must be deleted")
	}
	msgs := e.hook.messages()
	// 10 GB at the default 0.0619 €/GB plus one IP at 4.82 €.
	if len(msgs) != 2 || !strings.Contains(msgs[1], "costguard: 2 resource(s) deleted") ||
		!strings.Contains(msgs[1], "This run deleted 2 resource(s), saving about €5.44 per month (€65.28 per year).") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestInterruptedDeleteRunStillPosts(t *testing.T) {
	e := setup(t, "")
	for _, id := range []string{"s1", "s2", "s3"} {
		e.store.Add(stackit.Resource{Kind: stackit.KindServer, ID: id, ProjectID: "p1", Region: "eu01", Labels: map[string]string{"delete": "true"}})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// SIGTERM arrives right after the first deletion.
	e.store.AfterDelete = func(stackit.Resource) { cancel() }

	if code := e.runCtx(ctx, "delete"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "This run was interrupted after 1 deletion(s).") {
		t.Errorf("the summary must still go out: %v", msgs)
	}
	if len(e.store.CallsWith("delete server")) != 1 {
		t.Errorf("deleting must stop when told to: %v", e.store.CallsWith("delete server"))
	}
}

func TestCancelledRunStillPostsFailure(t *testing.T) {
	e := setup(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := e.runCtx(ctx, "flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "costguard flag run failed") ||
		!strings.Contains(msgs[0], "stopped from outside") || strings.Contains(msgs[0], "context canceled") {
		t.Errorf("the failure message must go out, in words people understand: %v", msgs)
	}
}

func TestTimedOutRunSaysSo(t *testing.T) {
	e := setup(t, "")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if code := e.runCtx(ctx, "report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "took longer than 60 minutes") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestFlagPostsACorrectionWhenLabelsFail(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	e.store.Errs["label:v1"] = fake.Status(500)
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 2 || !strings.Contains(msgs[1], "correction to today") ||
		!strings.Contains(msgs[1], "Not marked, so NOT deleted this week: 1") || !strings.Contains(msgs[1], "data") {
		t.Errorf("messages = %v", msgs)
	}
	if !stackit.Requested(e.store.Find("ip1").Labels) {
		t.Error("the other label must still be written")
	}
}

func TestDeleteWithNothingToDoStaysQuiet(t *testing.T) {
	e := setup(t, "")
	e.addCandidates() // unlabelled: Tuesday never flags
	if code := e.run("delete"); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if msgs := e.hook.messages(); len(msgs) != 0 {
		t.Errorf("no news, no message: %v", msgs)
	}
	if calls := e.store.CallsWith("costs"); len(calls) != 0 {
		t.Errorf("the delete run skips the cost call: %v", calls)
	}
	if len(e.store.CallsWith("label")) != 0 {
		t.Error("the delete run never flags")
	}
}

func TestDeleteBlockedPostsAndFails(t *testing.T) {
	e := setup(t, "skip:\n  folders: [gone]\n")
	e.store.Add(stackit.Resource{Kind: stackit.KindServer, ID: "s1", ProjectID: "p1", Region: "eu01", Labels: map[string]string{"delete": "true"}})
	if code := e.run("delete"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if e.store.Find("s1") == nil {
		t.Error("a blocked run deletes nothing")
	}
	if msgs := e.hook.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], "deletions blocked") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestDeleteSummaryDeliveryFailureIsFatal(t *testing.T) {
	e := setup(t, "")
	e.store.Add(stackit.Resource{Kind: stackit.KindServer, ID: "s1", ProjectID: "p1", Region: "eu01", Labels: map[string]string{"delete": "true"}})
	e.hook.status = http.StatusForbidden
	if code := e.run("delete"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
}

func TestScopeErrorPostsFailure(t *testing.T) {
	e := setup(t, "scope:\n  projects: [no-such-project]\n")
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard flag run failed") || !strings.Contains(msgs[0], "no-such-project") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestClientErrorPostsFailure(t *testing.T) {
	e := setup(t, "")
	e.clients = errors.New("no credentials")
	e.hook.status = http.StatusBadGateway
	if code := e.run("report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(e.logs.String(), "sending the failure message failed") {
		t.Errorf("logs = %s", e.logs)
	}
}

func TestLoginProblemsAreReadable(t *testing.T) {
	e := setup(t, "")
	e.clients = errors.New("setting up authentication: no valid credentials were found")
	if code := e.run("report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard cannot log in to STACKIT") ||
		!strings.Contains(msgs[0], "no valid credentials were found") {
		t.Errorf("messages = %v", msgs)
	}
}

func TestUnreadableOrganizationIsNotNothingFound(t *testing.T) {
	e := setup(t, "")
	e.addCandidates()
	e.store.Errs["projects:"+org] = fake.Status(401)
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "cannot read the organization") || strings.Contains(msgs[0], "Nothing found") {
		t.Errorf("messages = %v", msgs)
	}
	if len(e.store.CallsWith("label")) != 0 {
		t.Error("nothing may be flagged")
	}
}

func TestReportDeliveryFailureIsFatal(t *testing.T) {
	e := setup(t, "")
	e.hook.status = http.StatusNotFound
	if code := e.run("report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
}

func TestUsageAndConfigErrors(t *testing.T) {
	e := setup(t, "")
	if code := e.run("scan"); code != ExitUsage {
		t.Errorf("unknown subcommand: exit %d", code)
	}
	if code := Run(context.Background(), Options{Subcommand: "report", ConfigPath: filepath.Join(t.TempDir(), "missing")}); code != ExitFatal {
		t.Errorf("missing config: exit %d", code)
	}
	t.Setenv(config.EnvWebhookURL, "")
	if code := e.run("report"); code != ExitFatal {
		t.Errorf("invalid config: exit %d", code)
	}
}

func TestBrokenConfigIsPostedToTheChat(t *testing.T) {
	// A typo in a key: the strict parser refuses the file, the chat
	// settings are still usable.
	e := setup(t, "skips:\n  projects: [a]\n")
	if code := e.run("report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs := e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "costguard report run failed") || !strings.Contains(msgs[0], "skips") {
		t.Errorf("messages = %v", msgs)
	}

	// A value that fails validation: each problem is listed.
	e = setup(t, "warnEmptyAfterDays: 1\n")
	if code := e.run("flag"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	msgs = e.hook.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "warnEmptyAfterDays must be 0 (warnings off) or at least 7") {
		t.Errorf("messages = %v", msgs)
	}
	if len(e.store.CallsWith("list")) != 0 {
		t.Error("nothing may be scanned with a broken configuration")
	}
}

func TestBrokenChatSettingsOnlyLog(t *testing.T) {
	e := setup(t, "")
	t.Setenv(config.EnvWebhookURL, "http://hooks.example.com/not-https")
	if code := e.run("report"); code != ExitFatal {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(e.logs.String(), "the problem cannot be posted") {
		t.Errorf("logs = %s", e.logs)
	}
}

func TestBuildNotifierAndLogger(t *testing.T) {
	for _, out := range []string{config.OutputSlack, config.OutputTeams, config.OutputGoogleChat} {
		if buildNotifier(config.Config{Output: out, WebhookURL: "https://x"}) == nil {
			t.Errorf("%s: nil notifier", out)
		}
	}
	for _, lvl := range []string{"debug", "warn", "error", ""} {
		if newLogger(lvl) == nil {
			t.Errorf("%q: nil logger", lvl)
		}
	}
}
