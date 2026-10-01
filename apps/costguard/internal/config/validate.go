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
	"time"
)

var (
	regionPattern = regexp.MustCompile(`^[a-z]{2}[0-9]{2}$`)
	clockPattern  = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

const MaxThreshold = 1000

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
	switch runAt := strings.TrimSpace(c.DeleteRunAt); {
	case c.DeleteEnabled && runAt == "":
		add("deleteRunAt is required when deleteEnabled is true")
	case !c.DeleteEnabled && runAt != "":
		add("deleteRunAt is set, but deleteEnabled is false; set both or neither")
	}
	if c.DeleteEnabled && !c.ReportEnabled {
		add("deleteEnabled needs reportEnabled: nothing is deleted without a report message first")
	}
	if _, err := time.LoadLocation(c.TimeZone); err != nil || strings.TrimSpace(c.TimeZone) == "" || c.TimeZone == "Local" {
		add("timeZone must be an IANA time zone such as Europe/Berlin (got %q)", c.TimeZone)
	}
	if !absoluteHTTP(c.PortalURL) {
		add("portalUrl must be an absolute http(s) URL (got %q)", c.PortalURL)
	}
	if c.Budgets != nil {
		problems = append(problems, c.Budgets.validate()...)
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
}

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

func (b *Budgets) validate() []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	seenDay := map[string]bool{}
	for _, d := range b.Days {
		if dayNames[d] == "" {
			add("budgets.days: %q is not a day; write Mon, Tue, Wed, Thu, Fri, Sat or Sun", d)
		} else if seenDay[d] {
			add("budgets.days lists %s twice", d)
		}
		seenDay[d] = true
	}
	if len(b.Days) == 0 {
		add("budgets.days must list at least one day")
	}
	if !clockPattern.MatchString(b.Time) {
		add("budgets.time must be HH:MM (24 hours), e.g. \"10:00\" (got %q)", b.Time)
	}
	if len(b.Limits) == 0 {
		add("budgets.limits must list at least one budget")
	}
	seen := map[string]bool{}
	for i, l := range b.Limits {
		name := strings.TrimSpace(l.Name)
		field := fmt.Sprintf("budgets.limits[%d]", i)
		if name != "" {
			field = fmt.Sprintf("budget %q", l.Name)
		}
		switch key := strings.ToLower(name); {
		case key == "":
			add("%s needs a name", field)
		case seen[key]:
			add("budget name %q is used twice", l.Name)
		default:
			seen[key] = true
		}
		targets := 0
		if l.Organization {
			targets++
		}
		for _, t := range []string{l.Folder, l.Project} {
			if strings.TrimSpace(t) != "" {
				targets++
			}
		}
		if targets != 1 {
			add("%s needs exactly one target: organization: true, a folder or a project", field)
		}
		if !(l.MonthlyEUR > 0) {
			add("%s: monthlyEur must be more than 0", field)
		}
		if len(l.Thresholds) == 0 {
			add("%s needs at least one threshold", field)
		}
		for j, t := range l.Thresholds {
			if t < 1 || t > MaxThreshold {
				add("%s: thresholds must be between 1 and %d percent (got %d)", field, MaxThreshold, t)
			}
			if j > 0 && t <= l.Thresholds[j-1] {
				add("%s: thresholds must be ascending without repeats (got %v)", field, l.Thresholds)
				break
			}
		}
	}
	return problems
}
