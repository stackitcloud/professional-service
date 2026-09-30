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
		WithSnapshots:     []report.Item{{Kind: "volume", ID: "v2", Name: "db", ProjectID: "p1", ProjectName: "shop", Region: "eu01", Detail: "10 GB, 1 snapshot"}},
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
	if m.Title != "costguard: report" {
		t.Errorf("title = %q", m.Title)
	}
	if m.Subtitle != "Scope: whole organization · regions eu01 · 2026-09-28 08:00 UTC" {
		t.Errorf("subtitle = %q", m.Subtitle)
	}
	wantIntro := "3 resources labelled delete=true are deleted Tuesday 08:00, saving about €7,92 per month (€95,04 per year). " +
		"To keep a resource, open it and set the label do-not-delete=true (or remove delete=true)."
	if m.Intro != wantIntro {
		t.Errorf("intro = %q", m.Intro)
	}
	want := "Idle public IPs: 1, about €4,82/month | Detached volumes: 1, 50 GB, about €3,10/month | " +
		"Other resources labelled delete=true: 1 | Detached volumes with snapshots, not flagged: 1 | " +
		"In use again, delete label removed: 1 | Empty projects older than 30 days (warning only): 1 | " +
		"Empty network areas older than 30 days (warning only): 1"
	if got := titles(m); got != want {
		t.Errorf("sections =\n%s\nwant\n%s", got, want)
	}
	ip := m.Sections[0].Lines[0]
	if ip.Text != "public IP 192.0.2.1 · shop (eu01) · new" || ip.Link != "https://portal.example/public-ip/public-ips/ip1/overview?project=p1" {
		t.Errorf("ip line = %+v", ip)
	}
	if got := m.Sections[4].Lines[0].Text; got != "volume v3 · shop (eu01)" {
		t.Errorf("unnamed items use their ID: %q", got)
	}
	if got := m.Sections[5].Lines[0]; got.Text != "project old · created 2026-01-01" || got.Link != "https://portal.example/dashboard?project=p9" {
		t.Errorf("project line = %+v", got)
	}
	if got := m.Sections[6].Lines[0].Link; got != "" {
		t.Errorf("network areas have no link yet: %q", got)
	}
	if m.Footer != "Skipped: 1 folder, 4 projects · costguard v0.1.0" {
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
	wantIntro := "Automatic deletion is off; nothing was changed. With delete on, 3 resources would be deleted, " +
		"saving about €7,92 per month (€95,04 per year). To keep a resource once delete is on, set the label do-not-delete=true on it."
	if m.Title != "costguard: report" || m.Intro != wantIntro {
		t.Errorf("title/intro = %q / %q", m.Title, m.Intro)
	}
	if len(m.Alerts) != 2 || !strings.Contains(m.Alerts[0], "would block all deletions once delete is on") ||
		!strings.Contains(m.Alerts[0], "match nothing inside the scope (or it could not be read)") ||
		!strings.Contains(m.Alerts[0], "Skip entries must point to folders or projects inside the scope") {
		t.Errorf("alerts = %v", m.Alerts)
	}
	if !strings.HasPrefix(m.Alerts[1], "Some things could not be read") || !strings.Contains(m.Alerts[1], "Details at the end") {
		t.Errorf("scan error alert = %q", m.Alerts[1])
	}
	last := m.Sections[len(m.Sections)-1]
	if last.Title != "Could not be read: 5" || len(last.Lines) != 5 || last.Omitted != 0 || last.Lines[4].Text != "e5" {
		t.Errorf("scan error section = %+v", last)
	}
	if !strings.Contains(titles(m), "In use again; with delete on, the delete label would be removed: 1") {
		t.Errorf("sections = %s", titles(m))
	}

	m = composer().Report(rep, ModeFlag)
	if !strings.Contains(m.Alerts[0], "Nothing is flagged or deleted until the skip list is fixed") {
		t.Errorf("alert = %q", m.Alerts[0])
	}
	if m.Title != "costguard: report" ||
		m.Intro != "Once the skip list is fixed, 3 resources would be deleted, saving about €7,92 per month (€95,04 per year)." {
		t.Errorf("blocked title/intro = %q / %q", m.Title, m.Intro)
	}
	rep.IdlePublicIPs, rep.DetachedVolumes = nil, nil
	if got := composer().Report(rep, ModeFlag).Intro; got != "Once the skip list is fixed, 1 resource would be deleted." {
		t.Errorf("blocked intro without prices = %q", got)
	}
}

func TestComposeNothingFound(t *testing.T) {
	rep := &report.Report{GeneratedAt: at, Scope: "Teams"}
	m := composer().Report(rep, ModeFlag)
	if m.Title != "costguard: report" || m.Intro != "Nothing to delete." || len(m.Sections) != 0 {
		t.Errorf("message = %+v", m)
	}
	m = composer().Report(rep, ModeReport)
	if m.Intro != "Automatic deletion is off; nothing was changed. Nothing to clean up." {
		t.Errorf("intro = %q", m.Intro)
	}
	on := composer()
	on.DeleteEnabled = true
	if m := on.Report(rep, ModeReport); m.Intro != "This run changed nothing. Nothing to clean up." || len(m.Sections) != 0 {
		t.Errorf("delete on: %+v", m)
	}
}

func TestComposeBootMessage(t *testing.T) {
	c := composer()
	c.ReportRunAt, c.DeleteRunAt = "Monday 08:00 (Europe/Berlin)", ""
	m := c.Report(fullReport(), ModeBoot)
	if m.Title != "costguard: v0.1.0 is running" {
		t.Errorf("title = %q", m.Title)
	}
	wantIntro := "Login and this chat work. Automatic deletion is off; nothing was changed. With delete on, 3 resources would be deleted, " +
		"saving about €7,92 per month (€95,04 per year). To keep a resource once delete is on, set the label do-not-delete=true on it. " +
		"Next runs: report Monday 08:00 (Europe/Berlin)."
	if m.Intro != wantIntro {
		t.Errorf("intro =\n%s\nwant\n%s", m.Intro, wantIntro)
	}
	if !strings.HasPrefix(titles(m), "Idle public IPs: 1") {
		t.Errorf("sections = %s", titles(m))
	}
	if m := composer().Report(&report.Report{GeneratedAt: at}, ModeBoot); strings.Contains(m.Intro, "Next runs") {
		t.Errorf("intro = %q", m.Intro)
	}
}

func TestComposeReadOnlyWithDeleteOn(t *testing.T) {
	c := composer()
	c.DeleteEnabled, c.DeleteRunAt, c.ReportRunAt = true, "Tuesday 08:00 (Europe/Berlin)", "Monday 08:00 (Europe/Berlin)"
	rep := fullReport()
	rep.WaitingIdlePublicIPs = 2
	rep.Blocked = []string{`skip.projects: "gone"`}
	m := c.Report(rep, ModeBoot)
	wantIntro := "Login and this chat work. This run changed nothing. 2 resources labelled delete=true are deleted Tuesday 08:00 (Europe/Berlin), " +
		"saving about €3,10 per month (€37,20 per year). 3 new candidates are labelled at the next report run first. " +
		"To keep a resource, open it and set the label do-not-delete=true (or remove delete=true). " +
		"Next runs: report Monday 08:00 (Europe/Berlin) · delete Tuesday 08:00 (Europe/Berlin)."
	if m.Intro != wantIntro {
		t.Errorf("intro =\n%s\nwant\n%s", m.Intro, wantIntro)
	}
	want := "Labelled delete=true, deleted Tuesday 08:00 (Europe/Berlin), about €3,10/month: 2 | New candidates, labelled at the next report run: 3 | " +
		"Detached volumes with snapshots, not flagged: 1 | In use again, the delete label is removed at the next run: 1 | " +
		"Empty projects older than 30 days (warning only): 1 | Empty network areas older than 30 days (warning only): 1"
	if got := titles(m); got != want {
		t.Errorf("sections =\n%s\nwant\n%s", got, want)
	}
	if got := m.Sections[0].Lines; len(got) != 2 || got[0].Text != "volume data · shop (eu01) · 50 GB" || got[1].Text != "server web · shop (eu01)" {
		t.Errorf("labelled lines = %+v", got)
	}
	if got := m.Sections[1]; len(got.Lines) != 1 || got.Lines[0].Text != "public IP 192.0.2.1 · shop (eu01)" || got.Omitted != 2 {
		t.Errorf("new candidates = %+v", got)
	}
	if len(m.Alerts) != 1 || !strings.Contains(m.Alerts[0], "which blocks all flagging and deleting until the skip list is fixed") {
		t.Errorf("alerts = %v", m.Alerts)
	}
	if !rep.IdlePublicIPs[0].New {
		t.Error("composing must not change the report")
	}

	only := &report.Report{GeneratedAt: at, Requested: []report.Item{{Kind: "securitygroup", ID: "g1"}}}
	if got := c.Report(only, ModeReport).Intro; got != "This run changed nothing. 1 resource labelled delete=true is deleted Tuesday 08:00 (Europe/Berlin). "+keepHint {
		t.Errorf("intro = %q", got)
	}
	fresh := &report.Report{GeneratedAt: at, IdlePublicIPs: []report.Item{{Kind: "publicip", ID: "ip1", New: true}}}
	if got := c.Report(fresh, ModeReport); got.Title != "costguard: report" ||
		got.Intro != "This run changed nothing. 1 new candidate is labelled at the next report run first. "+keepHint {
		t.Errorf("message = %q / %q", got.Title, got.Intro)
	}
}

func TestComposeShowsTimesInTheConfiguredZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	c := composer()
	c.Location = berlin
	if got := c.Report(&report.Report{GeneratedAt: at, Scope: "whole organization"}, ModeReport).Subtitle; got != "Scope: whole organization · 2026-09-28 10:00 (Europe/Berlin)" {
		t.Errorf("subtitle = %q", got)
	}
	c.Location = time.UTC
	if got := c.Summary(&report.DeletionSummary{GeneratedAt: at, Scope: "x"}).Subtitle; got != "Scope: x · 2026-09-28 08:00 UTC" {
		t.Errorf("subtitle = %q", got)
	}
}

func TestSummaryIntroSaysWhatHappened(t *testing.T) {
	res := func(st report.Status) report.Result {
		return report.Result{Item: report.Item{Kind: "volume", ID: "v"}, Status: st}
	}
	for want, sum := range map[string]*report.DeletionSummary{
		"This run deleted 2 resources.":                                                           {Results: []report.Result{res(report.StatusDeleted), res(report.StatusDeleted), res(report.StatusFailed)}},
		"1 deletion failed; they are tried again in the next run.":                                {Interrupted: true, Results: []report.Result{res(report.StatusFailed), res(report.StatusUnflagged)}},
		"Nothing was deleted. The delete label was removed from 1 resource that is in use again.": {Results: []report.Result{res(report.StatusUnflagged)}},
		"Nothing was deleted.":                                                                    {Results: []report.Result{res(report.StatusDeferred)}},
		"":                                                                                        {Blocked: []string{"x"}, Results: []report.Result{res(report.StatusSkipped)}},
	} {
		sum.GeneratedAt = at
		if m := composer().Summary(sum); m.Title != "costguard: deletion" || m.Intro != want {
			t.Errorf("title/intro = %q / %q, want %q", m.Title, m.Intro, want)
		}
	}
}

func TestComposeListsEverythingActedOn(t *testing.T) {
	rep := &report.Report{GeneratedAt: at, WaitingIdlePublicIPs: 5}
	for i := 0; i < 27; i++ {
		rep.Requested = append(rep.Requested, report.Item{Kind: "securitygroup", ID: fmt.Sprint(i)})
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
		"Idle public IPs":                                  {1, 5},
		"Other resources labelled delete=true":             {27, 0},
		"In use again, delete label removed":               {27, 0},
		"Detached volumes with snapshots, not flagged":     {10, 5},
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
		{Item: report.Item{Kind: "securitygroup", ID: "g1", Name: "old"}, Status: report.StatusSkipped, Reason: "do-not-delete is set"},
	}}
	m := composer().Summary(sum)
	if m.Title != "costguard: deletion" || m.Subtitle != "Scope: whole organization · 2026-09-28 08:00 UTC" {
		t.Errorf("title/subtitle = %q / %q", m.Title, m.Subtitle)
	}
	want := "Deleted: 1 | Failed, will be tried again in the next run: 1 | In use again, delete label removed: 1 | Left for the next run: 1 | Skipped: 1"
	if got := titles(m); got != want {
		t.Errorf("sections = %s", got)
	}
	if got := m.Sections[1].Lines[0].Text; got != "security group default · shop (eu01) · still in use" {
		t.Errorf("failed line = %q", got)
	}
	if m.Intro != "This run deleted 1 resource." {
		t.Errorf("nothing priced was deleted, so no amount: %q", m.Intro)
	}

	sum.Results = append(sum.Results,
		report.Result{Item: report.Item{Kind: "publicip", ID: "ip1"}, Status: report.StatusDeleted},
		report.Result{Item: report.Item{Kind: "volume", ID: "v9", SizeGB: 100}, Status: report.StatusDeleted},
		report.Result{Item: report.Item{Kind: "volume", ID: "v8", SizeGB: 500}, Status: report.StatusDeleted, Reason: "already gone", AlreadyGone: true},
		report.Result{Item: report.Item{Kind: "volume", ID: "v7", SizeGB: 500}, Status: report.StatusFailed},
	)
	if got := composer().Summary(sum).Intro; got != "This run deleted 3 resources, saving about €11,02 per month (€132,24 per year)." {
		t.Errorf("saving = %q", got)
	}

	sum.Interrupted = true
	if m := composer().Summary(sum); !strings.HasPrefix(m.Alerts[0], "This run was interrupted after 3 deletions.") {
		t.Errorf("interrupted alert = %v", m.Alerts)
	}

	blocked := composer().Summary(&report.DeletionSummary{GeneratedAt: at, Blocked: []string{"x"}, ScanErrors: []string{"e"}})
	if blocked.Title != "costguard: deletion" || len(blocked.Alerts) != 2 || blocked.Intro != "" {
		t.Errorf("blocked = %+v", blocked)
	}
	if empty := composer().Summary(&report.DeletionSummary{GeneratedAt: at}); empty.Intro != "Nothing was deleted." {
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
	if got := find(composer().Report(rep, ModeReport)).Lines[0].Text; got != "volume data · with delete on, the delete label would be removed" {
		t.Errorf("report line = %q", got)
	}
	on := composer()
	on.DeleteEnabled = true
	if got := find(on.Report(rep, ModeBoot)).Lines[0].Text; got != "volume data · the delete label is removed at the next report run" {
		t.Errorf("boot line = %q", got)
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
	if m.Title != "costguard: correction" || !strings.Contains(m.Intro, "tries again at the next report run") {
		t.Errorf("header = %q / %q", m.Title, m.Intro)
	}
	if got := titles(m); got != "Not marked, so NOT deleted in the next delete run: 1 | delete label could not be removed (they are not deleted either way): 1" {
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
	if m.Title != "costguard: delete run failed" || m.Alerts[0] != "the scope cannot be resolved" || !strings.Contains(m.Intro, "before changing anything") {
		t.Errorf("failure = %+v", m)
	}
	if len(m.Sections) != 0 {
		t.Errorf("a one-line error needs no details: %+v", m.Sections)
	}

	m = composer().Failure(ModeReport, errors.New("invalid configuration:\n  - output is required\n  - region \"eu-1\" is not a STACKIT region ID\n"))
	if m.Alerts[0] != "invalid configuration" || len(m.Sections) != 1 || m.Sections[0].Title != "Details" {
		t.Fatalf("failure = %+v", m)
	}
	if got := m.Sections[0].Lines; len(got) != 2 || got[0].Text != "output is required" || got[1].Text != `region "eu-1" is not a STACKIT region ID` {
		t.Errorf("details = %+v", got)
	}
}

func TestOrganizationAndFolding(t *testing.T) {
	c := composer()
	c.Organization = "Acme"
	rep := fullReport()
	rep.ScanErrors = []string{"e1"}
	sum := &report.DeletionSummary{GeneratedAt: at, Scope: "whole organization", Results: []report.Result{
		{Item: report.Item{Kind: "volume", ID: "v1", Name: "data"}, Status: report.StatusDeleted},
	}}
	budgets, _ := c.Budgets(&report.BudgetCheck{Month: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Checked: at,
		Budgets:  []report.BudgetStatus{{Name: "Org", Target: "organization", LimitEUR: 10, Thresholds: []int{80}, MonthEUR: 9, Reached: 80}},
		Problems: []string{"gone"}})
	messages := map[string]Message{
		"report":  c.Report(rep, ModeReport),
		"boot":    c.Boot(nil, nil, nil, at),
		"summary": c.Summary(sum),
		"flag":    c.FlagProblems([]report.Item{{Kind: "volume", ID: "v1"}}, nil),
		"failure": c.Failure(ModeReport, errors.New("invalid configuration:\n- output is required")),
		"budgets": budgets,
	}
	wantTitles := map[string]string{
		"report":  "costguard: report (Acme)",
		"boot":    "costguard: v0.1.0 is running (Acme)",
		"summary": "costguard: deletion (Acme)",
		"flag":    "costguard: correction (Acme)",
		"failure": "costguard: report run failed (Acme)",
		"budgets": "costguard: budget (Acme)",
	}
	for name, m := range messages {
		if m.Title != wantTitles[name] {
			t.Errorf("%s: title = %q", name, m.Title)
		}
	}
	if got := messages["report"].Subtitle; got != "Scope: whole organization · regions eu01 · 2026-09-28 08:00 UTC" {
		t.Errorf("subtitle = %q", got)
	}
	if got := composer().Failure(ModeDelete, errors.New("x")).Title; got != "costguard: delete run failed" {
		t.Errorf("title without organization = %q", got)
	}

	for _, sec := range messages["report"].Sections {
		if !sec.Folded {
			t.Errorf("report section %q must fold", sec.Title)
		}
	}
	chk := &report.BudgetCheck{Month: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Checked: at,
		Budgets: []report.BudgetStatus{{Name: "Org", Target: "organization", LimitEUR: 10, Thresholds: []int{80}}}}
	boot := c.Boot(rep, chk, nil, at)
	if last := boot.Sections[len(boot.Sections)-1]; last.Title != "Could not be read: 1" || !strings.HasPrefix(boot.Sections[len(boot.Sections)-2].Title, "Budgets for September 2026") {
		t.Errorf("boot sections = %s", titles(boot))
	}
	for _, name := range []string{"summary", "flag"} {
		if sec := messages[name].Sections[0]; !sec.Folded {
			t.Errorf("%s section %q must fold", name, sec.Title)
		}
	}
	if sec := messages["failure"].Sections[0]; sec.Folded {
		t.Errorf("failure details must stay open: %+v", sec)
	}
	c.OrganizationID = "o-1"
	orgBudget := report.BudgetStatus{Name: "Org", Organization: true, LimitEUR: 10}
	if got := c.budgetLink(orgBudget); got != "https://portal.example/dashboard?organization=o-1" {
		t.Errorf("organization budget link = %q", got)
	}
	if got := c.budgetLink(report.BudgetStatus{Name: "x"}); got != "" {
		t.Errorf("unknown target link = %q", got)
	}
	if got := budgets.Sections; len(got) != 2 || got[0].Folded || got[0].Title != "" || !got[1].Folded || got[1].Title != "Budgets not checked: 1" {
		t.Errorf("budget sections = %+v", got)
	}
}

func TestPortalLink(t *testing.T) {
	const org = "o-1"
	item := func(kind, id string) report.Item {
		return report.Item{Kind: kind, ID: id, ProjectID: "p 1", Region: "eu01", NetworkID: "net1", VolumeID: "v1"}
	}
	for want, it := range map[string]report.Item{
		"https://p/server/servers/s1/overview?project=p+1":                  item("server", "s1"),
		"https://p/disk-volumes/volumes/v2/overview?project=p+1":            item("volume", "v2"),
		"https://p/public-ip/public-ips/ip1/overview?project=p+1":           item("publicip", "ip1"),
		"https://p/disk-volumes/volumes/v1/snapshots/sn1?project=p+1":       item("snapshot", "sn1"),
		"https://p/disk-volumes/volumes/v3/snapshots/sn9?project=p+1":       {Kind: "volume", ID: "v3", ProjectID: "p 1", SnapshotID: "sn9"},
		"https://p/nic/nics/n1/overview?project=p+1":                        item("nic", "n1"),
		"https://p/security-group/groups/g1/overview?project=p+1":           item("securitygroup", "g1"),
		"https://p/dashboard?project=p+1":                                   item(report.KindProject, "p 1"),
		"https://p/network-area/network-areas/a1/overview?organization=o-1": {Kind: report.KindNetworkArea, ID: "a1"},
	} {
		if got := PortalLink("https://p/", org, it); got != want {
			t.Errorf("%s: link = %q, want %q", it.Kind, got, want)
		}
	}
	noVolume := item("snapshot", "sn1")
	noVolume.VolumeID = ""
	if got := PortalLink("https://p", org, noVolume); got != "https://p/dashboard?project=p+1" {
		t.Errorf("snapshot without its volume = %q", got)
	}
	if PortalLink("https://p", "", report.Item{Kind: report.KindNetworkArea, ID: "a1"}) != "" || PortalLink("https://p", org, report.Item{Kind: "volume", ID: "v"}) != "" {
		t.Error("without the organization or project there is no link")
	}
}

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

func TestEuroFormat(t *testing.T) {
	for v, want := range map[float64]string{
		0:          "€0,00",
		0.004:      "€0,00",
		7.5:        "€7,50",
		999.999:    "€1.000,00",
		200000:     "€200.000,00",
		1234567.89: "€1.234.567,89",
		-2034.12:   "-€2.034,12",
		-0.001:     "€0,00",
	} {
		if got := eur(v); got != want {
			t.Errorf("eur(%v) = %q, want %q", v, got, want)
		}
	}
}
