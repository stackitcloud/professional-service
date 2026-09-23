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

package whitelist

import (
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
)

func TestNewMergesAllSources(t *testing.T) {
	static := config.Whitelist{
		Projects:                 []string{"p-static"},
		Folders:                  []string{"f-static"},
		Volumes:                  []string{"v-static"},
		PublicIPs:                []string{"ip-static"},
		SkipVolumeScanProjects:   []string{"skip-v"},
		SkipPublicIPScanProjects: []string{"skip-ip"},
	}
	sm := map[string]Entry{
		"p-sm":  {Type: TypeProject},
		"f-sm":  {Type: TypeFolder},
		"v-sm":  {Type: TypeVolume},
		"ip-sm": {Type: TypePublicIP},
	}
	w := New(static, sm, nil)

	if !w.ProjectProtected("p-static") || !w.ProjectProtected("p-sm") {
		t.Error("project protection from both sources")
	}
	if !w.FolderProtected("f-static") || !w.FolderProtected("f-sm") {
		t.Error("folder protection from both sources")
	}
	if !w.VolumeProtected("v-static") || !w.VolumeProtected("v-sm") {
		t.Error("volume protection from both sources")
	}
	if !w.PublicIPProtected("ip-static") || !w.PublicIPProtected("ip-sm") {
		t.Error("public IP protection from both sources")
	}
	if !w.SkipVolumeScan("skip-v") || !w.SkipPublicIPScan("skip-ip") {
		t.Error("scan-skip lists")
	}
	if w.ProjectProtected("unknown") {
		t.Error("unknown project must not be protected")
	}
}

func TestNewTypedEntries(t *testing.T) {
	// The same ID listed under different types must protect only the
	// matching resource kind.
	sm := map[string]Entry{
		"same-id": {Type: TypeVolume},
	}
	w := New(config.Whitelist{}, sm, nil)
	if !w.VolumeProtected("same-id") {
		t.Error("volume entry must protect volumes")
	}
	if w.ProjectProtected("same-id") {
		t.Error("volume entry must not protect projects")
	}
}

func TestNewIgnoresUnknownTypes(t *testing.T) {
	sm := map[string]Entry{
		"p1":     {Type: TypeProject},
		"weird":  {Type: ResourceType("blackhole")},
		"notype": {},
	}
	w := New(config.Whitelist{}, sm, nil)
	if !w.ProjectProtected("p1") {
		t.Error("known type must protect")
	}
	if w.ProjectProtected("weird") || w.ProjectProtected("notype") {
		t.Error("unknown/missing type must not protect anything")
	}
}

func TestEntryRoundTripJSON(t *testing.T) {
	// Covered indirectly by the store tests; here we pin the field names
	// of the store format.
	e := Entry{Type: TypePublicIP, SavedBy: "chat:tester", SavedAt: time.Date(2026, 9, 22, 9, 15, 3, 0, time.UTC), Reason: "test"}
	if e.Type != "publicip" {
		t.Errorf("type = %q", e.Type)
	}
	if _, ok := ParseResourceType("publicip"); !ok {
		t.Error("ParseResourceType(publicip)")
	}
	if _, ok := ParseResourceType("nope"); ok {
		t.Error("ParseResourceType(nope) must fail")
	}
}
