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

package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// s3ConfigFor returns a complete S3 config pointing at the test server.
func s3ConfigFor(endpoint string) config.S3Config {
	return config.S3Config{
		Endpoint:  endpoint,
		Region:    "eu01",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		Bucket:    "costguard-charts",
	}
}

func TestAttachChartUploadsAndSetsURL(t *testing.T) {
	// Minimal S3 fake: accept the path-style PUT, answer 200.
	var gotPut bool
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			gotPut = true
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(s3.Close)

	cfg := testConfig(t)
	cfg.S3 = s3ConfigFor(s3.URL)

	rep := reportForChart()
	attachChart(context.Background(), cfg, testLogger(t), rep)

	if !gotPut {
		t.Fatal("S3 PUT was not received")
	}
	if rep.ChartURL == "" || !strings.Contains(rep.ChartURL, "costguard/") {
		t.Fatalf("ChartURL = %q, want a presigned URL under costguard/", rep.ChartURL)
	}
}

func TestAttachChartDegradesOnUploadFailure(t *testing.T) {
	// A closed port: the SDK client initialises fine, the PUT fails.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	cfg := testConfig(t)
	cfg.S3 = s3ConfigFor("http://127.0.0.1:" + itoa(port))

	rep := reportForChart()
	attachChart(context.Background(), cfg, testLogger(t), rep)

	if rep.ChartURL != "" {
		t.Fatalf("ChartURL = %q, want empty after upload failure", rep.ChartURL)
	}
}

func TestAttachChartSkippedWhenS3NotConfigured(t *testing.T) {
	cfg := testConfig(t)
	rep := reportForChart()
	attachChart(context.Background(), cfg, testLogger(t), rep)
	if rep.ChartURL != "" {
		t.Fatalf("ChartURL = %q, want empty when S3 is not configured", rep.ChartURL)
	}
}

func TestLoadWhitelistMergesStoreEntries(t *testing.T) {
	cfg := testConfig(t)
	cfg.WhitelistSecretPath = "secret/costguard/whitelist"
	cfg.SecretsManager = config.SecretsManagerConfig{URL: "http://sm.invalid", Username: "u", Password: "p"}

	ts := httptest.NewServer(vaultFakeWithEntry("vol-9", whitelist.TypeVolume))
	t.Cleanup(ts.Close)

	oldStore := newSMStore
	t.Cleanup(func() { newSMStore = oldStore })
	newSMStore = func(baseURL, username, password, path string, logger *slog.Logger) (*whitelist.Store, error) {
		// A real Store bound to the Vault fake; Load returns one entry.
		return whitelist.NewStore(ts.URL, username, password, path, logger)
	}

	wl, err := loadWhitelist(context.Background(), cfg, testLogger(t))
	if err != nil {
		t.Fatalf("loadWhitelist: %v", err)
	}
	if !wl.VolumeProtected("vol-9") {
		t.Error("volume from the shared whitelist is not protected")
	}
}

func TestLoadWhitelistStoreErrorIsFatal(t *testing.T) {
	cfg := testConfig(t)
	cfg.WhitelistSecretPath = "secret/costguard/whitelist"
	cfg.SecretsManager = config.SecretsManagerConfig{URL: "http://sm.invalid", Username: "u", Password: "p"}

	oldStore := newSMStore
	t.Cleanup(func() { newSMStore = oldStore })
	newSMStore = func(baseURL, username, password, path string, logger *slog.Logger) (*whitelist.Store, error) {
		return nil, errors.New("boom")
	}

	if _, err := loadWhitelist(context.Background(), cfg, testLogger(t)); err == nil {
		t.Fatal("loadWhitelist should fail when the store connection fails")
	}
}

func TestLoadWhitelistPathWithoutSMURLFails(t *testing.T) {
	cfg := testConfig(t)
	cfg.WhitelistSecretPath = "secret/costguard/whitelist"
	if _, err := loadWhitelist(context.Background(), cfg, testLogger(t)); err == nil {
		t.Fatal("loadWhitelist should fail when the SM URL is missing")
	}
}

func TestBuildNotifierAllModes(t *testing.T) {
	logger := testLogger(t)
	base := testConfig(t)
	base.CallbackURL = "http://cb.invalid"
	base.CallbackSecret = "s3cr3t"

	for mode, wantErr := range map[string]bool{
		config.OutputGoogleChat: false,
		config.OutputSlack:      false,
		config.OutputTeams:      false,
		"carrier-pigeon":        true,
	} {
		cfg := base
		cfg.Output = mode
		n, err := buildNotifier(cfg, logger)
		if wantErr {
			if err == nil {
				t.Errorf("output %q: expected an error, got notifier %v", mode, n)
			}
			continue
		}
		if err != nil || n == nil {
			t.Errorf("output %q: got notifier=%v err=%v, want a notifier", mode, n, err)
		}
	}
}

func TestNewLoggerLevels(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"":      slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"nope":  slog.LevelInfo,
	}
	for in, want := range cases {
		l := newLogger(in)
		if !l.Enabled(context.Background(), want) {
			t.Errorf("newLogger(%q) should enable level %v", in, want)
		}
	}
}

func TestRunDeleteConfirmationFailureIsFatal(t *testing.T) {
	rm := newFakeRM()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	iaas := newFakeIaaS()
	iaas.ips["p1/eu01"] = []stackit.PublicIP{
		{ID: "ip-1", Address: "203.0.113.10", ProjectID: "p1", Region: "eu01",
			Labels: map[string]string{stackit.MarkLabelKey: expiredMark()}},
	}
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 500)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL

	code := runDelete(context.Background(), cfg, testLogger(t))
	if code != ExitFatal {
		t.Fatalf("runDelete exit code = %d, want %d when the confirmation cannot be delivered", code, ExitFatal)
	}
}

func TestRunScanWithScannerErrorsStillExitsOK(t *testing.T) {
	rm := newFakeRM()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	iaas := newFakeIaaS()
	iaas.errListIPs["p1/eu01"] = errors.New("forbidden")
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 204)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL

	code := runScan(context.Background(), cfg, testLogger(t))
	if code != ExitOK {
		t.Fatalf("runScan exit code = %d, want %d: non-fatal scanner errors must not abort the run", code, ExitOK)
	}
}

func TestRunCallbackServerStartFailureExitsFatal(t *testing.T) {
	cfg := testConfig(t)
	// An out-of-range port makes ListenAndServe fail deterministically.
	cfg.CallbackPort = 99999
	cfg.CallbackURL = "http://callback.invalid"
	cfg.CallbackSecret = "s3cr3t"
	cfg.WhitelistSecretPath = "secret/costguard/whitelist"

	rm, iaas := newFakeRM(), newFakeIaaS()
	installFakes(t, rm, iaas)

	vault := httptest.NewServer(newVaultFake())
	t.Cleanup(vault.Close)
	cfg.SecretsManager = config.SecretsManagerConfig{URL: vault.URL, Username: "u", Password: "p"}

	code := runCallback(context.Background(), cfg, testLogger(t))
	if code != ExitFatal {
		t.Fatalf("runCallback exit code = %d, want %d when the listener cannot start", code, ExitFatal)
	}
}

func TestRunMalformedConfigFileExitsFatal(t *testing.T) {
	setValidEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("scope: [unclosed"), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	code := Run(context.Background(), SubcommandScan, path, "error")
	if code != ExitFatal {
		t.Fatalf("Run exit code = %d, want %d for a malformed config file", code, ExitFatal)
	}
}

// ---- helpers -----------------------------------------------------------

func reportForChart() *report.Report {
	return &report.Report{
		DailyCosts: []report.DailyCost{
			{Date: "2026-09-01", CostEUR: 12.5},
			{Date: "2026-09-02", CostEUR: 14.1},
			{Date: "2026-09-03", CostEUR: 13.3},
		},
	}
}

// vaultFakeWithEntry serves login plus a KV v2 GET that returns the
// whitelist object containing one entry.
func vaultFakeWithEntry(id string, entryType whitelist.ResourceType) http.Handler {
	entries := map[string]whitelist.Entry{id: {Type: entryType}}
	raw, _ := json.Marshal(entries)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/user/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"client_token":"root","lease_duration":3600,"policies":["root"]}}`))
	})
	mux.HandleFunc("/v1/secret/data/costguard/whitelist", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			payload := map[string]any{
				"data": map[string]any{
					"data":     map[string]any{whitelist.WhitelistDataKey: string(raw)},
					"metadata": map[string]any{"version": 1},
				},
			}
			b, _ := json.Marshal(payload)
			_, _ = w.Write(b)
			return
		}
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	return mux
}
