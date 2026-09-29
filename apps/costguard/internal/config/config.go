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

// Package config loads and validates the costguard configuration: one
// YAML file (written by Terraform through cloud-init) plus the webhook URL,
// which is a secret and therefore only read from the environment. Unknown
// YAML keys are rejected, so a typo such as "skips:" can never silently
// switch off a protection.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// Environment variables.
const (
	EnvConfigPath = "COSTGUARD_CONFIG"
	EnvWebhookURL = "COSTGUARD_WEBHOOK_URL"
	EnvLogLevel   = "LOG_LEVEL"
)

// DefaultConfigPath is where cloud-init writes the configuration.
const DefaultConfigPath = "/etc/costguard/config.yaml"

// Output values.
const (
	OutputGoogleChat = "googlechat"
	OutputSlack      = "slack"
	OutputTeams      = "teams"
)

// Defaults.
const (
	DefaultWarnEmptyAfterDays = 30
	// A public IPv4 per month (STACKIT price list v1.0.43, 08/04/2026,
	// net).
	DefaultPublicIPMonthlyEUR = 2.92
	// Block storage capacity per GB and month (same list, 0.0000907638
	// EUR/h x 720 h). The per-disk performance class fee comes on top and
	// is not counted.
	DefaultVolumeGBMonthlyEUR = 0.065
	DefaultPortalURL          = "https://portal.stackit.cloud"
	// DefaultTimeZone is the zone the messages show times in.
	DefaultTimeZone = "Europe/Berlin"
)

// DefaultRegions is used when the config lists no regions: only eu01.
// Installs that use other regions must list every region they use.
var DefaultRegions = []string{"eu01"}

// Selection names folders and projects by ID or by name.
type Selection struct {
	Folders  []string `yaml:"folders"`
	Projects []string `yaml:"projects"`
}

// Empty reports whether the selection names nothing.
func (s Selection) Empty() bool {
	return len(s.Folders) == 0 && len(s.Projects) == 0
}

// Prices feed the savings estimate.
type Prices struct {
	PublicIPMonthlyEUR float64 `yaml:"publicIpMonthlyEur"`
	VolumeGBMonthlyEUR float64 `yaml:"volumeGbMonthlyEur"`
}

// Report converts the prices for the report package.
func (p Prices) Report() report.Prices {
	return report.Prices{PublicIPMonthlyEUR: p.PublicIPMonthlyEUR, VolumeGBMonthlyEUR: p.VolumeGBMonthlyEUR}
}

// Config is the complete costguard configuration.
type Config struct {
	// OrganizationID is needed for the Cost API and the network-area
	// check, and it is the scope when Scope is empty.
	OrganizationID string `yaml:"organizationId"`
	// Scope limits the run to these folders and projects. Empty means the
	// whole organization. Every entry must match exactly one container.
	Scope Selection `yaml:"scope"`
	// Skip lists folders and projects that are never scanned. A name that
	// matches several containers skips all of them; an entry that matches
	// nothing blocks all writes in the run.
	Skip    Selection `yaml:"skip"`
	Regions []string  `yaml:"regions"`
	Output  string    `yaml:"output"`
	// WarnEmptyAfterDays is the minimum age of an empty project or network
	// area before the report warns about it. 0 switches these warnings off
	// (no cost data needed, e.g. for installs without organization-wide
	// cost access).
	WarnEmptyAfterDays int `yaml:"warnEmptyAfterDays"`
	// DeleteEnabled says whether the delete feature is on. The flag and
	// delete runs refuse to work without it, and the read-only runs word
	// their messages by it.
	DeleteEnabled bool `yaml:"deleteEnabled"`
	// DeleteRunAt is when the delete run happens, as shown in the messages,
	// e.g. "Tuesday 08:00 (Europe/Berlin)". Required when delete is on.
	// Terraform writes it from the same schedule as the timer.
	DeleteRunAt string `yaml:"deleteRunAt"`
	// ReportRunAt is when the report run happens, e.g. "Monday 08:00
	// (Europe/Berlin)". Optional; the messages then just say "the next
	// report run".
	ReportRunAt string `yaml:"reportRunAt"`
	// TimeZone is the IANA zone the messages show times in.
	TimeZone  string `yaml:"timeZone"`
	Prices    Prices `yaml:"prices"`
	PortalURL string `yaml:"portalUrl"`

	// WebhookURL comes from COSTGUARD_WEBHOOK_URL only.
	WebhookURL string `yaml:"-"`
}

// EnvLookup abstracts os.Getenv for tests.
type EnvLookup func(string) string

// Load reads the YAML file (path argument, then COSTGUARD_CONFIG, then
// DefaultConfigPath), applies defaults and reads the webhook URL from the
// environment. It does not validate; call Validate.
func Load(path string, env EnvLookup) (*Config, error) {
	path = resolvePath(path, env)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	cfg.WebhookURL = env(EnvWebhookURL)
	return cfg, nil
}

// LoadFromOS is Load with os.Getenv.
func LoadFromOS(path string) (*Config, error) {
	return Load(path, os.Getenv)
}

// resolvePath picks the config file: the argument, then COSTGUARD_CONFIG,
// then DefaultConfigPath.
func resolvePath(path string, env EnvLookup) string {
	if path == "" {
		path = env(EnvConfigPath)
	}
	if path == "" {
		path = DefaultConfigPath
	}
	return path
}

// Chat returns where to report a configuration that cannot be used: the
// output from the file and the webhook URL from the environment. It reads
// the file leniently, so unknown keys and invalid values elsewhere do not
// matter. ok is false when these two settings are unusable themselves;
// then the problem can only go to the log.
func Chat(path string, env EnvLookup) (output, webhookURL string, ok bool) {
	data, err := os.ReadFile(resolvePath(path, env))
	if err != nil {
		return "", "", false
	}
	var lenient struct {
		Output string `yaml:"output"`
	}
	if err := yaml.Unmarshal(data, &lenient); err != nil {
		return "", "", false
	}
	webhookURL = env(EnvWebhookURL)
	switch lenient.Output {
	case OutputGoogleChat, OutputSlack, OutputTeams:
		return lenient.Output, webhookURL, secureURL(webhookURL)
	}
	return "", "", false
}

// ChatFromOS is Chat with os.Getenv.
func ChatFromOS(path string) (output, webhookURL string, ok bool) {
	return Chat(path, os.Getenv)
}

// Parse decodes a YAML document strictly (unknown keys are errors) on top
// of the defaults.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{
		WarnEmptyAfterDays: DefaultWarnEmptyAfterDays,
		TimeZone:           DefaultTimeZone,
		PortalURL:          DefaultPortalURL,
		Prices: Prices{
			PublicIPMonthlyEUR: DefaultPublicIPMonthlyEUR,
			VolumeGBMonthlyEUR: DefaultVolumeGBMonthlyEUR,
		},
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	// A second document (after "---") would be ignored silently: refuse it.
	var extra yaml.Node
	if err := dec.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	} else if err == nil && !emptyDocument(&extra) {
		return nil, errors.New("the file contains more than one YAML document (a second one after ---); put all settings into one")
	}
	if len(cfg.Regions) == 0 {
		cfg.Regions = append([]string(nil), DefaultRegions...)
	}
	return cfg, nil
}

// Location is the time zone of the messages. It falls back to UTC for a
// zone that Validate would refuse.
func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil || c.TimeZone == "" {
		return time.UTC
	}
	return loc
}

// emptyDocument reports whether a decoded YAML document has no content, as
// after a trailing "---".
func emptyDocument(n *yaml.Node) bool {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	return n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsID reports whether a scope or skip entry is an ID (a UUID) rather
// than a name.
func IsID(s string) bool {
	return uuidPattern.MatchString(s)
}
