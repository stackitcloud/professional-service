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

package scanner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/stackit"
)

// node is a folder or project of the walked tree.
type node struct {
	stackit.Container
	folder   bool
	parent   *node
	children []*node
	// skipRoot: this folder or project is skipped itself (skip list or
	// do-not-delete label), which skips everything below it.
	skipRoot bool
	// inScope: the node is inside the configured scope.
	inScope bool
}

// skipped reports whether the node or any folder above it is a skip root.
func (n *node) skipped() bool {
	for c := n; c != nil; c = c.parent {
		if c.skipRoot {
			return true
		}
	}
	return false
}

// tree is the resolved scope.
type tree struct {
	folders  []*node
	projects []*node
	byID     map[string]*node
	wholeOrg bool

	describe        string
	dangling        []string
	skippedFolders  int
	skippedProjects int
}

func (t *tree) add(c stackit.Container, folder bool, parent *node) *node {
	if n, ok := t.byID[c.ID]; ok {
		return n
	}
	n := &node{Container: c, folder: folder, parent: parent}
	if parent != nil {
		parent.children = append(parent.children, n)
	}
	t.byID[c.ID] = n
	if folder {
		t.folders = append(t.folders, n)
	} else {
		t.projects = append(t.projects, n)
	}
	return n
}

// relink attaches every node to the parent the API reports for it, when
// that parent is in the tree. The nesting of the walk is only the fallback:
// it would be wrong if a listing ever returned more than direct children,
// and then a project could escape the skip of its folder.
func (t *tree) relink() {
	nodes := append(append([]*node{}, t.folders...), t.projects...)
	for _, n := range nodes {
		n.children = nil
	}
	for _, n := range nodes {
		if p, ok := t.byID[n.ParentID]; ok && p != n && p.folder {
			n.parent = p
		}
		if n.parent != nil {
			n.parent.children = append(n.parent.children, n)
		}
	}
}

// active returns the in-scope projects that are not skipped.
func (t *tree) active() []*node {
	var out []*node
	for _, p := range t.projects {
		if p.inScope && !p.skipped() {
			out = append(out, p)
		}
	}
	return out
}

// resolve walks the scope, marks skips and finds dangling skip entries.
func (s *Scanner) resolve(ctx context.Context, errs *errorLog) (*tree, error) {
	cfg := s.Config
	t := &tree{byID: map[string]*node{}, wholeOrg: cfg.Scope.Empty()}

	needWholeOrg := t.wholeOrg
	for _, e := range append(append([]string{}, cfg.Scope.Folders...), cfg.Scope.Projects...) {
		if !config.IsID(e) {
			needWholeOrg = true
		}
	}

	if needWholeOrg {
		rootErr := s.walk(ctx, t, cfg.OrganizationID, nil, errs)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if rootErr != nil {
			// Nothing below the organization could be read: a report now
			// would say "nothing found". The login was checked before the
			// scan, so usually the service account's roles are missing.
			return nil, fmt.Errorf("costguard cannot read the organization %s. Check that its service account has the costguard.reader role on this organization (Terraform assigns it; run terraform apply).\n  - %s",
				cfg.OrganizationID, stackit.Describe(rootErr))
		}
		t.relink()
		if t.wholeOrg {
			for _, n := range t.byID {
				n.inScope = true
			}
			t.describe = "whole organization"
		} else if err := t.selectScope(cfg.Scope); err != nil {
			return nil, err
		}
	} else if err := s.fetchScope(ctx, t, errs); err != nil {
		return nil, err
	}

	t.applySkips(cfg.Skip)
	return t, nil
}

// walk lists everything below parentID. A listing error deeper down is a
// scan error: that subtree is simply not scanned. walk returns the error of
// listing parentID itself, so callers can refuse a scope root that cannot
// be read at all.
func (s *Scanner) walk(ctx context.Context, t *tree, parentID string, parent *node, errs *errorLog) error {
	if s.Pacer.Pace(ctx) != nil {
		return nil
	}
	container := parentID
	if parent != nil {
		container = parent.Name
	}
	projects, projectsErr := s.Clients.ResourceManager.ListProjects(ctx, parentID)
	if projectsErr != nil {
		errs.add("listing projects (their resources are not scanned)", container, projectsErr)
	}
	for _, p := range projects {
		if p.LifecycleState == "ACTIVE" {
			t.add(p, false, parent)
		}
	}
	if s.Pacer.Pace(ctx) != nil {
		return projectsErr
	}
	folders, err := s.Clients.ResourceManager.ListFolders(ctx, parentID)
	if err != nil {
		errs.add("listing folders (what is inside them is not scanned)", container, err)
		return err
	}
	for _, f := range folders {
		n := t.add(f, true, parent)
		_ = s.walk(ctx, t, f.ID, n, errs) // already recorded as a scan error
	}
	return projectsErr
}

// matches returns the nodes an entry names: by ID, or by name ignoring
// case.
func matches(nodes []*node, entry string) []*node {
	entry = strings.TrimSpace(entry)
	var out []*node
	for _, n := range nodes {
		if config.IsID(entry) {
			if strings.EqualFold(n.ID, entry) {
				out = append(out, n)
			}
		} else if strings.EqualFold(n.Name, entry) {
			out = append(out, n)
		}
	}
	return out
}

// selectScope resolves named scope entries in a whole-org walk. Each entry
// must match exactly one container.
func (t *tree) selectScope(scope config.Selection) error {
	var problems, parts []string
	pick := func(field string, entries []string, nodes []*node) {
		for _, e := range entries {
			m := matches(nodes, e)
			switch len(m) {
			case 0:
				problems = append(problems, fmt.Sprintf("%s: %q matches nothing (or it is not readable)", field, e))
			case 1:
				markScope(m[0])
				parts = append(parts, m[0].Name)
			default:
				problems = append(problems, fmt.Sprintf("%s: %q matches %d containers; use the ID", field, e, len(m)))
			}
		}
	}
	pick("scope.folders", scope.Folders, t.folders)
	pick("scope.projects", scope.Projects, t.projects)
	if len(problems) > 0 {
		return &ScopeError{Problems: problems}
	}
	t.describe = strings.Join(parts, ", ")
	return nil
}

func markScope(n *node) {
	n.inScope = true
	for _, c := range n.children {
		markScope(c)
	}
}

// fetchScope resolves a scope given only by IDs without reading the whole
// organization, so an install with folder-level rights works. Folders above
// a scope root are added (for skip names and labels) but are not in scope.
func (s *Scanner) fetchScope(ctx context.Context, t *tree, errs *errorLog) error {
	var problems, parts []string
	var roots []*node
	get := func(field, id string, folder bool) {
		if s.Pacer.Pace(ctx) != nil {
			return
		}
		var c *stackit.Container
		var err error
		if folder {
			c, err = s.Clients.ResourceManager.GetFolder(ctx, id)
		} else {
			c, err = s.Clients.ResourceManager.GetProject(ctx, id)
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s: %s", field, id, stackit.Describe(err)))
			return
		}
		if !folder && c.LifecycleState != "ACTIVE" {
			problems = append(problems, fmt.Sprintf("%s: %s is %s", field, id, c.LifecycleState))
			return
		}
		parent := s.ancestors(ctx, t, c.Ancestors)
		n := t.add(*c, folder, parent)
		if folder {
			if err := s.walk(ctx, t, c.ID, n, errs); err != nil && ctx.Err() == nil {
				problems = append(problems, fmt.Sprintf("%s: %s: its content cannot be listed: %s", field, id, stackit.Describe(err)))
				return
			}
		}
		roots = append(roots, n)
		parts = append(parts, c.Name)
	}
	for _, id := range s.Config.Scope.Folders {
		get("scope.folders", id, true)
	}
	for _, id := range s.Config.Scope.Projects {
		get("scope.projects", id, false)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return &ScopeError{Problems: problems}
	}
	t.relink()
	for _, n := range roots {
		markScope(n)
	}
	t.describe = strings.Join(parts, ", ")
	return nil
}

// ancestors adds the folders above a scope root, top first, and returns
// the nearest one. Their labels are read when the service account may;
// otherwise a do-not-delete on them cannot be seen, which is logged.
func (s *Scanner) ancestors(ctx context.Context, t *tree, chain []stackit.Ancestor) *node {
	var parent *node
	for i := len(chain) - 1; i >= 0; i-- {
		a := chain[i]
		c := stackit.Container{ID: a.ID, Name: a.Name}
		if parent != nil {
			c.ParentID = parent.ID
		}
		if _, known := t.byID[a.ID]; !known && s.Pacer.Pace(ctx) == nil {
			if f, err := s.Clients.ResourceManager.GetFolder(ctx, a.ID); err == nil {
				c.Labels = f.Labels
			} else {
				s.Logger.Info("cannot read the labels of a folder above the scope", "folder", a.Name, "error", err)
			}
		}
		parent = t.add(c, true, parent)
	}
	return parent
}

// applySkips marks skip roots, collects dangling skip entries and counts
// what is skipped inside the scope.
func (t *tree) applySkips(skip config.Selection) {
	for _, n := range t.byID {
		if stackit.Protected(n.Labels) {
			n.skipRoot = true
		}
	}
	mark := func(field string, entries []string, nodes []*node) {
		for _, e := range entries {
			m := matches(nodes, e)
			if len(m) == 0 {
				t.dangling = append(t.dangling, fmt.Sprintf("%s: %q", field, e))
			}
			for _, n := range m {
				n.skipRoot = true
			}
		}
	}
	mark("skip.folders", skip.Folders, t.folders)
	mark("skip.projects", skip.Projects, t.projects)
	sort.Strings(t.dangling)

	for _, f := range t.folders {
		if f.skipRoot && (f.inScope || isAboveScope(f)) {
			t.skippedFolders++
		}
	}
	for _, p := range t.projects {
		if p.inScope && p.skipped() {
			t.skippedProjects++
		}
	}
}

// isAboveScope reports whether an out-of-scope folder contains a scope
// root, so skipping it skips part of the scope.
func isAboveScope(n *node) bool {
	for _, c := range n.children {
		if c.inScope || isAboveScope(c) {
			return true
		}
	}
	return false
}
