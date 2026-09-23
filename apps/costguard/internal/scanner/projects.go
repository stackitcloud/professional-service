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
	"sync"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// hasSafeLabel reports whether the resource carries the configured safe
// label.
func (s *Scanner) hasSafeLabel(labels map[string]string) bool {
	return labels[s.Config.SafeLabelKey] == s.Config.SafeLabelValue
}

// walk traverses the org tree from the configured scope roots and returns
// every project with its protection state. One walk feeds all scanners.
func (s *Scanner) walk(ctx context.Context) ([]scannedProject, error) {
	var roots []containerRef
	if s.Config.Scope == config.ScopeFolder {
		for _, folderID := range s.Config.FolderIDs {
			ref := containerRef{ID: folderID, Name: folderID}
			// Fetch the folder so its own safe label is honored at the
			// root of the walk (folder label protects the whole
			// subtree).
			if s.Pacer.Pace(ctx) == nil {
				f, err := s.Clients.ResourceManager.GetFolder(ctx, folderID)
				if err != nil {
					return nil, fmt.Errorf("getting scope folder %s: %w", folderID, err)
				}
				ref.folder = f
				ref.Name = f.Name
			}
			roots = append(roots, ref)
		}
	} else {
		roots = append(roots, containerRef{ID: s.Config.OrgID, Name: "Organization"})
	}

	out := make([]scannedProject, 0)
	var firstErr error
	for _, root := range roots {
		s.walkContainer(ctx, root, false, &out, &firstErr)
	}
	return out, firstErr
}

// walkContainer lists the projects directly inside the container, then
// recurses into its sub-folders, carrying folder protection down the tree.
func (s *Scanner) walkContainer(ctx context.Context, ref containerRef, inherited bool, out *[]scannedProject, firstErr *error) {
	if *firstErr != nil || ctx.Err() != nil {
		return
	}
	// Protection is decided on the folder's OWN id (label or whitelist),
	// not the parent's.
	protected := inherited
	if ref.folder != nil {
		protected = protected || s.hasSafeLabel(ref.folder.Labels) || s.Whitelist.FolderProtected(ref.ID)
	}

	if s.Pacer.Pace(ctx) != nil {
		return
	}
	projects, err := s.Clients.ResourceManager.ListProjects(ctx, ref.ID)
	if err != nil {
		*firstErr = err
		return
	}
	for _, p := range projects {
		if p.LifecycleState != "ACTIVE" {
			continue
		}
		invisible := protected || s.Whitelist.ProjectProtected(p.ID)
		*out = append(*out, scannedProject{
			Project:          p,
			ParentFolderName: ref.Name,
			Invisible:        invisible,
		})
	}

	if s.Pacer.Pace(ctx) != nil {
		return
	}
	folders, err := s.Clients.ResourceManager.ListFolders(ctx, ref.ID)
	if err != nil {
		*firstErr = err
		return
	}
	for _, f := range folders {
		sub := containerRef{ID: f.ID, Name: f.Name, folder: &f}
		s.walkContainer(ctx, sub, protected, out, firstErr)
	}
}

// ageDays is the whole-day age of a resource at the current time.
func (s *Scanner) ageDays(createdAt time.Time) int {
	if createdAt.IsZero() {
		return 0
	}
	return int(s.Now().Sub(createdAt).Hours() / 24)
}

// staleProjects selects hygiene candidates (report-only)
// and enriches each with content probes so the report shows why an old
// project still matters.
func (s *Scanner) staleProjects(ctx context.Context, projects []scannedProject, addError func(string, ...any)) []report.StaleProject {
	maxAge := time.Duration(s.Config.MaxAgeDays) * 24 * time.Hour
	var candidates []scannedProject
	for _, p := range projects {
		if p.CreationTime.IsZero() {
			continue
		}
		if s.Now().Sub(p.CreationTime) <= maxAge {
			// Boundary: exactly MaxAgeDays old is NOT a candidate.
			continue
		}
		if s.hasSafeLabel(p.Labels) {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		return nil
	}

	out := make([]report.StaleProject, 0, len(candidates))
	jobs := make([]func(context.Context), 0, len(candidates))
	var mu sync.Mutex
	for i := range candidates {
		i := i
		jobs = append(jobs, func(ctx context.Context) {
			c := candidates[i]
			probe := s.contentProbes(ctx, c, addError)
			mu.Lock()
			out = append(out, report.StaleProject{
				ID:               c.ID,
				Name:             c.Name,
				AgeDays:          s.ageDays(c.CreationTime),
				CreatedAt:        c.CreationTime,
				ParentFolderID:   c.ParentContainerID,
				ParentFolderName: c.ParentFolderName,
				SKEClusters:      probe.clusters,
				StorageBuckets:   probe.buckets,
				ServerCount:      probe.servers,
			})
			mu.Unlock()
		})
	}
	runPooled(ctx, s.Workers, jobs)
	return out
}

// contentProbeResult carries the per-project content signals.
type contentProbeResult struct {
	clusters []string
	buckets  []string
	servers  int
}

// contentProbes lists what still lives in the project: SKE clusters,
// object storage buckets and IaaS servers. Probe failures are non-fatal
// — the project is still reported, just without that signal.
func (s *Scanner) contentProbes(ctx context.Context, p scannedProject, addError func(string, ...any)) contentProbeResult {
	var res contentProbeResult
	if err := s.Pacer.Pace(ctx); err == nil {
		if res.clusters, err = s.Clients.SKE.ListClusterNames(ctx, p.ID); err != nil {
			addError("content probe (SKE) project %s: %v", p.Name, err)
		}
	}
	if err := s.Pacer.Pace(ctx); err == nil {
		if res.buckets, err = s.Clients.ObjectStorage.ListBucketNames(ctx, p.ID); err != nil {
			addError("content probe (object storage) project %s: %v", p.Name, err)
		}
	}
	for _, region := range s.Config.Regions {
		if err := s.Pacer.Pace(ctx); err != nil {
			break
		}
		servers, err := s.Clients.IaaS.ListServers(ctx, p.ID, region)
		if err != nil {
			addError("content probe (servers) project %s region %s: %v", p.Name, region, err)
			continue
		}
		res.servers += len(servers)
	}
	return res
}
