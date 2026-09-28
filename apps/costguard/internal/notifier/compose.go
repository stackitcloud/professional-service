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
	"fmt"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// Mode is the kind of run a message is about.
type Mode string

// Run modes.
const (
	ModeReport Mode = "report"
	ModeFlag   Mode = "flag"
	ModeDelete Mode = "delete"
)

// kindNames are the display names of item kinds.
var kindNames = map[string]string{
	"server":               "server",
	"volume":               "volume",
	"publicip":             "public IP",
	"snapshot":             "snapshot",
	"image":                "image",
	"nic":                  "network interface",
	"securitygroup":        "security group",
	report.KindProject:     "project",
	report.KindNetworkArea: "network area",
}

// Composer writes the messages.
type Composer struct {
	PortalURL string
	// DeleteRunAt is when the delete run happens, e.g. "Tuesday 08:00".
	DeleteRunAt string
	// Prices value what a run deletes.
	Prices             report.Prices
	WarnEmptyAfterDays int
	Version            string
}

const keepHint = "To keep a resource, open it and set the label do-not-delete=true (or remove delete=true)."

// Report composes the message of a report (stage 1) or flag (stage 2
// Monday) run.
func (c Composer) Report(rep *report.Report, mode Mode) Message {
	n := rep.ToDelete()
	// What the next delete run would delete, and what that saves.
	var toDelete []report.Item
	for _, list := range [][]report.Item{rep.IdlePublicIPs, rep.DetachedVolumes, rep.Requested} {
		toDelete = append(toDelete, list...)
	}
	saving := savingPhrase(report.Estimate(toDelete, c.Prices))

	m := Message{Subtitle: subtitle(rep.Scope, rep.Regions, rep.GeneratedAt)}
	switch {
	case mode == ModeReport:
		m.Title = "costguard report (stage 1)"
		m.Intro = fmt.Sprintf("Stage 1 only reports; nothing was changed. In stage 2, %d resource(s) would be deleted", n)
		if saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += "."
	case len(rep.Blocked) > 0:
		m.Title = "costguard: deletions blocked"
		m.Intro = fmt.Sprintf("Once the skip list is fixed, %d resource(s) would be deleted", n)
		if saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += "."
	case n == 0:
		m.Title = "costguard: nothing to delete"
	default:
		m.Title = fmt.Sprintf("costguard: %d resource(s) will be deleted %s", n, c.DeleteRunAt)
		if saving != "" {
			m.Intro = fmt.Sprintf("Deleting these %d resource(s) saves %s. They are labelled delete=true. ", n, saving)
		} else {
			m.Intro = fmt.Sprintf("These %d resource(s) are labelled delete=true. ", n)
		}
		m.Intro += keepHint
	}

	if len(rep.Blocked) > 0 {
		if mode == ModeReport {
			m.Alerts = append(m.Alerts, "These skip entries match nothing inside the scope (or it could not be read), which would block all deletions in stage 2: "+
				strings.Join(rep.Blocked, ", ")+". Skip entries must point to folders or projects inside the scope.")
		} else {
			m.Alerts = append(m.Alerts, "Nothing is flagged or deleted until the skip list is fixed. These entries match nothing inside the scope (or it could not be read): "+
				strings.Join(rep.Blocked, ", ")+". Skip entries must point to folders or projects inside the scope.")
		}
	}
	m.Alerts = append(m.Alerts, scanErrorAlert(rep.ScanErrors)...)

	ips := report.Estimate(rep.IdlePublicIPs, c.Prices)
	vols := report.Estimate(rep.DetachedVolumes, c.Prices)
	// Everything that will be deleted or unflagged is listed in full; the
	// scanner already capped the new candidates. Report-only lists show
	// report.ListLimit entries.
	c.add(&m, fmt.Sprintf("Idle public IPs: %d, about %s/month", ips.IdleIPs, eur(ips.TotalEUR())), rep.IdlePublicIPs, 0, rep.WaitingIdlePublicIPs)
	c.add(&m, fmt.Sprintf("Detached volumes: %d, %d GB, about %s/month", vols.Volumes, vols.VolumesGB, eur(vols.TotalEUR())), rep.DetachedVolumes, 0, rep.WaitingDetachedVolumes)
	c.add(&m, fmt.Sprintf("Other resources labelled delete=true: %d", len(rep.Requested)), rep.Requested, 0, 0)
	c.add(&m, fmt.Sprintf("Detached volumes with snapshots, not flagged: %d", len(rep.WithSnapshots)), rep.WithSnapshots, report.ListLimit, 0)
	backInUse := "In use again, delete label removed"
	if mode == ModeReport {
		backInUse = "In use again, stage 2 would remove the delete label"
	}
	c.add(&m, fmt.Sprintf("%s: %d", backInUse, len(rep.BackInUse)), rep.BackInUse, 0, 0)
	c.add(&m, fmt.Sprintf("Protected by do-not-delete but still marked delete=true: %d", len(rep.ProtectedMarked)),
		protectedMarkedItems(rep.ProtectedMarked, mode), 0, 0)
	c.add(&m, fmt.Sprintf("Empty projects older than %d days (warning only): %d", c.WarnEmptyAfterDays, len(rep.EmptyProjects)), rep.EmptyProjects, report.ListLimit, 0)
	c.add(&m, fmt.Sprintf("Empty network areas older than %d days (warning only): %d", c.WarnEmptyAfterDays, len(rep.EmptyNetworkAreas)), rep.EmptyNetworkAreas, report.ListLimit, 0)

	if len(m.Sections) == 0 && m.Intro == "" {
		m.Intro = "Nothing found."
	} else if len(m.Sections) == 0 {
		m.Intro += " Nothing found."
	}
	addScanErrors(&m, rep.ScanErrors)
	m.Footer = fmt.Sprintf("Skipped: %d folder(s), %d project(s) · costguard %s", rep.SkippedFolders, rep.SkippedProjects, c.Version)
	return m
}

// Summary composes the message of a delete run.
func (c Composer) Summary(sum *report.DeletionSummary) Message {
	deleted := sum.Count(report.StatusDeleted)
	m := Message{
		Title:    fmt.Sprintf("costguard: %d resource(s) deleted", deleted),
		Subtitle: subtitle(sum.Scope, nil, sum.GeneratedAt),
		Footer:   "costguard " + c.Version,
	}
	if len(sum.Blocked) > 0 {
		m.Title = "costguard: deletions blocked"
		m.Alerts = append(m.Alerts, "Nothing was deleted: these skip entries match nothing inside the scope (or it could not be read): "+
			strings.Join(sum.Blocked, ", ")+". Fix the skip list; entries must point to folders or projects inside the scope.")
	}
	if sum.Interrupted {
		m.Alerts = append(m.Alerts, fmt.Sprintf("This run was interrupted after %d deletion(s). Everything listed below is what happened before that; the rest keeps its label and follows in the next run.",
			len(sum.DeletedByThisRun())))
	}
	m.Alerts = append(m.Alerts, scanErrorAlert(sum.ScanErrors)...)
	for _, g := range []struct {
		status report.Status
		title  string
	}{
		{report.StatusDeleted, "Deleted"},
		{report.StatusFailed, "Failed, tried again next run"},
		{report.StatusUnflagged, "In use again, delete label removed"},
		{report.StatusDeferred, "Left for the next run"},
		{report.StatusSkipped, "Skipped"},
	} {
		results := sum.ByStatus(g.status)
		items := make([]report.Item, 0, len(results))
		for _, r := range results {
			it := r.Item
			if r.Reason != "" {
				it.Detail = r.Reason
			}
			items = append(items, it)
		}
		c.add(&m, fmt.Sprintf("%s: %d", g.title, len(items)), items, 0, 0)
	}
	if byRun := sum.DeletedByThisRun(); len(byRun) > 0 {
		m.Intro = fmt.Sprintf("This run deleted %d resource(s)", len(byRun))
		if saving := savingPhrase(report.Estimate(byRun, c.Prices)); saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += "."
	}
	if len(m.Sections) == 0 && len(m.Alerts) == 0 {
		m.Intro = "Nothing happened."
	}
	addScanErrors(&m, sum.ScanErrors)
	return m
}

// savingPhrase renders "about €19.28 per month (€231.36 per year)", or ""
// when nothing has a price.
func savingPhrase(s report.Savings) string {
	total := s.TotalEUR()
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("about %s per month (%s per year)", eur(total), eur(total*12))
}

// FlagProblems composes the follow-up of a flag run whose label writes
// failed after its message went out: the message needs a correction.
func (c Composer) FlagProblems(notFlagged, notCleared []report.Item) Message {
	m := Message{
		Title:  "costguard: correction to today's message",
		Intro:  "Some labels could not be written after the message went out. costguard tries again next Monday.",
		Footer: "costguard " + c.Version,
	}
	c.add(&m, fmt.Sprintf("Not marked, so NOT deleted this week: %d", len(notFlagged)), notFlagged, 0, 0)
	c.add(&m, fmt.Sprintf("delete label could not be removed (they are not deleted either way): %d", len(notCleared)), notCleared, 0, 0)
	return m
}

// Failure composes the message of a run that stopped before changing
// anything.
// Multi-line errors (the configuration and scope checks list one problem
// per line) become the alert plus a list of the problems.
func (c Composer) Failure(mode Mode, err error) Message {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	m := Message{
		Title:  fmt.Sprintf("costguard %s run failed", mode),
		Alerts: []string{strings.TrimSuffix(strings.TrimSpace(lines[0]), ":")},
		Intro:  "The run stopped before changing anything.",
		Footer: "costguard " + c.Version,
	}
	if len(lines) > 1 {
		sec := Section{Title: "Details"}
		for _, l := range lines[1:] {
			if l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- ")); l != "" {
				sec.Lines = append(sec.Lines, Line{Text: l})
			}
		}
		m.Sections = append(m.Sections, sec)
	}
	return m
}

// add appends a section. limit > 0 lists at most that many entries;
// waiting counts entries the caller already left out. Either way the
// section says how many more there are.
func (c Composer) add(m *Message, title string, items []report.Item, limit, waiting int) {
	if len(items) == 0 && waiting == 0 {
		return
	}
	sec := Section{Title: title, Omitted: waiting}
	for i, it := range items {
		if limit > 0 && i == limit {
			sec.Omitted += len(items) - limit
			break
		}
		sec.Lines = append(sec.Lines, Line{Text: lineText(it), Link: PortalLink(c.PortalURL, it)})
	}
	m.Sections = append(m.Sections, sec)
}

// protectedMarkedItems says per entry what happens to its stale delete
// label: costguard removes it from volumes and public IPs (in stage 2), the
// rest needs a person.
func protectedMarkedItems(items []report.Item, mode Mode) []report.Item {
	out := make([]report.Item, 0, len(items))
	for _, it := range items {
		switch {
		case it.Kind != "volume" && it.Kind != "publicip":
			it.Detail = "remove the delete label by hand; it is never deleted while do-not-delete is set"
		case mode == ModeReport:
			it.Detail = "stage 2 removes the delete label"
		default:
			it.Detail = "delete label removed"
		}
		out = append(out, it)
	}
	return out
}

// lineText renders "volume data · shop (eu01) · 50 GB · new".
func lineText(it report.Item) string {
	name := it.Name
	if name == "" {
		name = it.ID
	}
	parts := []string{strings.TrimSpace(kindNames[it.Kind] + " " + name)}
	if it.ProjectName != "" && it.Kind != report.KindProject {
		where := it.ProjectName
		if it.Region != "" {
			where += " (" + it.Region + ")"
		}
		parts = append(parts, where)
	}
	if it.Detail != "" {
		parts = append(parts, it.Detail)
	}
	if it.New {
		parts = append(parts, "new")
	}
	return strings.Join(parts, " · ")
}

func subtitle(scope string, regions []string, at time.Time) string {
	parts := []string{"Scope: " + scope}
	if len(regions) > 0 {
		parts = append(parts, "regions "+strings.Join(regions, ", "))
	}
	parts = append(parts, at.UTC().Format("2006-01-02 15:04 UTC"))
	return strings.Join(parts, " · ")
}

func scanErrorAlert(errs []string) []string {
	if len(errs) == 0 {
		return nil
	}
	return []string{"Some things could not be read, so this may be incomplete. Nothing that could not be read is flagged or deleted. Details at the end."}
}

// addScanErrors lists every cause of a scan error once (the scanner groups
// them), at the end of the message.
func addScanErrors(m *Message, errs []string) {
	if len(errs) == 0 {
		return
	}
	sec := Section{Title: fmt.Sprintf("Could not be read: %d", len(errs))}
	for _, e := range errs {
		sec.Lines = append(sec.Lines, Line{Text: e})
	}
	m.Sections = append(m.Sections, sec)
}

func eur(v float64) string {
	return fmt.Sprintf("€%.2f", v)
}
