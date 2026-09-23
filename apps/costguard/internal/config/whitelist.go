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

package config

import "strings"

// MergeWhitelist unions two static whitelists, deduplicating while
// preserving order — protection must be monotonic: a resource listed in
// any source is protected.
func MergeWhitelist(base, extra Whitelist) Whitelist {
	return Whitelist{
		Projects:                 union(base.Projects, extra.Projects),
		Folders:                  union(base.Folders, extra.Folders),
		Volumes:                  union(base.Volumes, extra.Volumes),
		PublicIPs:                union(base.PublicIPs, extra.PublicIPs),
		SkipVolumeScanProjects:   union(base.SkipVolumeScanProjects, extra.SkipVolumeScanProjects),
		SkipPublicIPScanProjects: union(base.SkipPublicIPScanProjects, extra.SkipPublicIPScanProjects),
	}
}

func union(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string{}, a...), b...) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
