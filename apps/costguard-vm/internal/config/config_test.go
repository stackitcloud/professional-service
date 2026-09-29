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
	"strings"
	"testing"
	"time"
)

const orgID = "11111111-2222-3333-4444-555555555555"

func env(m map[string]string) EnvLookup {
	return func(k string) string { return m[k] }
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validConfig() *Config {
	cfg, err := Parse([]byte("organizationId: " + orgID + "\noutput: slack\n"))
	if err != nil {
		panic(err)
	}
	cfg.WebhookURL = "https://hooks.example.com/x"
	return cfg
}

func TestParseAppliesDefaults(t *testing.T) {
	cfg, err := Parse([]byte("organizationId: " + orgID + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.Regions, ",") != "eu01" {
		t.Errorf("regions = %v", cfg.Regions)
	}
	if cfg.WarnEmptyAfterDays != 30 || cfg.PortalURL != DefaultPortalURL || cfg.TimeZone != "Europe/Berlin" {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	// Delete is off unless the file says otherwise, and has no default time.
	if cfg.DeleteEnabled || cfg.DeleteRunAt != "" || cfg.ReportRunAt != "" {
		t.Errorf("delete defaults: %+v", cfg)
	}
	if cfg.Prices.PublicIPMonthlyEUR != DefaultPublicIPMonthlyEUR || cfg.Prices.VolumeGBMonthlyEUR != DefaultVolumeGBMonthlyEUR {
		t.Errorf("price defaults = %+v", cfg.Prices)
	}
	if p := cfg.Prices.Report(); p.PublicIPMonthlyEUR != DefaultPublicIPMonthlyEUR || p.VolumeGBMonthlyEUR != DefaultVolumeGBMonthlyEUR {
		t.Errorf("Report() = %+v", p)
	}
}

func TestParseKeepsDefaultsOfPartialNestedBlocks(t *testing.T) {
	cfg, err := Parse([]byte("prices:\n  publicIpMonthlyEur: 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prices.PublicIPMonthlyEUR != 5 || cfg.Prices.VolumeGBMonthlyEUR != DefaultVolumeGBMonthlyEUR {
		t.Errorf("prices = %+v", cfg.Prices)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse([]byte("skips:\n  projects: [a]\n"))
	if err == nil || !strings.Contains(err.Error(), "skips") {
		t.Fatalf("want unknown-key error, got %v", err)
	}
}

func TestParseRefusesASecondDocument(t *testing.T) {
	_, err := Parse([]byte("organizationId: " + orgID + "\n---\noutput: slack\n"))
	if err == nil || !strings.Contains(err.Error(), "more than one YAML document") {
		t.Fatalf("want second-document error, got %v", err)
	}
	for _, ok := range []string{
		"organizationId: " + orgID + "\n---\n",    // trailing separator
		"---\norganizationId: " + orgID + "\n",    // leading separator
		"organizationId: " + orgID + "\n---\n~\n", // explicit empty document
	} {
		if _, err := Parse([]byte(ok)); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	if _, err := Parse([]byte("organizationId: " + orgID + "\n---\n[unclosed\n")); err == nil {
		t.Error("a broken second document must fail too")
	}
}

func TestChatReadsOnlyTheChatSettings(t *testing.T) {
	webhook := env(map[string]string{EnvWebhookURL: "https://hooks.example.com/x"})
	// Unknown keys and invalid values elsewhere do not matter.
	path := writeConfig(t, "output: teams\nskips: [a]\nwarnEmptyAfterDays: nope\n")
	if out, url, ok := Chat(path, webhook); !ok || out != OutputTeams || url != "https://hooks.example.com/x" {
		t.Errorf("Chat = %q %q %v", out, url, ok)
	}
	for name, c := range map[string]struct {
		content string
		env     EnvLookup
	}{
		"no output":    {"skips: [a]\n", webhook},
		"bad output":   {"output: email\n", webhook},
		"no webhook":   {"output: slack\n", env(nil)},
		"http webhook": {"output: slack\n", env(map[string]string{EnvWebhookURL: "http://hooks.example.com/x"})},
		"broken YAML":  {"output: [slack\n", webhook},
	} {
		if _, _, ok := Chat(writeConfig(t, c.content), c.env); ok {
			t.Errorf("%s: must not be usable", name)
		}
	}
	if _, _, ok := Chat(filepath.Join(t.TempDir(), "missing.yaml"), webhook); ok {
		t.Error("missing file: must not be usable")
	}
	t.Setenv(EnvConfigPath, path)
	t.Setenv(EnvWebhookURL, "https://hooks.example.com/y")
	if out, _, ok := ChatFromOS(""); !ok || out != OutputTeams {
		t.Errorf("ChatFromOS = %q %v", out, ok)
	}
}

func TestParseEmptyDocument(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OrganizationID != "" || len(cfg.Regions) != 1 {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestParseFullDocument(t *testing.T) {
	cfg, err := Parse([]byte(`
organizationId: ` + orgID + `
scope:
  folders: [Sandboxes]
  projects: [aaaaaaaa-0000-0000-0000-000000000001]
skip:
  folders: [Platform]
  projects: [prod-billing]
regions: [eu01]
output: teams
warnEmptyAfterDays: 45
deleteEnabled: true
deleteRunAt: "Tuesday 09:00"
reportRunAt: "Monday 08:00"
timeZone: UTC
portalUrl: https://portal.example
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scope.Folders[0] != "Sandboxes" || cfg.Skip.Projects[0] != "prod-billing" || cfg.Regions[0] != "eu01" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Output != OutputTeams || cfg.WarnEmptyAfterDays != 45 || !cfg.DeleteEnabled || cfg.DeleteRunAt != "Tuesday 09:00" ||
		cfg.ReportRunAt != "Monday 08:00" || cfg.TimeZone != "UTC" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Scope.Empty() || !(Selection{}).Empty() {
		t.Error("Selection.Empty is wrong")
	}
}

func TestLoadPathPrecedence(t *testing.T) {
	flagPath := writeConfig(t, "organizationId: "+orgID+"\noutput: slack\n")
	envPath := writeConfig(t, "organizationId: "+orgID+"\noutput: teams\n")

	cfg, err := Load(flagPath, env(map[string]string{EnvConfigPath: envPath, EnvWebhookURL: "https://h"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != OutputSlack || cfg.WebhookURL != "https://h" {
		t.Errorf("flag path must win: %+v", cfg)
	}
	cfg, err = Load("", env(map[string]string{EnvConfigPath: envPath}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != OutputTeams {
		t.Errorf("env path not used: %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), env(nil)); err == nil {
		t.Error("missing file must fail")
	}
	bad := writeConfig(t, "organizationId: [\n")
	if _, err := Load(bad, env(nil)); err == nil || !strings.Contains(err.Error(), "parsing config file") {
		t.Errorf("bad YAML: %v", err)
	}
}

func TestLoadFromOSUsesDefaultPath(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	_, err := LoadFromOS("")
	if err == nil || !strings.Contains(err.Error(), "reading config file") {
		t.Errorf("want read error for the default path, got %v", err)
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateListsAllProblems(t *testing.T) {
	cfg := &Config{
		OrganizationID:     "not-a-uuid",
		Scope:              Selection{Folders: []string{"a", "A"}, Projects: []string{" "}},
		Skip:               Selection{Folders: []string{"x", "x"}},
		Regions:            []string{"eu01", "eu01", "EU-1"},
		Output:             "prometheus",
		WebhookURL:         "not a url",
		WarnEmptyAfterDays: 1,
		DeleteEnabled:      true,
		DeleteRunAt:        " ",
		TimeZone:           "Mars/Olympus",
		Prices:             Prices{PublicIPMonthlyEUR: -1},
		PortalURL:          "portal",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{
		"organizationId must be a UUID",
		`scope.folders lists "A" twice`,
		"scope.projects contains an empty entry",
		`skip.folders lists "x" twice`,
		`region "eu01" is listed twice`,
		`region "EU-1"`,
		"output must be",
		"absolute http(s) URL",
		"warnEmptyAfterDays",
		"deleteRunAt is required when deleteEnabled is true",
		`timeZone must be an IANA time zone such as Europe/Berlin (got "Mars/Olympus")`,
		"prices",
		"portalUrl",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestValidateRequiredFields(t *testing.T) {
	cfg := validConfig()
	cfg.OrganizationID, cfg.Output, cfg.WebhookURL = "", "", ""
	err := cfg.Validate()
	for _, want := range []string{"organizationId is required", "output is required", "webhook URL is required"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestDeleteSettingsBelongTogether(t *testing.T) {
	for _, tc := range []struct {
		enabled bool
		runAt   string
		want    string
	}{
		{false, "", ""},
		{true, "Tuesday 08:00 (Europe/Berlin)", ""},
		{true, "", "deleteRunAt is required when deleteEnabled is true"},
		{false, "Tuesday 08:00", "deleteRunAt is set, but deleteEnabled is false"},
	} {
		cfg := validConfig()
		cfg.DeleteEnabled, cfg.DeleteRunAt = tc.enabled, tc.runAt
		err := cfg.Validate()
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%v %q: err = %v, want %q", tc.enabled, tc.runAt, err, tc.want)
		}
	}
}

func TestTimeZone(t *testing.T) {
	for zone, ok := range map[string]bool{"Europe/Berlin": true, "UTC": true, "": false, "Local": false, "Berlin": false} {
		cfg := validConfig()
		cfg.TimeZone = zone
		if err := cfg.Validate(); (err == nil) != ok {
			t.Errorf("%q: err = %v", zone, err)
		}
	}
	cfg := validConfig()
	if loc := cfg.Location(); loc.String() != "Europe/Berlin" {
		t.Errorf("Location = %v", loc)
	}
	cfg.TimeZone = "Mars/Olympus"
	if loc := cfg.Location(); loc != time.UTC {
		t.Errorf("an invalid zone falls back to UTC, got %v", loc)
	}
	cfg.TimeZone = ""
	if loc := cfg.Location(); loc != time.UTC {
		t.Errorf("an empty zone falls back to UTC, got %v", loc)
	}
}

func TestWarningsCanBeSwitchedOff(t *testing.T) {
	for days, ok := range map[int]bool{0: true, 7: true, 30: true, 6: false, -1: false} {
		cfg := validConfig()
		cfg.WarnEmptyAfterDays = days
		if err := cfg.Validate(); (err == nil) != ok {
			t.Errorf("%d: err = %v", days, err)
		}
	}
	cfg, err := Parse([]byte("warnEmptyAfterDays: 0\n"))
	if err != nil || cfg.WarnEmptyAfterDays != 0 {
		t.Errorf("an explicit 0 must override the default: %v %+v", err, cfg)
	}
}

func TestWebhookMustBeHTTPS(t *testing.T) {
	for url, ok := range map[string]bool{
		"https://hooks.example.com/x": true,
		"http://hooks.example.com/x":  false,
		"http://127.0.0.1:8080/x":     true,
		"http://localhost/x":          true,
		"http://[::1]:9/x":            true,
		"ftp://hooks.example.com/x":   false,
	} {
		cfg := validConfig()
		cfg.WebhookURL = url
		if err := cfg.Validate(); (err == nil) != ok {
			t.Errorf("%s: err = %v", url, err)
		}
	}
}

func TestIsID(t *testing.T) {
	if !IsID(orgID) || !IsID(strings.ToUpper(orgID)) {
		t.Error("UUID not recognised")
	}
	if IsID("prod-billing") || IsID(orgID+"x") {
		t.Error("name taken for an ID")
	}
}
