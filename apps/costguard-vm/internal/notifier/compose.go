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

	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/report"
)

// Mode is the kind of run a message is about.
type Mode string

// Run modes.
const (
	ModeReport Mode = "report"
	ModeFlag   Mode = "flag"
	ModeDelete Mode = "delete"
	// ModeBoot is the read-only run right after the server was (re)created.
	ModeBoot Mode = "boot"
)

// Composer writes the messages.
type Composer struct {
	PortalURL string
	// DeleteEnabled words the read-only runs (report, boot): with delete on
	// they say what the next runs do, with delete off what they would do.
	DeleteEnabled bool
	// DeleteRunAt is when the delete run happens, e.g. "Tuesday 08:00
	// (Europe/Berlin)".
	DeleteRunAt string
	// ReportRunAt is when the report run happens, for the boot message's
	// "Next runs"; empty if not known.
	ReportRunAt string
	// Location is the time zone of the timestamps; nil means UTC.
	Location *time.Location
	// Prices value what a run deletes.
	Prices             report.Prices
	WarnEmptyAfterDays int
	Version            string
}

const (
	keepHint = "To keep a resource, open it and set the label do-not-delete=true (or remove delete=true)."
	// keepHintOff tells the readers of a report without delete how to keep
	// something before delete gets switched on.
	keepHintOff = "To keep a resource once delete is on, set the label do-not-delete=true on it."
)

// Report composes the message of a read-only run (report, boot) or a flag
// run.
func (c Composer) Report(rep *report.Report, mode Mode) Message {
	n := rep.ToDelete()
	saving := savingPhrase(report.Estimate(toDelete(rep), c.Prices))
	readOnly := mode == ModeReport || mode == ModeBoot

	m := Message{Subtitle: c.subtitle(rep.Scope, rep.Regions, rep.GeneratedAt)}
	switch {
	case readOnly:
		m.Title = "costguard report"
		if mode == ModeBoot {
			m.Title = "costguard-vm " + c.Version + " is running"
		}
		m.Intro = c.readOnlyIntro(rep, mode, n, saving)
	case len(rep.Blocked) > 0:
		m.Title = "costguard: deletions blocked"
		m.Intro = fmt.Sprintf("Once the skip list is fixed, %s would be deleted", report.Count(n, "resource"))
		if saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += "."
	case n == 0:
		m.Title = "costguard: nothing to delete"
	default:
		m.Title = fmt.Sprintf("costguard: %s will be deleted %s", report.Count(n, "resource"), c.DeleteRunAt)
		if saving != "" {
			m.Intro = fmt.Sprintf("Deleting these %s saves %s. They are labelled delete=true. ", report.Count(n, "resource"), saving)
		} else {
			m.Intro = fmt.Sprintf("These %s are labelled delete=true. ", report.Count(n, "resource"))
		}
		m.Intro += keepHint
	}

	if len(rep.Blocked) > 0 {
		blocked := strings.Join(rep.Blocked, ", ") + ". Skip entries must point to folders or projects inside the scope."
		switch {
		case readOnly && !c.DeleteEnabled:
			m.Alerts = append(m.Alerts, "These skip entries match nothing inside the scope (or it could not be read), which would block all deletions once delete is on: "+blocked)
		case readOnly:
			m.Alerts = append(m.Alerts, "These skip entries match nothing inside the scope (or it could not be read), which blocks all flagging and deleting until the skip list is fixed: "+blocked)
		default:
			m.Alerts = append(m.Alerts, "Nothing is flagged or deleted until the skip list is fixed. These entries match nothing inside the scope (or it could not be read): "+blocked)
		}
	}
	m.Alerts = append(m.Alerts, scanErrorAlert(rep.ScanErrors)...)

	if readOnly && c.DeleteEnabled {
		c.addUpcoming(&m, rep)
	} else {
		ips := report.Estimate(rep.IdlePublicIPs, c.Prices)
		vols := report.Estimate(rep.DetachedVolumes, c.Prices)
		// Everything that will be deleted or unflagged is listed in full;
		// the scanner already capped the new candidates. Report-only lists
		// show report.ListLimit entries.
		c.add(&m, fmt.Sprintf("Idle public IPs: %d, about %s/month", ips.IdleIPs, eur(ips.TotalEUR())), rep.IdlePublicIPs, 0, rep.WaitingIdlePublicIPs)
		c.add(&m, fmt.Sprintf("Detached volumes: %d, %d GB, about %s/month", vols.Volumes, vols.VolumesGB, eur(vols.TotalEUR())), rep.DetachedVolumes, 0, rep.WaitingDetachedVolumes)
		c.add(&m, fmt.Sprintf("Other resources labelled delete=true: %d", len(rep.Requested)), rep.Requested, 0, 0)
	}
	c.add(&m, fmt.Sprintf("Detached volumes with snapshots, not flagged: %d", len(rep.WithSnapshots)), rep.WithSnapshots, report.ListLimit, 0)
	backInUse := "In use again, delete label removed"
	switch {
	case readOnly && !c.DeleteEnabled:
		backInUse = "In use again; with delete on, the delete label would be removed"
	case readOnly:
		backInUse = "In use again, the delete label is removed at the next run"
	}
	c.add(&m, fmt.Sprintf("%s: %d", backInUse, len(rep.BackInUse)), rep.BackInUse, 0, 0)
	c.add(&m, fmt.Sprintf("Protected by do-not-delete but still marked delete=true: %d", len(rep.ProtectedMarked)),
		c.protectedMarkedItems(rep.ProtectedMarked, readOnly), 0, 0)
	c.add(&m, fmt.Sprintf("Empty projects older than %d days (warning only): %d", c.WarnEmptyAfterDays, len(rep.EmptyProjects)), rep.EmptyProjects, report.ListLimit, 0)
	c.add(&m, fmt.Sprintf("Empty network areas older than %d days (warning only): %d", c.WarnEmptyAfterDays, len(rep.EmptyNetworkAreas)), rep.EmptyNetworkAreas, report.ListLimit, 0)

	if !readOnly && len(m.Sections) == 0 {
		m.Intro = strings.TrimSpace(m.Intro + " Nothing to clean up.")
	}
	if mode == ModeBoot {
		if next := c.nextRuns(); next != "" {
			m.Intro += " " + next
		}
	}
	addScanErrors(&m, rep.ScanErrors)
	m.Footer = fmt.Sprintf("Skipped: %s, %s · costguard-vm %s",
		report.Count(rep.SkippedFolders, "folder"), report.Count(rep.SkippedProjects, "project"), c.Version)
	return m
}

// toDelete is what the next delete run would delete.
func toDelete(rep *report.Report) []report.Item {
	var out []report.Item
	for _, list := range [][]report.Item{rep.IdlePublicIPs, rep.DetachedVolumes, rep.Requested} {
		out = append(out, list...)
	}
	return out
}

// labelled and fresh split what the next delete run deletes (labelled
// now) from the new candidates (labelled by the next report run first).
func labelled(rep *report.Report) (labelled, fresh []report.Item) {
	for _, it := range append(append([]report.Item{}, rep.IdlePublicIPs...), rep.DetachedVolumes...) {
		if it.New {
			it.New = false
			fresh = append(fresh, it)
		} else {
			labelled = append(labelled, it)
		}
	}
	return append(labelled, rep.Requested...), fresh
}

// readOnlyIntro says that nothing changed and what delete does or would do.
func (c Composer) readOnlyIntro(rep *report.Report, mode Mode, n int, saving string) string {
	var b strings.Builder
	if mode == ModeBoot {
		b.WriteString("Login and this chat work. ")
	}
	if !c.DeleteEnabled {
		b.WriteString("Automatic deletion is off; nothing was changed.")
		if n == 0 {
			b.WriteString(" Nothing to clean up.")
			return b.String()
		}
		fmt.Fprintf(&b, " With delete on, %s would be deleted", report.Count(n, "resource"))
		if saving != "" {
			b.WriteString(", saving " + saving)
		}
		b.WriteString(". " + keepHintOff)
		return b.String()
	}
	b.WriteString("This run changed nothing.")
	now, fresh := labelled(rep)
	if len(now) == 0 && len(fresh) == 0 {
		b.WriteString(" Nothing to clean up.")
		return b.String()
	}
	if len(now) > 0 {
		fmt.Fprintf(&b, " %s labelled delete=true %s deleted %s", report.Count(len(now), "resource"), isAre(len(now)), c.DeleteRunAt)
		if s := savingPhrase(report.Estimate(now, c.Prices)); s != "" {
			b.WriteString(", saving " + s)
		}
		b.WriteString(".")
	}
	if waiting := len(fresh) + rep.WaitingIdlePublicIPs + rep.WaitingDetachedVolumes; waiting > 0 {
		fmt.Fprintf(&b, " %s %s labelled at the next report run first.", report.Count(waiting, "new candidate"), isAre(waiting))
	}
	b.WriteString(" " + keepHint)
	return b.String()
}

// addUpcoming lists, for a read-only run with delete on, what the next
// delete run deletes and what the next report run labels.
func (c Composer) addUpcoming(m *Message, rep *report.Report) {
	now, fresh := labelled(rep)
	title := "Labelled delete=true, deleted " + c.DeleteRunAt
	if s := report.Estimate(now, c.Prices).TotalEUR(); s > 0 {
		title += fmt.Sprintf(", about %s/month", eur(s))
	}
	c.add(m, fmt.Sprintf("%s: %d", title, len(now)), now, 0, 0)
	waiting := rep.WaitingIdlePublicIPs + rep.WaitingDetachedVolumes
	c.add(m, fmt.Sprintf("New candidates, labelled at the next report run: %d", len(fresh)+waiting), fresh, 0, waiting)
}

// nextRuns renders "Next runs: report Monday 08:00 · delete Tuesday 08:00."
// from what is known.
func (c Composer) nextRuns() string {
	var runs []string
	if c.ReportRunAt != "" {
		runs = append(runs, "report "+c.ReportRunAt)
	}
	if c.DeleteEnabled && c.DeleteRunAt != "" {
		runs = append(runs, "delete "+c.DeleteRunAt)
	}
	if len(runs) == 0 {
		return ""
	}
	return "Next runs: " + strings.Join(runs, " · ") + "."
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// Summary composes the message of a delete run.
func (c Composer) Summary(sum *report.DeletionSummary) Message {
	m := Message{
		Title:    summaryTitle(sum),
		Subtitle: c.subtitle(sum.Scope, nil, sum.GeneratedAt),
		Footer:   "costguard-vm " + c.Version,
	}
	if len(sum.Blocked) > 0 {
		m.Alerts = append(m.Alerts, "Nothing was deleted: these skip entries match nothing inside the scope (or it could not be read): "+
			strings.Join(sum.Blocked, ", ")+". Fix the skip list; entries must point to folders or projects inside the scope.")
	}
	if sum.Interrupted {
		m.Alerts = append(m.Alerts, fmt.Sprintf("This run was interrupted after %s. Everything listed below is what happened before that; the rest keeps its label and follows in the next run.",
			report.Count(len(sum.DeletedByThisRun()), "deletion")))
	}
	m.Alerts = append(m.Alerts, scanErrorAlert(sum.ScanErrors)...)
	for _, g := range []struct {
		status report.Status
		title  string
	}{
		{report.StatusDeleted, "Deleted"},
		{report.StatusFailed, "Failed, will be tried again in the next run"},
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
		m.Intro = fmt.Sprintf("This run deleted %s", report.Count(len(byRun), "resource"))
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

// summaryTitle names the most important thing a delete run did.
func summaryTitle(sum *report.DeletionSummary) string {
	deleted, failed, unflagged := sum.Count(report.StatusDeleted), sum.Count(report.StatusFailed), sum.Count(report.StatusUnflagged)
	switch {
	case len(sum.Blocked) > 0:
		return "costguard: deletions blocked"
	case deleted > 0:
		return "costguard: " + report.Count(deleted, "resource") + " deleted"
	case sum.Interrupted:
		return "costguard: delete run interrupted"
	case failed > 0:
		return "costguard: " + report.Count(failed, "deletion") + " failed"
	case unflagged > 0:
		return "costguard: delete label removed from " + report.Count(unflagged, "resource")
	}
	return "costguard: nothing deleted"
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
		Intro:  "Some labels could not be written after the message went out. costguard tries again at the next report run.",
		Footer: "costguard-vm " + c.Version,
	}
	c.add(&m, fmt.Sprintf("Not marked, so NOT deleted in the next delete run: %d", len(notFlagged)), notFlagged, 0, 0)
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
		Footer: "costguard-vm " + c.Version,
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
// label: the flag run removes it from volumes and public IPs, the rest
// needs a person.
func (c Composer) protectedMarkedItems(items []report.Item, readOnly bool) []report.Item {
	out := make([]report.Item, 0, len(items))
	for _, it := range items {
		switch {
		case it.Kind != "volume" && it.Kind != "publicip":
			it.Detail = "remove the delete label by hand; it is never deleted while do-not-delete is set"
		case readOnly && !c.DeleteEnabled:
			it.Detail = "with delete on, the delete label would be removed"
		case readOnly:
			it.Detail = "the delete label is removed at the next report run"
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
	parts := []string{strings.TrimSpace(report.KindName(it.Kind) + " " + name)}
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

// subtitle renders "Scope: … · regions eu01 · 2026-10-01 14:03
// (Europe/Berlin)", in the configured time zone like the schedules.
func (c Composer) subtitle(scope string, regions []string, at time.Time) string {
	parts := []string{"Scope: " + scope}
	if len(regions) > 0 {
		parts = append(parts, "regions "+strings.Join(regions, ", "))
	}
	if c.Location == nil || c.Location == time.UTC {
		parts = append(parts, at.UTC().Format("2006-01-02 15:04 UTC"))
	} else {
		parts = append(parts, at.In(c.Location).Format("2006-01-02 15:04")+" ("+c.Location.String()+")")
	}
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
