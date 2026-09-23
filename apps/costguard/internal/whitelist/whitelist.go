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

// Package whitelist provides the merged protection view over all three
// whitelist sources (env CSV + YAML static lists, and the Secrets Manager
// KV v2 entry) plus the store client used by the callback server to
// append protections. A hit in any source protects.
package whitelist

import (
	"log/slog"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
)

// ResourceType identifies which kind of resource a whitelist entry
// protects. Entries are typed so that an ID collision between resource
// kinds can never protect the wrong resource.
type ResourceType string

// The four scanned resource types.
const (
	TypeProject  ResourceType = "project"
	TypeFolder   ResourceType = "folder"
	TypeVolume   ResourceType = "volume"
	TypePublicIP ResourceType = "publicip"
)

// ParseResourceType reports whether s is a known resource type.
func ParseResourceType(s string) (ResourceType, bool) {
	switch ResourceType(strings.ToLower(s)) {
	case TypeProject, TypeFolder, TypeVolume, TypePublicIP:
		return ResourceType(strings.ToLower(s)), true
	default:
		return "", false
	}
}

// Entry is one whitelist record as stored in the Secrets Manager (one KV
// v2 secret, JSON object keyed by resource ID). Annotations stay
// minimal on purpose: no customer names required.
type Entry struct {
	Type    ResourceType `json:"type"`
	SavedBy string       `json:"savedBy,omitempty"`
	SavedAt time.Time    `json:"savedAt,omitempty"`
	Reason  string       `json:"reason,omitempty"`
}

// Whitelist is the read-only, per-run merged view. It is immutable after
// construction: the bot reads it at startup of each run (fresh, one-shot
// pods) and caches it within the run.
type Whitelist struct {
	projects                 map[string]bool
	folders                  map[string]bool
	volumes                  map[string]bool
	publicIPs                map[string]bool
	skipVolumeScanProjects   map[string]bool
	skipPublicIPScanProjects map[string]bool
}

// New builds the merged whitelist from the static (env+YAML) list and the
// Secrets Manager entries. SM entries are mapped into typed sets by their
// Entry.Type; entries with an unknown type are logged and skipped.
func New(static config.Whitelist, sm map[string]Entry, logger *slog.Logger) *Whitelist {
	w := &Whitelist{
		projects:                 make(map[string]bool),
		folders:                  make(map[string]bool),
		volumes:                  make(map[string]bool),
		publicIPs:                make(map[string]bool),
		skipVolumeScanProjects:   make(map[string]bool),
		skipPublicIPScanProjects: make(map[string]bool),
	}
	for _, id := range static.Projects {
		w.projects[id] = true
	}
	for _, id := range static.Folders {
		w.folders[id] = true
	}
	for _, id := range static.Volumes {
		w.volumes[id] = true
	}
	for _, id := range static.PublicIPs {
		w.publicIPs[id] = true
	}
	for _, id := range static.SkipVolumeScanProjects {
		w.skipVolumeScanProjects[id] = true
	}
	for _, id := range static.SkipPublicIPScanProjects {
		w.skipPublicIPScanProjects[id] = true
	}
	for id, e := range sm {
		if logger != nil {
			logger.Debug("whitelist: SM entry", "id", id, "type", e.Type, "savedBy", e.SavedBy)
		}
		switch e.Type {
		case TypeProject:
			w.projects[id] = true
		case TypeFolder:
			w.folders[id] = true
		case TypeVolume:
			w.volumes[id] = true
		case TypePublicIP:
			w.publicIPs[id] = true
		case "":
			if logger != nil {
				logger.Warn("whitelist: SM entry without type is ignored", "id", id)
			}
		default:
			if logger != nil {
				logger.Warn("whitelist: SM entry with unknown type is ignored", "id", id, "type", e.Type)
			}
		}
	}
	return w
}

// ProjectProtected reports whether the project ID is whitelisted in any
// source (such projects appear nowhere in the report).
func (w *Whitelist) ProjectProtected(id string) bool { return w.projects[id] }

// FolderProtected reports whether the folder ID is whitelisted in any
// source (such folders and everything inside are invisible).
func (w *Whitelist) FolderProtected(id string) bool { return w.folders[id] }

// VolumeProtected reports whether the volume ID is never to be deleted.
func (w *Whitelist) VolumeProtected(id string) bool { return w.volumes[id] }

// PublicIPProtected reports whether the public IP ID is never to be
// deleted.
func (w *Whitelist) PublicIPProtected(id string) bool { return w.publicIPs[id] }

// SkipVolumeScan reports whether volume scanning is skipped entirely for
// the project (e.g. backup projects with many expected detached volumes).
func (w *Whitelist) SkipVolumeScan(projectID string) bool { return w.skipVolumeScanProjects[projectID] }

// SkipPublicIPScan reports whether public IP scanning is skipped entirely
// for the project.
func (w *Whitelist) SkipPublicIPScan(projectID string) bool {
	return w.skipPublicIPScanProjects[projectID]
}
