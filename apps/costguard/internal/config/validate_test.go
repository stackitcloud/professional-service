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
	"strconv"
	"strings"
	"testing"
	"time"
)

func mustValid(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load("", envFrom(validEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid env must pass validation: %v", err)
	}
	return cfg
}

func TestValidateValid(t *testing.T) {
	mustValid(t)
}

func TestValidateMissingRequired(t *testing.T) {
	cases := []struct {
		name   string
		unset  string
		expect string
	}{
		{"missing scope", EnvScope, "scope is required"},
		{"missing org id", EnvOrgID, "organisation ID is required"},
		{"missing regions", EnvRegions, "region is required"},
		{"missing safe label key", EnvSafeLabelKey, "safe label key is required"},
		{"missing safe label value", EnvSafeLabelValue, "safe label value is required"},
		{"missing output", EnvOutput, "output is required"},
		{"missing webhook", EnvWebhookURL, "webhook URL is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			delete(env, tc.unset)
			cfg, err := Load("", envFrom(env))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			err = cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.expect) {
				t.Errorf("error %q does not contain %q", err, tc.expect)
			}
		})
	}
}

func TestValidateFolderScopeWithoutIDs(t *testing.T) {
	env := validEnv()
	env[EnvScope] = ScopeFolder
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "at least one folder ID") {
		t.Fatalf("expected folder-ID error, got %v", err)
	}
}

func TestValidatePrometheusRejected(t *testing.T) {
	env := validEnv()
	env[EnvOutput] = OutputPrometheus
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "prometheus is not available in v1") {
		t.Fatalf("expected prometheus deferred error, got %v", err)
	}
}

func TestValidateUnknownOutput(t *testing.T) {
	env := validEnv()
	env[EnvOutput] = "carrier-pigeon"
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for unknown output")
	}
}

func TestValidateListsAllProblems(t *testing.T) {
	cfg, err := Load("", envFrom(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"scope is required", "organisation ID is required", "region is required", "output is required"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should list every problem, missing %q in %q", want, msg)
		}
	}
}

func TestValidatePartialS3(t *testing.T) {
	// Every subset of the five S3 fields that is not the full set must be
	// rejected.
	all := map[string]string{
		EnvS3Endpoint:  "https://s3.example",
		EnvS3Region:    "eu01",
		EnvS3AccessKey: "ak",
		EnvS3SecretKey: "sk",
		EnvS3Bucket:    "bucket",
	}
	keys := []string{EnvS3Endpoint, EnvS3Region, EnvS3AccessKey, EnvS3SecretKey, EnvS3Bucket}
	for n := 1; n < 5; n++ {
		t.Run("partial of size "+itoa(n), func(t *testing.T) {
			env := validEnv()
			for i, k := range keys {
				if i < n {
					env[k] = all[k]
				}
			}
			cfg, err := Load("", envFrom(env))
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "partial S3") {
				t.Fatalf("expected partial-S3 error, got %v", err)
			}
		})
	}
	// Full set passes.
	env := validEnv()
	for k, v := range all {
		env[k] = v
	}
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("complete S3 config must validate: %v", err)
	}
}

func TestValidateCallbackRequires(t *testing.T) {
	base := func(extra map[string]string) *Config {
		env := validEnv()
		env[EnvCallbackURL] = "https://cb.example"
		for k, v := range extra {
			env[k] = v
		}
		cfg, err := Load("", envFrom(env))
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	full := map[string]string{
		EnvCallbackSecret:      "sec",
		EnvWhitelistSecretPath: "secret/wl",
		EnvSMURL:               "https://sm.example",
		EnvSMUsername:          "u",
		EnvSMPasswrd:           "p",
	}
	if err := base(full).Validate(); err != nil {
		t.Fatalf("complete callback config must validate: %v", err)
	}

	cases := map[string]string{
		"missing secret":      EnvCallbackSecret,
		"missing wl path":     EnvWhitelistSecretPath,
		"missing sm url":      EnvSMURL,
		"missing sm username": EnvSMUsername,
		"missing sm password": EnvSMPasswrd,
	}
	for name, drop := range cases {
		t.Run(name, func(t *testing.T) {
			m := make(map[string]string, len(full))
			for k, v := range full {
				if k != drop {
					m[k] = v
				}
			}
			if err := base(m).Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	t.Run("bad port", func(t *testing.T) {
		m := map[string]string{EnvCallbackPort: "99999"}
		m[EnvCallbackSecret] = "sec"
		m[EnvWhitelistSecretPath] = "secret/wl"
		m[EnvSMURL] = "https://sm.example"
		m[EnvSMUsername] = "u"
		m[EnvSMPasswrd] = "p"
		err := base(m).Validate()
		if err == nil || !strings.Contains(err.Error(), "callback port") {
			t.Fatalf("expected port error, got %v", err)
		}
	})
}

func TestValidateSanityBounds(t *testing.T) {
	cases := []struct {
		name   string
		env    map[string]string
		expect string
	}{
		{"max age 1", setEnv(validEnv(), EnvMaxAgeDays, "1"), "maxAgeDays must be at least 7"},
		{"sna age 3", setEnv(validEnv(), EnvSNAMaxAgeDays, "3"), "snaMaxAgeDays must be at least 7"},
		{"grace 1h", setEnv(validEnv(), EnvGracePeriod, "1h"), "gracePeriod must be at least 2h"},
		{"grace 0", setEnv(validEnv(), EnvGracePeriod, "0s"), "gracePeriod must be at least 2h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load("", envFrom(tc.env))
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("expected %q, got %v", tc.expect, err)
			}
		})
	}
}

func TestValidateBoundaryValuesAccepted(t *testing.T) {
	env := validEnv()
	env[EnvMaxAgeDays] = "7"
	env[EnvSNAMaxAgeDays] = "7"
	env[EnvGracePeriod] = "2h"
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("boundary values must validate: %v", err)
	}
	if cfg.GracePeriod != 2*time.Hour {
		t.Errorf("GracePeriod = %s", cfg.GracePeriod)
	}
}

func TestValidateBadWebhookURL(t *testing.T) {
	env := validEnv()
	env[EnvWebhookURL] = "not a url"
	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "webhook URL") {
		t.Fatalf("expected webhook URL error, got %v", err)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
