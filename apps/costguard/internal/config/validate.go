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
	"strings"
	"time"
)

// Validate checks all required and conditional fields and returns a
// single error listing every problem. It must run before any
// API call; the caller exits non-zero on error.
func (c *Config) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if c.Scope == "" {
		add("scope is required (env %s, values: %s or %s)", EnvScope, ScopeOrganisation, ScopeFolder)
	} else if c.Scope != ScopeOrganisation && c.Scope != ScopeFolder {
		add("scope must be %q or %q (got %q)", ScopeOrganisation, ScopeFolder, c.Scope)
	}
	if c.OrgID == "" {
		add("organisation ID is required (env %s)", EnvOrgID)
	}
	if c.Scope == ScopeFolder && len(c.FolderIDs) == 0 {
		add("scope=folder requires at least one folder ID (env %s)", EnvFolderIDs)
	}
	if len(c.Regions) == 0 {
		add("at least one region is required (env %s, e.g. eu01)", EnvRegions)
	}
	if c.SafeLabelKey == "" {
		add("safe label key is required (env %s)", EnvSafeLabelKey)
	}
	if c.SafeLabelValue == "" {
		add("safe label value is required (env %s)", EnvSafeLabelValue)
	}

	switch c.Output {
	case "":
		add("output is required (env %s, values: %s, %s or %s)", EnvOutput, OutputGoogleChat, OutputSlack, OutputTeams)
	case OutputPrometheus:
		// Deferred in v1: reject with a clear, actionable error.
		add("output=prometheus is not available in v1; use %s, %s or %s", OutputGoogleChat, OutputSlack, OutputTeams)
	case OutputGoogleChat, OutputSlack, OutputTeams:
		if c.WebhookURL == "" {
			add("webhook URL is required for output %q (env %s)", c.Output, EnvWebhookURL)
		}
	default:
		add("output must be one of %s, %s or %s (got %q)", OutputGoogleChat, OutputSlack, OutputTeams, c.Output)
	}
	if c.WebhookURL != "" {
		u, err := url.Parse(c.WebhookURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			add("webhook URL must be an absolute http(s) URL (got %q)", c.WebhookURL)
		}
	}

	// Sanity bounds: these kill the "typed 1 instead of 90" failure class.
	if c.MaxAgeDays < 7 {
		add("maxAgeDays must be at least 7 (got %d) — check for a typo such as 1 instead of 90", c.MaxAgeDays)
	}
	if c.SNAMaxAgeDays < 7 {
		add("snaMaxAgeDays must be at least 7 (got %d) — check for a typo", c.SNAMaxAgeDays)
	}
	if c.GracePeriod < 2*time.Hour {
		// The grace period is the floor of the warning window and must
		// exceed the 1 h scan→delete offset with margin, otherwise a
		// candidate first marked by this week's scan would be deletable
		// by this week's delete run.
		add("gracePeriod must be at least 2h (got %s) to exceed the 1h scan→delete offset with margin", c.GracePeriod)
	}
	if c.CostAnomalyThresholdPct < 0 {
		add("costAnomalyThresholdPct must not be negative (got %v)", c.CostAnomalyThresholdPct)
	}
	if c.VolumeCostEurPerGB < 0 {
		add("volumeCostEurPerGB must not be negative (got %v)", c.VolumeCostEurPerGB)
	}
	if c.PublicIPCostEurPerMonth < 0 {
		add("publicIPCostEurPerMonth must not be negative (got %v)", c.PublicIPCostEurPerMonth)
	}

	// S3: all-or-nothing group.
	if n := c.S3.SetCount(); n != 0 && !c.S3.Complete() {
		add("partial S3 configuration: %d of 5 fields set; set all of %s or none", n,
			strings.Join([]string{EnvS3Endpoint, EnvS3Region, EnvS3AccessKey, EnvS3SecretKey, EnvS3Bucket}, ", "))
	}

	// Callback: when buttons are enabled the callback server has to be
	// fully configurable — it has nowhere else to write its whitelist.
	if c.CallbackURL != "" {
		if _, err := url.Parse(c.CallbackURL); err != nil || !strings.HasPrefix(c.CallbackURL, "http") {
			add("callback URL must be an absolute http(s) URL (got %q)", c.CallbackURL)
		}
		if c.CallbackPort < 1 || c.CallbackPort > 65535 {
			add("callback port must be between 1 and 65535 (got %d)", c.CallbackPort)
		}
		if c.CallbackSecret == "" {
			add("callback HMAC secret is required when %s is set (env %s)", EnvCallbackURL, EnvCallbackSecret)
		}
		if c.WhitelistSecretPath == "" {
			add("%s is required when %s is set — the callback has nowhere else to write", EnvWhitelistSecretPath, EnvCallbackURL)
		}
		if c.SecretsManager.URL == "" || c.SecretsManager.Username == "" || c.SecretsManager.Password == "" {
			add("Secrets Manager credentials are required when %s is set (env %s, %s, %s)",
				EnvCallbackURL, EnvSMURL, EnvSMUsername, EnvSMPasswrd)
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
}
