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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EnvConfigPath = "COSTGUARD_CONFIG"
	EnvWebhookURL = "COSTGUARD_WEBHOOK_URL"
	EnvLogLevel   = "LOG_LEVEL"
)

const DefaultConfigPath = "/etc/costguard/config.yaml"

const (
	OutputGoogleChat = "googlechat"
	OutputSlack      = "slack"
	OutputTeams      = "teams"
)

const (
	DefaultWarnEmptyAfterDays = 30
	DefaultPortalURL          = "https://portal.stackit.cloud"
	DefaultTimeZone           = "Europe/Berlin"
)

var DefaultRegions = []string{"eu01"}

type Selection struct {
	Folders  []string `yaml:"folders"`
	Projects []string `yaml:"projects"`
}

func (s Selection) Empty() bool {
	return len(s.Folders) == 0 && len(s.Projects) == 0
}

type Config struct {
	OrganizationID     string    `yaml:"organizationId"`
	Scope              Selection `yaml:"scope"`
	Skip               Selection `yaml:"skip"`
	Regions            []string  `yaml:"regions"`
	Output             string    `yaml:"output"`
	WarnEmptyAfterDays int       `yaml:"warnEmptyAfterDays"`
	ReportEnabled      bool      `yaml:"reportEnabled"`
	DeleteEnabled      bool      `yaml:"deleteEnabled"`
	DeleteRunAt        string    `yaml:"deleteRunAt"`
	ReportRunAt        string    `yaml:"reportRunAt"`
	TimeZone           string    `yaml:"timeZone"`
	PortalURL          string    `yaml:"portalUrl"`
	Budgets            *Budgets  `yaml:"budgets"`

	WebhookURL string `yaml:"-"`
}

type Budgets struct {
	Days   []string `yaml:"days"`
	Time   string   `yaml:"time"`
	Limits []Budget `yaml:"limits"`
}

var Weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

var dayNames = map[string]string{"Mon": "Monday", "Tue": "Tuesday", "Wed": "Wednesday", "Thu": "Thursday",
	"Fri": "Friday", "Sat": "Saturday", "Sun": "Sunday"}

func (c *Config) BudgetsRunAt() string {
	if c.Budgets == nil {
		return ""
	}
	return fmt.Sprintf("%s %s (%s)", DaysText(c.Budgets.Days), c.Budgets.Time, c.TimeZone)
}

func DaysText(days []string) string {
	var names []string
	var short []string
	for _, d := range Weekdays {
		for _, given := range days {
			if given == d {
				names = append(names, dayNames[d])
				short = append(short, d)
				break
			}
		}
	}
	switch strings.Join(short, ",") {
	case strings.Join(Weekdays, ","):
		return "every day"
	case "Mon,Tue,Wed,Thu,Fri":
		return "Monday to Friday"
	}
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

type Budget struct {
	Name         string  `yaml:"name"`
	Organization bool    `yaml:"organization"`
	Folder       string  `yaml:"folder"`
	Project      string  `yaml:"project"`
	MonthlyEUR   float64 `yaml:"monthlyEur"`
	Thresholds   []int   `yaml:"thresholds"`
}

type EnvLookup func(string) string

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

func LoadFromOS(path string) (*Config, error) {
	return Load(path, os.Getenv)
}

func resolvePath(path string, env EnvLookup) string {
	if path == "" {
		path = env(EnvConfigPath)
	}
	if path == "" {
		path = DefaultConfigPath
	}
	return path
}

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

func ChatFromOS(path string) (output, webhookURL string, ok bool) {
	return Chat(path, os.Getenv)
}

func Parse(data []byte) (*Config, error) {
	cfg := &Config{
		ReportEnabled:      true,
		WarnEmptyAfterDays: DefaultWarnEmptyAfterDays,
		TimeZone:           DefaultTimeZone,
		PortalURL:          DefaultPortalURL,
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
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

func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil || c.TimeZone == "" {
		return time.UTC
	}
	return loc
}

func emptyDocument(n *yaml.Node) bool {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	return n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsID(s string) bool {
	return uuidPattern.MatchString(s)
}
