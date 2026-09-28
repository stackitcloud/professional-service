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

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var regionPattern = regexp.MustCompile(`^[a-z]{2}[0-9]{2}$`)

// Validate checks every field and returns one error that lists all
// problems. It runs before any API call.
func (c *Config) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if c.OrganizationID == "" {
		add("organizationId is required")
	} else if !IsID(c.OrganizationID) {
		add("organizationId must be a UUID (got %q)", c.OrganizationID)
	}

	checkEntries := func(field string, entries []string) {
		seen := map[string]bool{}
		for _, e := range entries {
			key := strings.ToLower(strings.TrimSpace(e))
			switch {
			case key == "":
				add("%s contains an empty entry", field)
			case seen[key]:
				add("%s lists %q twice", field, e)
			}
			seen[key] = true
		}
	}
	checkEntries("scope.folders", c.Scope.Folders)
	checkEntries("scope.projects", c.Scope.Projects)
	checkEntries("skip.folders", c.Skip.Folders)
	checkEntries("skip.projects", c.Skip.Projects)

	seenRegion := map[string]bool{}
	for _, r := range c.Regions {
		if !regionPattern.MatchString(r) {
			add("region %q is not a STACKIT region ID such as eu01", r)
		}
		if seenRegion[r] {
			add("region %q is listed twice", r)
		}
		seenRegion[r] = true
	}

	switch c.Output {
	case OutputGoogleChat, OutputSlack, OutputTeams:
	case "":
		add("output is required (%s, %s or %s)", OutputGoogleChat, OutputSlack, OutputTeams)
	default:
		add("output must be %s, %s or %s (got %q)", OutputGoogleChat, OutputSlack, OutputTeams, c.Output)
	}
	if c.WebhookURL == "" {
		add("the webhook URL is required (env %s)", EnvWebhookURL)
	} else if !secureURL(c.WebhookURL) {
		add("%s must be an https URL", EnvWebhookURL)
	}

	if c.WarnEmptyAfterDays != 0 && c.WarnEmptyAfterDays < 7 {
		add("warnEmptyAfterDays must be 0 (warnings off) or at least 7 (got %d)", c.WarnEmptyAfterDays)
	}
	if strings.TrimSpace(c.DeleteRunAt) == "" {
		add("deleteRunAt must not be empty")
	}
	if c.Prices.PublicIPMonthlyEUR < 0 || c.Prices.VolumeGBMonthlyEUR < 0 {
		add("prices must not be negative")
	}
	if !absoluteHTTP(c.PortalURL) {
		add("portalUrl must be an absolute http(s) URL (got %q)", c.PortalURL)
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
}

// secureURL accepts https URLs, and http only on the local machine (tests).
// Messages carry resource names and portal links, so they are never sent
// unencrypted over the network.
func secureURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	return false
}

func absoluteHTTP(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
