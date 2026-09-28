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

package notifier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

var at = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)

func composer() Composer {
	return Composer{PortalURL: "https://portal.example/", DeleteRunAt: "Tuesday 08:00", WarnEmptyAfterDays: 30, Version: "v0.1.0",
		Prices: report.Prices{PublicIPMonthlyEUR: 4.82, VolumeGBMonthlyEUR: 0.062}}
}

func fullReport() *report.Report {
	return &report.Report{
		GeneratedAt: at, Scope: "whole organization", Regions: []string{"eu01"},
		SkippedFolders: 1, SkippedProjects: 4,
		IdlePublicIPs:     []report.Item{{Kind: "publicip", ID: "ip1", Name: "192.0.2.1", ProjectID: "p1", ProjectName: "shop", Region: "eu01", New: true}},
		DetachedVolumes:   []report.Item{{Kind: "volume", ID: "v1", Name: "data", ProjectID: "p1", ProjectName: "shop", Region: "eu01", Detail: "50 GB", SizeGB: 50}},
		Requested:         []report.Item{{Kind: "server", ID: "s1", Name: "web", ProjectID: "p1", ProjectName: "shop", Region: "eu01"}},
		WithSnapshots:     []report.Item{{Kind: "volume", ID: "v2", Name: "db", ProjectID: "p1", ProjectName: "shop", Region: "eu01", Detail: "10 GB, 1 snapshot(s)"}},
		BackInUse:         []report.Item{{Kind: "volume", ID: "v3", ProjectID: "p1", ProjectName: "shop", Region: "eu01"}},
		EmptyProjects:     []report.Item{{Kind: report.KindProject, ID: "p9", Name: "old", ProjectID: "p9", ProjectName: "old", Detail: "created 2026-01-01"}},
		EmptyNetworkAreas: []report.Item{{Kind: report.KindNetworkArea, ID: "a1", Name: "hub"}},
	}
}

func titles(m Message) string {
	var out []string
	for _, s := range m.Sections {
		out = append(out, s.Title)
	}
	return strings.Join(out, " | ")
}

func TestComposeFlagMessage(t *testing.T) {
	m := composer().Report(fullReport(), ModeFlag)
	if m.Title != "costguard: 3 resource(s) will be deleted Tuesday 08:00" {
		t.Errorf("title = %q", m.Title)
	}
	if m.Subtitle != "Scope: whole organization · regions eu01 · 2026-09-28 08:00 UTC" {
		t.Errorf("subtitle = %q", m.Subtitle)
	}
	wantIntro := "Deleting these 3 resource(s) saves about €7.92 per month (€95.04 per year). " +
		"They are labelled delete=true. To keep a resource, open it and set the label do-not-delete=true (or remove delete=true)."
	if m.Intro != wantIntro {
		t.Errorf("intro = %q", m.Intro)
	}
	want := "Idle public IPs: 1, about €4.82/month | Detached volumes: 1, 50 GB, about €3.10/month | " +
		"Other resources labelled delete=true: 1 | Detached volumes with snapshots, not flagged: 1 | " +
		"In use again, delete label removed: 1 | Empty projects older than 30 days (warning only): 1 | " +
		"Empty network areas older than 30 days (warning only): 1"
	if got := titles(m); got != want {
		t.Errorf("sections =\n%s\nwant\n%s", got, want)
	}
	ip := m.Sections[0].Lines[0]
	if ip.Text != "public IP 192.0.2.1 · shop (eu01) · new" || ip.Link != "https://portal.example/projects/p1" {
		t.Errorf("ip line = %+v", ip)
	}
	if got := m.Sections[4].Lines[0].Text; got != "volume v3 · shop (eu01)" {
		t.Errorf("unnamed items use their ID: %q", got)
	}
	if got := m.Sections[5].Lines[0]; got.Text != "project old · created 2026-01-01" || got.Link != "https://portal.example/projects/p9" {
		t.Errorf("project line = %+v", got)
	}
	if got := m.Sections[6].Lines[0].Link; got != "" {
		t.Errorf("network areas have no link yet: %q", got)
	}
	if m.Footer != "Skipped: 1 folder(s), 4 project(s) · costguard v0.1.0" {
		t.Errorf("footer = %q", m.Footer)
	}
	if len(m.Alerts) != 0 {
		t.Errorf("alerts = %v", m.Alerts)
	}
}

func TestComposeReportModeAndBlockedAndErrors(t *testing.T) {
	rep := fullReport()
	rep.Blocked = []string{`skip.projects: "gone"`}
	rep.ScanErrors = []string{"e1", "e2", "e3", "e4", "e5"}
	m := composer().Report(rep, ModeReport)
	if m.Title != "costguard report (stage 1)" || !strings.HasSuffix(m.Intro, "3 resource(s) would be deleted, saving about €7.92 per month (€95.04 per year).") {
		t.Errorf("title/intro = %q / %q", m.Title, m.Intro)
	}
	if len(m.Alerts) != 2 || !strings.Contains(m.Alerts[0], "would block all deletions in stage 2") ||
		!strings.Contains(m.Alerts[0], "match nothing inside the scope (or it could not be read)") ||
		!strings.Contains(m.Alerts[0], "Skip entries must point to folders or projects inside the scope") {
		t.Errorf("alerts = %v", m.Alerts)
	}
	if !strings.HasPrefix(m.Alerts[1], "Some things could not be read") || !strings.Contains(m.Alerts[1], "Details at the end") {
		t.Errorf("scan error alert = %q", m.Alerts[1])
	}
	// Every grouped cause is listed, as the last section, none left out.
	last := m.Sections[len(m.Sections)-1]
	if last.Title != "Could not be read: 5" || len(last.Lines) != 5 || last.Omitted != 0 || last.Lines[4].Text != "e5" {
		t.Errorf("scan error section = %+v", last)
	}
	if !strings.Contains(titles(m), "stage 2 would remove the delete label") {
		t.Errorf("sections = %s", titles(m))
	}

	m = composer().Report(rep, ModeFlag)
	if !strings.Contains(m.Alerts[0], "Nothing is flagged or deleted until the skip list is fixed") {
		t.Errorf("alert = %q", m.Alerts[0])
	}
	// A blocked flag run must not announce deletions.
	if m.Title != "costguard: deletions blocked" ||
		m.Intro != "Once the skip list is fixed, 3 resource(s) would be deleted, saving about €7.92 per month (€95.04 per year)." {
		t.Errorf("blocked title/intro = %q / %q", m.Title, m.Intro)
	}
	rep.IdlePublicIPs, rep.DetachedVolumes = nil, nil
	if got := composer().Report(rep, ModeFlag).Intro; got != "Once the skip list is fixed, 1 resource(s) would be deleted." {
		t.Errorf("blocked intro without prices = %q", got)
	}
}

func TestComposeNothingFound(t *testing.T) {
	rep := &report.Report{GeneratedAt: at, Scope: "Teams"}
	m := composer().Report(rep, ModeFlag)
	if m.Title != "costguard: nothing to delete" || m.Intro != "Nothing found." || len(m.Sections) != 0 {
		t.Errorf("message = %+v", m)
	}
	m = composer().Report(rep, ModeReport)
	if !strings.HasSuffix(m.Intro, "0 resource(s) would be deleted. Nothing found.") {
		t.Errorf("intro = %q", m.Intro)
	}
}

func TestComposeListsEverythingActedOn(t *testing.T) {
	rep := &report.Report{GeneratedAt: at, WaitingIdlePublicIPs: 5}
	for i := 0; i < 27; i++ {
		rep.Requested = append(rep.Requested, report.Item{Kind: "image", ID: fmt.Sprint(i)})
		rep.BackInUse = append(rep.BackInUse, report.Item{Kind: "volume", ID: fmt.Sprint(i)})
	}
	for i := 0; i < 15; i++ {
		rep.WithSnapshots = append(rep.WithSnapshots, report.Item{Kind: "volume", ID: fmt.Sprint(i)})
		rep.EmptyProjects = append(rep.EmptyProjects, report.Item{Kind: report.KindProject, ID: fmt.Sprint(i)})
	}
	rep.IdlePublicIPs = []report.Item{{Kind: "publicip", ID: "ip1"}}
	m := composer().Report(rep, ModeFlag)

	counts := map[string][2]int{}
	for _, s := range m.Sections {
		counts[strings.SplitN(s.Title, ":", 2)[0]] = [2]int{len(s.Lines), s.Omitted}
	}
	want := map[string][2]int{
		"Idle public IPs":                                  {1, 5},  // listed + waiting new candidates
		"Other resources labelled delete=true":             {27, 0}, // always in full
		"In use again, delete label removed":               {27, 0}, // always in full
		"Detached volumes with snapshots, not flagged":     {10, 5}, // report-only: capped
		"Empty projects older than 30 days (warning only)": {10, 5},
	}
	for title, w := range want {
		if counts[title] != w {
			t.Errorf("%s: lines/omitted = %v, want %v", title, counts[title], w)
		}
	}
	if OmittedText(7) != "… and 7 more; they will be listed in the next runs." {
		t.Errorf("omitted text = %q", OmittedText(7))
	}
	// Waiting candidates alone still get a section.
	if m := composer().Report(&report.Report{GeneratedAt: at, WaitingDetachedVolumes: 3}, ModeFlag); len(m.Sections) != 1 || m.Sections[0].Omitted != 3 {
		t.Errorf("sections = %+v", m.Sections)
	}
}

func TestComposeSummary(t *testing.T) {
	sum := &report.DeletionSummary{GeneratedAt: at, Scope: "whole organization", Results: []report.Result{
		{Item: report.Item{Kind: "server", ID: "s1", Name: "web", ProjectName: "shop", Region: "eu01"}, Status: report.StatusDeleted},
		{Item: report.Item{Kind: "securitygroup", ID: "g1", Name: "default", ProjectName: "shop", Region: "eu01"}, Status: report.StatusFailed, Reason: "still in use"},
		{Item: report.Item{Kind: "volume", ID: "v1", Name: "data"}, Status: report.StatusUnflagged, Reason: "attached to a server again"},
		{Item: report.Item{Kind: "volume", ID: "v2", Name: "logs"}, Status: report.StatusDeferred, Reason: "next run"},
		{Item: report.Item{Kind: "image", ID: "i1", Name: "old"}, Status: report.StatusSkipped, Reason: "do-not-delete is set"},
	}}
	m := composer().Summary(sum)
	if m.Title != "costguard: 1 resource(s) deleted" || m.Subtitle != "Scope: whole organization · 2026-09-28 08:00 UTC" {
		t.Errorf("title/subtitle = %q / %q", m.Title, m.Subtitle)
	}
	want := "Deleted: 1 | Failed, tried again next run: 1 | In use again, delete label removed: 1 | Left for the next run: 1 | Skipped: 1"
	if got := titles(m); got != want {
		t.Errorf("sections = %s", got)
	}
	if got := m.Sections[1].Lines[0].Text; got != "security group default · shop (eu01) · still in use" {
		t.Errorf("failed line = %q", got)
	}
	if m.Intro != "This run deleted 1 resource(s)." {
		t.Errorf("nothing priced was deleted, so no amount: %q", m.Intro)
	}

	sum.Results = append(sum.Results,
		report.Result{Item: report.Item{Kind: "publicip", ID: "ip1"}, Status: report.StatusDeleted},
		report.Result{Item: report.Item{Kind: "volume", ID: "v9", SizeGB: 100}, Status: report.StatusDeleted},
		report.Result{Item: report.Item{Kind: "volume", ID: "v8", SizeGB: 500}, Status: report.StatusDeleted, Reason: "already gone", AlreadyGone: true},
		report.Result{Item: report.Item{Kind: "volume", ID: "v7", SizeGB: 500}, Status: report.StatusFailed},
	)
	if got := composer().Summary(sum).Intro; got != "This run deleted 3 resource(s), saving about €11.02 per month (€132.24 per year)." {
		t.Errorf("saving = %q", got)
	}

	sum.Interrupted = true
	if m := composer().Summary(sum); !strings.HasPrefix(m.Alerts[0], "This run was interrupted after 3 deletion(s).") {
		t.Errorf("interrupted alert = %v", m.Alerts)
	}

	blocked := composer().Summary(&report.DeletionSummary{GeneratedAt: at, Blocked: []string{"x"}, ScanErrors: []string{"e"}})
	if blocked.Title != "costguard: deletions blocked" || len(blocked.Alerts) != 2 || blocked.Intro != "" {
		t.Errorf("blocked = %+v", blocked)
	}
	if empty := composer().Summary(&report.DeletionSummary{GeneratedAt: at}); empty.Intro != "Nothing happened." {
		t.Errorf("empty = %+v", empty)
	}
}

func TestComposeProtectedButMarked(t *testing.T) {
	rep := &report.Report{GeneratedAt: at, ProtectedMarked: []report.Item{
		{Kind: "volume", ID: "v1", Name: "data"},
		{Kind: "server", ID: "s1", Name: "web"},
	}}
	find := func(m Message) Section {
		for _, s := range m.Sections {
			if strings.HasPrefix(s.Title, "Protected by do-not-delete but still marked delete=true: 2") {
				return s
			}
		}
		t.Fatalf("section missing: %+v", m.Sections)
		return Section{}
	}
	flag := find(composer().Report(rep, ModeFlag))
	if flag.Lines[0].Text != "volume data · delete label removed" ||
		flag.Lines[1].Text != "server web · remove the delete label by hand; it is never deleted while do-not-delete is set" {
		t.Errorf("flag lines = %+v", flag.Lines)
	}
	if got := find(composer().Report(rep, ModeReport)).Lines[0].Text; got != "volume data · stage 2 removes the delete label" {
		t.Errorf("report line = %q", got)
	}
	if rep.ProtectedMarked[0].Detail != "" {
		t.Error("composing must not change the report")
	}
}

func TestComposeFlagProblems(t *testing.T) {
	m := composer().FlagProblems(
		[]report.Item{{Kind: "volume", ID: "v1", Name: "data", ProjectID: "p1", ProjectName: "shop", Region: "eu01", Detail: "HTTP 500"}},
		[]report.Item{{Kind: "publicip", ID: "ip1", Name: "192.0.2.1", ProjectID: "p1", ProjectName: "shop", Region: "eu01", Detail: "HTTP 403"}},
	)
	if m.Title != "costguard: correction to today's message" || !strings.Contains(m.Intro, "tries again next Monday") {
		t.Errorf("header = %q / %q", m.Title, m.Intro)
	}
	if got := titles(m); got != "Not marked, so NOT deleted this week: 1 | delete label could not be removed (they are not deleted either way): 1" {
		t.Errorf("sections = %s", got)
	}
	if got := m.Sections[0].Lines[0].Text; got != "volume data · shop (eu01) · HTTP 500" {
		t.Errorf("line = %q", got)
	}
	if only := composer().FlagProblems(nil, []report.Item{{Kind: "volume", ID: "v2"}}); len(only.Sections) != 1 {
		t.Errorf("empty groups must be left out: %+v", only.Sections)
	}
}

func TestComposeFailure(t *testing.T) {
	m := composer().Failure(ModeDelete, errors.New("the scope cannot be resolved"))
	if m.Title != "costguard delete run failed" || m.Alerts[0] != "the scope cannot be resolved" || !strings.Contains(m.Intro, "before changing anything") {
		t.Errorf("failure = %+v", m)
	}
	if len(m.Sections) != 0 {
		t.Errorf("a one-line error needs no details: %+v", m.Sections)
	}

	// Multi-line errors: the first line is the alert, the problems a list.
	m = composer().Failure(ModeReport, errors.New("invalid configuration:\n  - output is required\n  - region \"eu-1\" is not a STACKIT region ID\n"))
	if m.Alerts[0] != "invalid configuration" || len(m.Sections) != 1 || m.Sections[0].Title != "Details" {
		t.Fatalf("failure = %+v", m)
	}
	if got := m.Sections[0].Lines; len(got) != 2 || got[0].Text != "output is required" || got[1].Text != `region "eu-1" is not a STACKIT region ID` {
		t.Errorf("details = %+v", got)
	}
}

func TestPortalLink(t *testing.T) {
	it := report.Item{Kind: "nic", ID: "n1", ProjectID: "p 1", NetworkID: "net1", Region: "eu01"}
	if got := PortalLink("https://p/", it); got != "https://p/projects/p%201" {
		t.Errorf("link = %q", got)
	}
	if PortalLink("https://p", report.Item{Kind: report.KindNetworkArea, ID: "a1"}) != "" {
		t.Error("items without a project have no link")
	}
}

// scripted answers with the given statuses in turn and counts requests.
type scripted struct {
	statuses []int
	calls    int
	body     string
}

func (s *scripted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.body = r.Header.Get("Content-Type") + " " + string(b)
	status := s.statuses[len(s.statuses)-1]
	if s.calls < len(s.statuses) {
		status = s.statuses[s.calls]
	}
	s.calls++
	w.WriteHeader(status)
	_, _ = w.Write([]byte("invalid_blocks\n"))
}

func fastRetries(t *testing.T, pauses ...time.Duration) {
	t.Helper()
	old := RetryPauses
	RetryPauses = pauses
	t.Cleanup(func() { RetryPauses = old })
}

func TestPosterRetriesTransientErrors(t *testing.T) {
	fastRetries(t, time.Millisecond, time.Millisecond)
	for name, c := range map[string]struct {
		statuses []int
		calls    int
		ok       bool
	}{
		"503, 429, then ok":       {[]int{503, 429, 200}, 3, true},
		"ok at once":              {[]int{200}, 1, true},
		"400 is not retried":      {[]int{400}, 1, false},
		"404 is not retried":      {[]int{404}, 1, false},
		"three 500s give up":      {[]int{500}, 3, false},
		"202 counts as delivered": {[]int{202}, 1, true},
	} {
		srv := &scripted{statuses: c.statuses}
		ts := httptest.NewServer(srv)
		err := NewPoster(ts.URL, "Slack").Post(context.Background(), map[string]string{"a": "b"})
		ts.Close()
		if (err == nil) != c.ok || srv.calls != c.calls {
			t.Errorf("%s: err=%v calls=%d", name, err, srv.calls)
		}
		if c.ok && srv.body != `application/json {"a":"b"}` {
			t.Errorf("%s: request = %q", name, srv.body)
		}
	}

	srv := &scripted{statuses: []int{400}}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	err := NewPoster(ts.URL, "Slack").Post(context.Background(), map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request: invalid_blocks") {
		t.Errorf("err = %v", err)
	}
}

func TestPosterNetworkErrorsAndLimits(t *testing.T) {
	fastRetries(t, time.Millisecond, time.Millisecond)
	ts := httptest.NewServer(http.NotFoundHandler())
	p := NewPoster(ts.URL+"/secret-token", "Slack")
	ts.Close()
	err := p.Post(context.Background(), map[string]string{})
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("connection errors must not leak the webhook URL: %v", err)
	}
	if err := p.Post(context.Background(), func() {}); err == nil {
		t.Error("unencodable payload must fail")
	}
	if err := NewPoster("http://in valid", "Teams").Post(context.Background(), 1); err == nil {
		t.Error("bad URL must fail")
	}

	// A cancelled run does not sit out the pauses.
	fastRetries(t, time.Hour, time.Hour)
	srv := &scripted{statuses: []int{503}}
	ts = httptest.NewServer(srv)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := NewPoster(ts.URL, "Teams").Post(ctx, 1); err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("err=%v after %v", err, time.Since(start))
	}
}
