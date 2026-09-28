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
	"net/url"
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// projectPath is the portal page of a project.
//
// ❓ Open: every link points to the resource's project page for now,
// because the portal's URL format is not verified yet. Once it is checked
// on the test org, add per-kind deep links here (server, volume, public
// IP, snapshot, image, NIC, security group, network area) and verify this
// path too. See TODO.md, phase 3.
const projectPath = "/projects/{project}"

// PortalLink returns the portal page for an item, or "" when there is none.
// Network areas belong to the organization and have no link yet.
func PortalLink(portalURL string, it report.Item) string {
	if it.ProjectID == "" {
		return ""
	}
	return strings.TrimRight(portalURL, "/") + strings.ReplaceAll(projectPath, "{project}", url.PathEscape(it.ProjectID))
}
