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
	"math"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

type Mode string

const (
	ModeReport  Mode = "report"
	ModeFlag    Mode = "flag"
	ModeDelete  Mode = "delete"
	ModeBudgets Mode = "budgets"
	ModeBoot    Mode = "boot"
)

type Composer struct {
	Organization       string
	OrganizationID     string
	PortalURL          string
	DeleteEnabled      bool
	DeleteRunAt        string
	ReportRunAt        string
	BudgetsRunAt       string
	Location           *time.Location
	Prices             report.Prices
	WarnEmptyAfterDays int
	Version            string
}

const (
	keepHint    = "To keep a resource, open it and set the label do-not-delete=true (or remove delete=true)."
	keepHintOff = "To keep a resource once delete is on, set the label do-not-delete=true on it."
)

func (c Composer) Report(rep *report.Report, mode Mode) Message {
	n := rep.ToDelete()
	saving := savingPhrase(report.Estimate(toDelete(rep), c.Prices))
	readOnly := mode == ModeReport || mode == ModeBoot

	m := Message{Title: c.title("report"), Subtitle: c.subtitle(rep.Scope, rep.Regions, rep.GeneratedAt)}
	switch {
	case readOnly:
		if mode == ModeBoot {
			m.Title = c.title(c.Version + " is running")
		}
		m.Intro = c.readOnlyIntro(rep, mode, n, saving)
	case n == 0:
		m.Intro = "Nothing to delete."
	case len(rep.Blocked) > 0:
		m.Intro = fmt.Sprintf("Once the skip list is fixed, %s would be deleted", report.Count(n, "resource"))
		if saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += "."
	default:
		m.Intro = fmt.Sprintf("%s labelled delete=true %s deleted %s", report.Count(n, "resource"), isAre(n), c.DeleteRunAt)
		if saving != "" {
			m.Intro += ", saving " + saving
		}
		m.Intro += ". " + keepHint
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

	if mode == ModeBoot {
		if next := c.nextRuns(); next != "" {
			m.Intro += " " + next
		}
	}
	if mode != ModeBoot {
		addScanErrors(&m, rep.ScanErrors)
	}
	m.Footer = fmt.Sprintf("Skipped: %s, %s · costguard %s",
		report.Count(rep.SkippedFolders, "folder"), report.Count(rep.SkippedProjects, "project"), c.Version)
	return m
}

func (c Composer) Boot(rep *report.Report, chk *report.BudgetCheck, budgetErr error, at time.Time) Message {
	var m Message
	if rep != nil {
		m = c.Report(rep, ModeBoot)
	} else {
		m = Message{
			Title:    c.title(c.Version + " is running"),
			Subtitle: c.clock(at),
			Intro:    "Login and this chat work. The cleanup report is off.",
			Footer:   "costguard " + c.Version,
		}
		if next := c.nextRuns(); next != "" {
			m.Intro += " " + next
		}
	}
	switch {
	case chk != nil:
		c.addBudgets(&m, chk)
	case budgetErr != nil:
		lines := strings.Split(strings.TrimSpace(budgetErr.Error()), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "- "))
		}
		m.Alerts = append(m.Alerts, "The budgets could not be checked: "+strings.Join(lines, " "))
	}
	if rep != nil {
		addScanErrors(&m, rep.ScanErrors)
	}
	return m
}

func toDelete(rep *report.Report) []report.Item {
	var out []report.Item
	for _, list := range [][]report.Item{rep.IdlePublicIPs, rep.DetachedVolumes, rep.Requested} {
		out = append(out, list...)
	}
	return out
}

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

func (c Composer) nextRuns() string {
	var runs []string
	if c.ReportRunAt != "" {
		runs = append(runs, "report "+c.ReportRunAt)
	}
	if c.DeleteEnabled && c.DeleteRunAt != "" {
		runs = append(runs, "delete "+c.DeleteRunAt)
	}
	if c.BudgetsRunAt != "" {
		runs = append(runs, "budgets "+c.BudgetsRunAt)
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

func (c Composer) Summary(sum *report.DeletionSummary) Message {
	m := Message{
		Title:    c.title("deletion"),
		Subtitle: c.subtitle(sum.Scope, nil, sum.GeneratedAt),
		Footer:   "costguard " + c.Version,
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
	m.Intro = c.summaryIntro(sum)
	addScanErrors(&m, sum.ScanErrors)
	return m
}

func (c Composer) summaryIntro(sum *report.DeletionSummary) string {
	byRun := sum.DeletedByThisRun()
	failed, unflagged := sum.Count(report.StatusFailed), sum.Count(report.StatusUnflagged)
	switch {
	case len(byRun) > 0:
		intro := "This run deleted " + report.Count(len(byRun), "resource")
		if saving := savingPhrase(report.Estimate(byRun, c.Prices)); saving != "" {
			intro += ", saving " + saving
		}
		return intro + "."
	case len(sum.Blocked) > 0:
		return ""
	case failed > 0:
		return report.Count(failed, "deletion") + " failed; they are tried again in the next run."
	case unflagged > 0:
		return fmt.Sprintf("Nothing was deleted. The delete label was removed from %s that %s in use again.", report.Count(unflagged, "resource"), isAre(unflagged))
	}
	return "Nothing was deleted."
}

func (c Composer) link(it report.Item) string {
	return PortalLink(c.PortalURL, c.OrganizationID, it)
}

func (c Composer) title(kind string) string {
	if c.Organization == "" {
		return "costguard: " + kind
	}
	return "costguard: " + kind + " (" + c.Organization + ")"
}

func savingPhrase(s report.Savings) string {
	total := s.TotalEUR()
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("about %s per month (%s per year)", eur(total), eur(total*12))
}

func (c Composer) FlagProblems(notFlagged, notCleared []report.Item) Message {
	m := Message{
		Title:  c.title("correction"),
		Intro:  "Some labels could not be written after today's message went out. costguard tries again at the next report run.",
		Footer: "costguard " + c.Version,
	}
	c.add(&m, fmt.Sprintf("Not marked, so NOT deleted in the next delete run: %d", len(notFlagged)), notFlagged, 0, 0)
	c.add(&m, fmt.Sprintf("delete label could not be removed (they are not deleted either way): %d", len(notCleared)), notCleared, 0, 0)
	return m
}

func (c Composer) Failure(mode Mode, err error) Message {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	m := Message{
		Title:  c.title(string(mode) + " run failed"),
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

func (c Composer) add(m *Message, title string, items []report.Item, limit, waiting int) {
	if len(items) == 0 && waiting == 0 {
		return
	}
	sec := Section{Title: title, Omitted: waiting, Folded: true}
	for i, it := range items {
		if limit > 0 && i == limit {
			sec.Omitted += len(items) - limit
			break
		}
		sec.Lines = append(sec.Lines, Line{Text: lineText(it), Link: c.link(it)})
	}
	m.Sections = append(m.Sections, sec)
}

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

func (c Composer) subtitle(scope string, regions []string, at time.Time) string {
	parts := []string{"Scope: " + scope}
	if len(regions) > 0 {
		parts = append(parts, "regions "+strings.Join(regions, ", "))
	}
	parts = append(parts, c.clock(at))
	return strings.Join(parts, " · ")
}

func scanErrorAlert(errs []string) []string {
	if len(errs) == 0 {
		return nil
	}
	return []string{"Some things could not be read, so this may be incomplete. Nothing that could not be read is flagged or deleted. Details at the end."}
}

func addScanErrors(m *Message, errs []string) {
	if len(errs) == 0 {
		return
	}
	sec := Section{Title: fmt.Sprintf("Could not be read: %d", len(errs)), Folded: true}
	for _, e := range errs {
		sec.Lines = append(sec.Lines, Line{Text: e})
	}
	m.Sections = append(m.Sections, sec)
}

func eur(v float64) string {
	s := fmt.Sprintf("%.2f", math.Abs(v))
	whole, cents := s[:len(s)-3], s[len(s)-2:]
	var b strings.Builder
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteByte(whole[i])
	}
	sign := ""
	if v < 0 && s != "0.00" {
		sign = "-"
	}
	return sign + "€" + b.String() + "," + cents
}
