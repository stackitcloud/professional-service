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

package budget

import (
	"context"
	"fmt"
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// maxDepth bounds walking up or down the folder tree against loops.
const maxDepth = 100

// target is a budget with what it covers.
type target struct {
	budget   config.Budget
	describe string
	// wholeOrg: every project the Cost API lists, deleted ones included.
	wholeOrg bool
	// projects are the projects of a folder or project budget.
	projects map[string]bool
	// projectID is set for project budgets.
	projectID string
}

func (t target) covers(projectID string) bool {
	return t.wholeOrg || t.projects[projectID]
}

// orgTree is every folder and project of the organization, linked by the
// parents the API reports (not by the nesting of the walk, in case a
// listing ever returns more than direct children).
type orgTree struct {
	folders  []stackit.Container
	projects []stackit.Container
	parent   map[string]string
	// incomplete is the first listing error below the organization.
	incomplete error
}

// resolve finds each budget's target. The tree is read only when a
// budget names a folder or project. A budget whose target matches nothing
// or several containers is a problem, and so is every folder or project
// budget when part of the tree could not be read (a missing project could
// hide spend or make a name look unique).
func (c *Checker) resolve(ctx context.Context) ([]target, []string, error) {
	var tree *orgTree
	for _, b := range c.Config.Budgets.Limits {
		if !b.Organization && tree == nil {
			t, err := c.readTree(ctx)
			if err != nil {
				return nil, nil, err
			}
			tree = t
		}
	}

	var targets []target
	var problems []string
	for _, b := range c.Config.Budgets.Limits {
		if b.Organization {
			targets = append(targets, target{budget: b, describe: "organization", wholeOrg: true})
			continue
		}
		if tree.incomplete != nil {
			problems = append(problems, fmt.Sprintf("budget %q: not checked, the organization's folders and projects could not all be read: %s", b.Name, stackit.Describe(tree.incomplete)))
			continue
		}
		kind, entry, candidates := "folder", b.Folder, tree.folders
		if b.Folder == "" {
			kind, entry = "project", b.Project
			candidates = tree.projects
			if !config.IsID(strings.TrimSpace(entry)) {
				candidates = active(tree.projects)
			}
		}
		m := matches(candidates, entry)
		switch len(m) {
		case 0:
			problems = append(problems, fmt.Sprintf("budget %q: %s %q matches nothing (or it is not readable)", b.Name, kind, entry))
			continue
		case 1:
		default:
			problems = append(problems, fmt.Sprintf("budget %q: %s %q matches %d %ss; use the ID", b.Name, kind, entry, len(m), kind))
			continue
		}
		t := target{budget: b, describe: kind + " " + m[0].Name, projects: map[string]bool{}}
		if kind == "project" {
			t.projectID = m[0].ID
			t.projects[m[0].ID] = true
		} else {
			for _, p := range tree.projects {
				if tree.below(p.ID, m[0].ID) {
					t.projects[p.ID] = true
				}
			}
		}
		targets = append(targets, t)
	}
	return targets, problems, nil
}

// readTree lists every folder and project of the organization. When the
// organization itself cannot be listed nothing can be checked, which
// usually means the service account's role is missing.
func (c *Checker) readTree(ctx context.Context) (*orgTree, error) {
	t := &orgTree{parent: map[string]string{}}
	seen := map[string]bool{}
	var walk func(parentID string, depth int) error
	walk = func(parentID string, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("the folders below %s are nested more than %d levels deep", parentID, maxDepth)
		}
		if err := c.Pacer.Pace(ctx); err != nil {
			return err
		}
		projects, err := c.Clients.ResourceManager.ListProjects(ctx, parentID)
		if err != nil {
			return err
		}
		for _, p := range projects {
			if !seen[p.ID] {
				seen[p.ID] = true
				t.projects = append(t.projects, p)
				t.parent[p.ID] = p.ParentID
			}
		}
		if err := c.Pacer.Pace(ctx); err != nil {
			return err
		}
		folders, err := c.Clients.ResourceManager.ListFolders(ctx, parentID)
		if err != nil {
			return err
		}
		for _, f := range folders {
			if seen[f.ID] {
				continue
			}
			seen[f.ID] = true
			t.folders = append(t.folders, f)
			t.parent[f.ID] = f.ParentID
			if err := walk(f.ID, depth+1); err != nil && t.incomplete == nil {
				t.incomplete = err
			}
		}
		return nil
	}
	if err := walk(c.Config.OrganizationID, 0); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("costguard cannot read the organization %s. Check that its service account has the costguard.reader role on this organization (Terraform assigns it; run terraform apply).\n  - %s",
			c.Config.OrganizationID, stackit.Describe(err))
	}
	if t.incomplete != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return t, nil
}

// below reports whether the container lies anywhere below the folder.
func (t *orgTree) below(id, folderID string) bool {
	p := t.parent[id]
	for i := 0; i < maxDepth && p != ""; i++ {
		if p == folderID {
			return true
		}
		p = t.parent[p]
	}
	return false
}

// active returns the projects in the ACTIVE state: a name should not
// become ambiguous because of a project that is being deleted.
func active(projects []stackit.Container) []stackit.Container {
	var out []stackit.Container
	for _, p := range projects {
		if p.LifecycleState == "ACTIVE" {
			out = append(out, p)
		}
	}
	return out
}

// matches returns the containers an entry names: by ID, or by name
// ignoring case (the rule of scope entries).
func matches(containers []stackit.Container, entry string) []stackit.Container {
	entry = strings.TrimSpace(entry)
	var out []stackit.Container
	for _, c := range containers {
		if config.IsID(entry) {
			if strings.EqualFold(c.ID, entry) {
				out = append(out, c)
			}
		} else if strings.EqualFold(c.Name, entry) {
			out = append(out, c)
		}
	}
	return out
}
