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

// Package config loads and validates costguard configuration from a YAML
// file overlaid with environment variables (env wins). No
// configuration value — including secrets — is hardcoded; everything is
// supplied at runtime.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Environment variable names.
const (
	EnvScope                   = "COSTGUARD_SCOPE"
	EnvOrgID                   = "COSTGUARD_ORG_ID"
	EnvFolderIDs               = "COSTGUARD_FOLDER_IDS"
	EnvRegions                 = "COSTGUARD_REGIONS"
	EnvMaxAgeDays              = "COSTGUARD_MAX_AGE_DAYS"
	EnvSNAMaxAgeDays           = "COSTGUARD_SNA_MAX_AGE_DAYS"
	EnvSafeLabelKey            = "COSTGUARD_SAFE_LABEL_KEY"
	EnvSafeLabelValue          = "COSTGUARD_SAFE_LABEL_VALUE"
	EnvOutput                  = "COSTGUARD_OUTPUT"
	EnvWebhookURL              = "COSTGUARD_WEBHOOK_URL"
	EnvDryRun                  = "COSTGUARD_DRY_RUN"
	EnvCostAnomalyThresholdPct = "COSTGUARD_COST_ANOMALY_THRESHOLD_PCT"
	EnvVolumeCostEurPerGB      = "COSTGUARD_VOLUME_COST_EUR_PER_GB"
	EnvPublicIPCostEurPerMonth = "COSTGUARD_PUBLIC_IP_COST_EUR_PER_MONTH"
	EnvCallbackURL             = "COSTGUARD_CALLBACK_URL"
	EnvCallbackPort            = "COSTGUARD_CALLBACK_PORT"
	EnvCallbackSecret          = "COSTGUARD_CALLBACK_SECRET"
	EnvGracePeriod             = "COSTGUARD_GRACE_PERIOD"
	EnvWhitelistSecretPath     = "COSTGUARD_WHITELIST_SECRET_PATH"
	EnvSMURL                   = "COSTGUARD_SM_URL"
	EnvSMUsername              = "COSTGUARD_SM_USERNAME"
	EnvSMPasswrd               = "COSTGUARD_SM_PASSWORD"
	EnvS3Endpoint              = "COSTGUARD_S3_ENDPOINT"
	EnvS3Region                = "COSTGUARD_S3_REGION"
	EnvS3AccessKey             = "COSTGUARD_S3_ACCESS_KEY"
	EnvS3SecretKey             = "COSTGUARD_S3_SECRET_KEY"
	EnvS3Bucket                = "COSTGUARD_S3_BUCKET"
	EnvWhitelistProjects       = "COSTGUARD_WHITELIST_PROJECTS"
	EnvWhitelistFolders        = "COSTGUARD_WHITELIST_FOLDERS"
	EnvWhitelistVolumes        = "COSTGUARD_WHITELIST_VOLUMES"
	EnvWhitelistPublicIPs      = "COSTGUARD_WHITELIST_PUBLIC_IPS"
	EnvWhitelistSkipVolScan    = "COSTGUARD_WHITELIST_SKIP_VOLUME_SCAN_PROJECTS"
	EnvWhitelistSkipIPScan     = "COSTGUARD_WHITELIST_SKIP_PUBLIC_IP_SCAN_PROJECTS"
	EnvConfigPath              = "COSTGUARD_CONFIG"
	EnvLogLevel                = "LOG_LEVEL"
)

// Scope values.
const (
	ScopeOrganisation = "organisation"
	ScopeFolder       = "folder"
)

// Output values. OutputPrometheus is rejected at startup (not available in v1).
const (
	OutputGoogleChat = "googlechat"
	OutputSlack      = "slack"
	OutputTeams      = "teams"
	OutputPrometheus = "prometheus"
)

// Defaults.
const (
	DefaultCostAnomalyThresholdPct = 20.0
	// Standard STACKIT block storage price; verify the current price before
	// relying on it for savings figures.
	DefaultVolumeCostEurPerGB = 0.0619
	// ~0.0066 EUR/h x 730 h public IP reservation; verify the current price.
	DefaultPublicIPCostEurPerMonth = 4.82
	DefaultCallbackPort            = 8080
	DefaultGracePeriod             = 8 * time.Hour
)

// S3Config is the optional chart-upload target. All five fields are
// required together; a partial S3 config is a startup validation error.
type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	Bucket    string `yaml:"bucket"`
}

// Complete reports whether all five S3 fields are set.
func (s S3Config) Complete() bool {
	return s.Endpoint != "" && s.Region != "" && s.AccessKey != "" && s.SecretKey != "" && s.Bucket != ""
}

// SetCount returns how many of the five S3 fields are set.
func (s S3Config) SetCount() int {
	n := 0
	for _, v := range []string{s.Endpoint, s.Region, s.AccessKey, s.SecretKey, s.Bucket} {
		if v != "" {
			n++
		}
	}
	return n
}

// Whitelist holds the static (deploy-time) whitelist: the union of the
// YAML file lists and the env CSV variables. At runtime it is
// merged with the Secrets Manager entry by internal/whitelist — a hit in
// any source protects.
type Whitelist struct {
	Projects                 []string `yaml:"projects"`
	Folders                  []string `yaml:"folders"`
	Volumes                  []string `yaml:"volumes"`
	PublicIPs                []string `yaml:"publicIps"`
	SkipVolumeScanProjects   []string `yaml:"skipVolumeScanProjects"`
	SkipPublicIPScanProjects []string `yaml:"skipPublicIpScanProjects"`
}

// SecretsManagerConfig locates the dedicated Secrets Manager instance that
// hosts the shared whitelist.
type SecretsManagerConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Config is the complete costguard configuration.
type Config struct {
	Scope                   string               `yaml:"scope"`
	OrgID                   string               `yaml:"orgId"`
	FolderIDs               []string             `yaml:"folderIds"`
	Regions                 []string             `yaml:"regions"`
	MaxAgeDays              int                  `yaml:"maxAgeDays"`
	SNAMaxAgeDays           int                  `yaml:"snaMaxAgeDays"`
	SafeLabelKey            string               `yaml:"safeLabelKey"`
	SafeLabelValue          string               `yaml:"safeLabelValue"`
	Output                  string               `yaml:"output"`
	WebhookURL              string               `yaml:"webhookUrl"`
	DryRun                  bool                 `yaml:"dryRun"`
	CostAnomalyThresholdPct float64              `yaml:"costAnomalyThresholdPct"`
	VolumeCostEurPerGB      float64              `yaml:"volumeCostEurPerGB"`
	PublicIPCostEurPerMonth float64              `yaml:"publicIPCostEurPerMonth"`
	CallbackURL             string               `yaml:"callbackUrl"`
	CallbackPort            int                  `yaml:"callbackPort"`
	CallbackSecret          string               `yaml:"callbackSecret"`
	GracePeriod             time.Duration        `yaml:"-"`
	WhitelistSecretPath     string               `yaml:"whitelistSecretPath"`
	SecretsManager          SecretsManagerConfig `yaml:"secretsManager"`
	S3                      S3Config             `yaml:"s3"`
	Whitelist               Whitelist            `yaml:"whitelist"`
}

// fileConfig mirrors Config with pointer fields so that "absent in the
// YAML file" can be distinguished from "zero value". The overlay is
// hand-rolled: the field set is fixed and flat, which
// keeps one fewer dependency than a config library.
type fileConfig struct {
	Scope                   *string               `yaml:"scope"`
	OrgID                   *string               `yaml:"orgId"`
	FolderIDs               []string              `yaml:"folderIds"`
	Regions                 []string              `yaml:"regions"`
	MaxAgeDays              *int                  `yaml:"maxAgeDays"`
	SNAMaxAgeDays           *int                  `yaml:"snaMaxAgeDays"`
	SafeLabelKey            *string               `yaml:"safeLabelKey"`
	SafeLabelValue          *string               `yaml:"safeLabelValue"`
	Output                  *string               `yaml:"output"`
	WebhookURL              *string               `yaml:"webhookUrl"`
	DryRun                  *bool                 `yaml:"dryRun"`
	CostAnomalyThresholdPct *float64              `yaml:"costAnomalyThresholdPct"`
	VolumeCostEurPerGB      *float64              `yaml:"volumeCostEurPerGB"`
	PublicIPCostEurPerMonth *float64              `yaml:"publicIPCostEurPerMonth"`
	CallbackURL             *string               `yaml:"callbackUrl"`
	CallbackPort            *int                  `yaml:"callbackPort"`
	CallbackSecret          *string               `yaml:"callbackSecret"`
	GracePeriod             *string               `yaml:"gracePeriod"`
	WhitelistSecretPath     *string               `yaml:"whitelistSecretPath"`
	SecretsManager          *SecretsManagerConfig `yaml:"secretsManager"`
	S3                      *S3Config             `yaml:"s3"`
	Whitelist               *Whitelist            `yaml:"whitelist"`
}

// EnvLookup abstracts os.Getenv so tests can inject deterministic
// environments.
type EnvLookup func(string) string

// Load builds the configuration: defaults, then the YAML file (path from
// the --config flag or COSTGUARD_CONFIG), then environment variables,
// which take precedence.
func Load(configPath string, env EnvLookup) (*Config, error) {
	cfg := defaults()

	path := configPath
	if path == "" {
		path = env(EnvConfigPath)
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", path, err)
		}
		var fc fileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", path, err)
		}
		if err := cfg.applyFile(&fc); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", path, err)
		}
	}
	if err := cfg.applyEnv(env); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFromOS is Load with os.Getenv.
func LoadFromOS(configPath string) (*Config, error) {
	return Load(configPath, os.Getenv)
}

func defaults() *Config {
	return &Config{
		DryRun:                  true,
		CostAnomalyThresholdPct: DefaultCostAnomalyThresholdPct,
		VolumeCostEurPerGB:      DefaultVolumeCostEurPerGB,
		PublicIPCostEurPerMonth: DefaultPublicIPCostEurPerMonth,
		CallbackPort:            DefaultCallbackPort,
		GracePeriod:             DefaultGracePeriod,
	}
}

func (c *Config) applyFile(fc *fileConfig) error {
	if fc.Scope != nil {
		c.Scope = *fc.Scope
	}
	if fc.OrgID != nil {
		c.OrgID = *fc.OrgID
	}
	if fc.FolderIDs != nil {
		c.FolderIDs = fc.FolderIDs
	}
	if fc.Regions != nil {
		c.Regions = fc.Regions
	}
	if fc.MaxAgeDays != nil {
		c.MaxAgeDays = *fc.MaxAgeDays
	}
	if fc.SNAMaxAgeDays != nil {
		c.SNAMaxAgeDays = *fc.SNAMaxAgeDays
	}
	if fc.SafeLabelKey != nil {
		c.SafeLabelKey = *fc.SafeLabelKey
	}
	if fc.SafeLabelValue != nil {
		c.SafeLabelValue = *fc.SafeLabelValue
	}
	if fc.Output != nil {
		c.Output = *fc.Output
	}
	if fc.WebhookURL != nil {
		c.WebhookURL = *fc.WebhookURL
	}
	if fc.DryRun != nil {
		c.DryRun = *fc.DryRun
	}
	if fc.CostAnomalyThresholdPct != nil {
		c.CostAnomalyThresholdPct = *fc.CostAnomalyThresholdPct
	}
	if fc.VolumeCostEurPerGB != nil {
		c.VolumeCostEurPerGB = *fc.VolumeCostEurPerGB
	}
	if fc.PublicIPCostEurPerMonth != nil {
		c.PublicIPCostEurPerMonth = *fc.PublicIPCostEurPerMonth
	}
	if fc.CallbackURL != nil {
		c.CallbackURL = *fc.CallbackURL
	}
	if fc.CallbackPort != nil {
		c.CallbackPort = *fc.CallbackPort
	}
	if fc.CallbackSecret != nil {
		c.CallbackSecret = *fc.CallbackSecret
	}
	if fc.GracePeriod != nil {
		d, err := time.ParseDuration(*fc.GracePeriod)
		if err != nil {
			return fmt.Errorf("gracePeriod: %w", err)
		}
		c.GracePeriod = d
	}
	if fc.WhitelistSecretPath != nil {
		c.WhitelistSecretPath = *fc.WhitelistSecretPath
	}
	if fc.SecretsManager != nil {
		c.SecretsManager = *fc.SecretsManager
	}
	if fc.S3 != nil {
		c.S3 = *fc.S3
	}
	if fc.Whitelist != nil {
		c.Whitelist = MergeWhitelist(c.Whitelist, *fc.Whitelist)
	}
	return nil
}

// applyEnv overlays every set environment variable on the config (env
// wins). Empty strings are treated as unset.
func (c *Config) applyEnv(env EnvLookup) error {
	if v := env(EnvScope); v != "" {
		c.Scope = v
	}
	if v := env(EnvOrgID); v != "" {
		c.OrgID = v
	}
	if v := env(EnvFolderIDs); v != "" {
		c.FolderIDs = splitCSV(v)
	}
	if v := env(EnvRegions); v != "" {
		c.Regions = splitCSV(v)
	}
	if v := env(EnvMaxAgeDays); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvMaxAgeDays, err)
		}
		c.MaxAgeDays = n
	}
	if v := env(EnvSNAMaxAgeDays); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvSNAMaxAgeDays, err)
		}
		c.SNAMaxAgeDays = n
	}
	if v := env(EnvSafeLabelKey); v != "" {
		c.SafeLabelKey = v
	}
	if v := env(EnvSafeLabelValue); v != "" {
		c.SafeLabelValue = v
	}
	if v := env(EnvOutput); v != "" {
		c.Output = v
	}
	if v := env(EnvWebhookURL); v != "" {
		c.WebhookURL = v
	}
	if v := env(EnvDryRun); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvDryRun, err)
		}
		c.DryRun = b
	}
	if v := env(EnvCostAnomalyThresholdPct); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvCostAnomalyThresholdPct, err)
		}
		c.CostAnomalyThresholdPct = f
	}
	if v := env(EnvVolumeCostEurPerGB); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvVolumeCostEurPerGB, err)
		}
		c.VolumeCostEurPerGB = f
	}
	if v := env(EnvPublicIPCostEurPerMonth); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvPublicIPCostEurPerMonth, err)
		}
		c.PublicIPCostEurPerMonth = f
	}
	if v := env(EnvCallbackURL); v != "" {
		c.CallbackURL = v
	}
	if v := env(EnvCallbackPort); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvCallbackPort, err)
		}
		c.CallbackPort = n
	}
	if v := env(EnvCallbackSecret); v != "" {
		c.CallbackSecret = v
	}
	if v := env(EnvGracePeriod); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("%s: %w", EnvGracePeriod, err)
		}
		c.GracePeriod = d
	}
	if v := env(EnvWhitelistSecretPath); v != "" {
		c.WhitelistSecretPath = v
	}
	if v := env(EnvSMURL); v != "" {
		c.SecretsManager.URL = v
	}
	if v := env(EnvSMUsername); v != "" {
		c.SecretsManager.Username = v
	}
	if v := env(EnvSMPasswrd); v != "" {
		c.SecretsManager.Password = v
	}
	if v := env(EnvS3Endpoint); v != "" {
		c.S3.Endpoint = v
	}
	if v := env(EnvS3Region); v != "" {
		c.S3.Region = v
	}
	if v := env(EnvS3AccessKey); v != "" {
		c.S3.AccessKey = v
	}
	if v := env(EnvS3SecretKey); v != "" {
		c.S3.SecretKey = v
	}
	if v := env(EnvS3Bucket); v != "" {
		c.S3.Bucket = v
	}
	if v := env(EnvWhitelistProjects); v != "" {
		c.Whitelist.Projects = union(c.Whitelist.Projects, splitCSV(v))
	}
	if v := env(EnvWhitelistFolders); v != "" {
		c.Whitelist.Folders = union(c.Whitelist.Folders, splitCSV(v))
	}
	if v := env(EnvWhitelistVolumes); v != "" {
		c.Whitelist.Volumes = union(c.Whitelist.Volumes, splitCSV(v))
	}
	if v := env(EnvWhitelistPublicIPs); v != "" {
		c.Whitelist.PublicIPs = union(c.Whitelist.PublicIPs, splitCSV(v))
	}
	if v := env(EnvWhitelistSkipVolScan); v != "" {
		c.Whitelist.SkipVolumeScanProjects = union(c.Whitelist.SkipVolumeScanProjects, splitCSV(v))
	}
	if v := env(EnvWhitelistSkipIPScan); v != "" {
		c.Whitelist.SkipPublicIPScanProjects = union(c.Whitelist.SkipPublicIPScanProjects, splitCSV(v))
	}
	return nil
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
