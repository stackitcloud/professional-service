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
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func envFrom(m map[string]string) EnvLookup {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		EnvScope:          ScopeOrganisation,
		EnvOrgID:          "org-1",
		EnvRegions:        "eu01",
		EnvMaxAgeDays:     "90",
		EnvSNAMaxAgeDays:  "60",
		EnvSafeLabelKey:   "do-not-delete",
		EnvSafeLabelValue: "true",
		EnvOutput:         OutputGoogleChat,
		EnvWebhookURL:     "https://chat.example/webhook",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("", envFrom(validEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.DryRun {
		t.Error("DryRun default must be true")
	}
	if cfg.CostAnomalyThresholdPct != DefaultCostAnomalyThresholdPct {
		t.Errorf("threshold default = %v", cfg.CostAnomalyThresholdPct)
	}
	if cfg.VolumeCostEurPerGB != DefaultVolumeCostEurPerGB {
		t.Errorf("volume rate default = %v", cfg.VolumeCostEurPerGB)
	}
	if cfg.PublicIPCostEurPerMonth != DefaultPublicIPCostEurPerMonth {
		t.Errorf("IP rate default = %v", cfg.PublicIPCostEurPerMonth)
	}
	if cfg.CallbackPort != DefaultCallbackPort {
		t.Errorf("callback port default = %d", cfg.CallbackPort)
	}
	if cfg.GracePeriod != DefaultGracePeriod {
		t.Errorf("grace default = %s", cfg.GracePeriod)
	}
}

func TestLoadEnvValues(t *testing.T) {
	env := validEnv()
	env[EnvDryRun] = "false"
	env[EnvGracePeriod] = "72h"
	env[EnvCostAnomalyThresholdPct] = "33.5"
	env[EnvCallbackURL] = "https://cb.example"
	env[EnvCallbackPort] = "9091"
	env[EnvCallbackSecret] = "s3cr3t"
	env[EnvWhitelistSecretPath] = "secret/wl"
	env[EnvSMURL] = "https://sm.example"
	env[EnvSMUsername] = "u"
	env[EnvSMPasswrd] = "p"
	env[EnvWhitelistProjects] = "p1, p2"
	env[EnvWhitelistPublicIPs] = "ip1"

	cfg, err := Load("", envFrom(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DryRun {
		t.Error("DryRun should be false")
	}
	if cfg.GracePeriod != 72*time.Hour {
		t.Errorf("GracePeriod = %s", cfg.GracePeriod)
	}
	if cfg.CostAnomalyThresholdPct != 33.5 {
		t.Errorf("threshold = %v", cfg.CostAnomalyThresholdPct)
	}
	if !reflect.DeepEqual(cfg.Regions, []string{"eu01"}) {
		t.Errorf("Regions = %v", cfg.Regions)
	}
	if !reflect.DeepEqual(cfg.Whitelist.Projects, []string{"p1", "p2"}) {
		t.Errorf("Whitelist.Projects = %v", cfg.Whitelist.Projects)
	}
	if !reflect.DeepEqual(cfg.Whitelist.PublicIPs, []string{"ip1"}) {
		t.Errorf("Whitelist.PublicIPs = %v", cfg.Whitelist.PublicIPs)
	}
	if cfg.S3.SetCount() != 0 {
		t.Error("S3 must be empty when env unset")
	}
}

func TestLoadParseErrors(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"bad max age", setEnv(validEnv(), EnvMaxAgeDays, "abc")},
		{"bad dry run", setEnv(validEnv(), EnvDryRun, "notabool")},
		{"bad grace period", setEnv(validEnv(), EnvGracePeriod, "8x")},
		{"bad threshold", setEnv(validEnv(), EnvCostAnomalyThresholdPct, "xyz")},
		{"bad port", setEnv(validEnv(), EnvCallbackPort, "nope")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load("", envFrom(tc.env)); err == nil {
				t.Error("expected parse error")
			}
		})
	}
}

func setEnv(base map[string]string, k, v string) map[string]string {
	m := make(map[string]string, len(base)+1)
	for kk, vv := range base {
		m[kk] = vv
	}
	m[k] = v
	return m
}

func TestLoadEnvOverYAML(t *testing.T) {
	yaml := []byte(`
scope: folder
orgId: yaml-org
regions: [eu01]
maxAgeDays: 30
snaMaxAgeDays: 30
safeLabelKey: yaml-key
safeLabelValue: yaml-value
output: slack
webhookUrl: https://yaml.example/hook
gracePeriod: 24h
whitelist:
  projects: [yaml-proj]
  volumes: [yaml-vol]
`)
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatal(err)
	}
	env := validEnv()
	delete(env, EnvOrgID) // let the YAML value win
	env[EnvScope] = "folder"
	env[EnvFolderIDs] = "f1,f2"
	env[EnvMaxAgeDays] = "45"
	env[EnvOutput] = OutputTeams
	env[EnvWhitelistProjects] = "env-proj"

	cfg, err := Load(path, envFrom(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Scope != "folder" || len(cfg.FolderIDs) != 2 {
		t.Errorf("folder scope not applied: %+v", cfg)
	}
	if cfg.OrgID != "yaml-org" {
		t.Errorf("OrgID from YAML = %q (env not set, YAML must win over default)", cfg.OrgID)
	}
	if cfg.MaxAgeDays != 45 {
		t.Errorf("MaxAgeDays = %d, want env 45 over YAML 30", cfg.MaxAgeDays)
	}
	if cfg.Output != OutputTeams {
		t.Errorf("Output = %q, want env teams over YAML slack", cfg.Output)
	}
	if cfg.GracePeriod != 24*time.Hour {
		t.Errorf("GracePeriod = %s, want YAML 24h", cfg.GracePeriod)
	}
	if cfg.WebhookURL != "https://chat.example/webhook" {
		t.Errorf("WebhookURL = %q, want env value (env wins over YAML)", cfg.WebhookURL)
	}
	if !reflect.DeepEqual(cfg.Whitelist.Projects, []string{"yaml-proj", "env-proj"}) {
		t.Errorf("whitelist union = %v", cfg.Whitelist.Projects)
	}
}

func TestLoadYAMLFileMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml"), envFrom(validEnv())); err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoadYAMLInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("scope: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, envFrom(validEnv())); err == nil {
		t.Error("expected YAML parse error")
	}
}

func TestWhitelistMergeDedup(t *testing.T) {
	base := Whitelist{Projects: []string{"a", "b"}}
	extra := Whitelist{Projects: []string{" b ", "c"}}
	merged := MergeWhitelist(base, extra)
	if !reflect.DeepEqual(merged.Projects, []string{"a", "b", "c"}) {
		t.Errorf("merged = %v", merged.Projects)
	}
}

func TestS3ConfigSetCount(t *testing.T) {
	if (S3Config{Endpoint: "e"}).SetCount() != 1 {
		t.Error("SetCount with one field")
	}
	if (S3Config{Endpoint: "e", Region: "r", AccessKey: "a", SecretKey: "s", Bucket: "b"}).SetCount() != 5 {
		t.Error("SetCount with five fields")
	}
}
