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

func PortalLink(portalURL, organizationID string, it report.Item) string {
	base := strings.TrimRight(portalURL, "/")
	id := url.PathEscape(it.ID)
	if it.Kind == report.KindNetworkArea {
		if organizationID == "" {
			return ""
		}
		return base + "/network-area/network-areas/" + id + "/overview?organization=" + url.QueryEscape(organizationID)
	}
	if it.ProjectID == "" {
		return ""
	}
	inProject := func(path string) string {
		return base + path + "?project=" + url.QueryEscape(it.ProjectID)
	}
	switch it.Kind {
	case "server":
		return inProject("/server/servers/" + id + "/overview")
	case "volume":
		return inProject("/disk-volumes/volumes/" + id + "/overview")
	case "publicip":
		return inProject("/public-ip/public-ips/" + id + "/overview")
	case "snapshot":
		if it.VolumeID != "" {
			return inProject("/disk-volumes/volumes/" + url.PathEscape(it.VolumeID) + "/snapshots/" + id)
		}
	case "nic":
		return inProject("/nic/nics/" + id + "/overview")
	case "securitygroup":
		return inProject("/security-group/groups/" + id + "/overview")
	}
	return dashboard(portalURL, "project", it.ProjectID)
}

func dashboard(portalURL, container, id string) string {
	if id == "" {
		return ""
	}
	return strings.TrimRight(portalURL, "/") + "/dashboard?" + container + "=" + url.QueryEscape(id)
}
