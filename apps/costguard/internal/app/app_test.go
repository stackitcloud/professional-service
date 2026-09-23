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
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// ---- fakes -------------------------------------------------------------

type fakeRM struct {
	mu       sync.Mutex
	projects map[string][]stackit.Project
	folders  map[string][]stackit.Folder
}

func newFakeRM() *fakeRM {
	return &fakeRM{projects: map[string][]stackit.Project{}, folders: map[string][]stackit.Folder{}}
}

func (f *fakeRM) ListProjects(_ context.Context, containerID string) ([]stackit.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]stackit.Project{}, f.projects[containerID]...), nil
}

func (f *fakeRM) ListFolders(_ context.Context, containerID string) ([]stackit.Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]stackit.Folder{}, f.folders[containerID]...), nil
}

func (f *fakeRM) GetProject(_ context.Context, projectID string) (*stackit.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, list := range f.projects {
		for _, p := range list {
			if p.ID == projectID {
				return &p, nil
			}
		}
	}
	return nil, errors.New("project not found")
}

func (f *fakeRM) GetFolder(_ context.Context, folderID string) (*stackit.Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, list := range f.folders {
		for _, fo := range list {
			if fo.ID == folderID {
				return &fo, nil
			}
		}
	}
	return nil, errors.New("folder not found")
}

type fakeIaaS struct {
	mu             sync.Mutex
	ips            map[string][]stackit.PublicIP
	volumes        map[string][]stackit.Volume
	snapshots      map[string][]stackit.Snapshot
	setMarks       map[string]string
	clearedMarks   map[string]bool
	deletedIPs     []string
	deletedVolumes []string
	deletedSnaps   []string
	errListIPs     map[string]error
}

func newFakeIaaS() *fakeIaaS {
	return &fakeIaaS{
		ips:          map[string][]stackit.PublicIP{},
		volumes:      map[string][]stackit.Volume{},
		snapshots:    map[string][]stackit.Snapshot{},
		setMarks:     map[string]string{},
		clearedMarks: map[string]bool{},
		errListIPs:   map[string]error{},
	}
}

func prk(project, region string) string { return project + "/" + region }

func (f *fakeIaaS) ListNetworkAreas(_ context.Context, _ string) ([]stackit.NetworkArea, error) {
	return nil, nil
}
func (f *fakeIaaS) GetNetworkArea(_ context.Context, _, id string) (*stackit.NetworkArea, error) {
	return nil, errors.New("area not found")
}
func (f *fakeIaaS) ListPublicIPs(_ context.Context, projectID, region string) ([]stackit.PublicIP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListIPs[prk(projectID, region)] != nil {
		return nil, f.errListIPs[prk(projectID, region)]
	}
	return append([]stackit.PublicIP{}, f.ips[prk(projectID, region)]...), nil
}
func (f *fakeIaaS) GetPublicIP(_ context.Context, projectID, region, id string) (*stackit.PublicIP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ip := range f.ips[prk(projectID, region)] {
		if ip.ID == id {
			return &ip, nil
		}
	}
	return nil, errors.New("ip not found")
}
func (f *fakeIaaS) SetPublicIPMark(_ context.Context, projectID, region, id, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setMarks["ip/"+prk(projectID, region)+"/"+id] = value
	return nil
}
func (f *fakeIaaS) ClearPublicIPMark(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearedMarks["ip/"+prk(projectID, region)+"/"+id] = true
	return nil
}
func (f *fakeIaaS) DeletePublicIP(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedIPs = append(f.deletedIPs, "ip/"+prk(projectID, region)+"/"+id)
	return nil
}
func (f *fakeIaaS) ListVolumes(_ context.Context, projectID, region string) ([]stackit.Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]stackit.Volume{}, f.volumes[prk(projectID, region)]...), nil
}
func (f *fakeIaaS) GetVolume(_ context.Context, projectID, region, id string) (*stackit.Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.volumes[prk(projectID, region)] {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, errors.New("volume not found")
}
func (f *fakeIaaS) SetVolumeMark(_ context.Context, projectID, region, id, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setMarks["vol/"+prk(projectID, region)+"/"+id] = value
	return nil
}
func (f *fakeIaaS) ClearVolumeMark(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearedMarks["vol/"+prk(projectID, region)+"/"+id] = true
	return nil
}
func (f *fakeIaaS) DeleteVolume(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedVolumes = append(f.deletedVolumes, "vol/"+prk(projectID, region)+"/"+id)
	return nil
}
func (f *fakeIaaS) ListSnapshots(_ context.Context, projectID, region string) ([]stackit.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]stackit.Snapshot{}, f.snapshots[prk(projectID, region)]...), nil
}
func (f *fakeIaaS) DeleteSnapshot(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedSnaps = append(f.deletedSnaps, id)
	return nil
}
func (f *fakeIaaS) ListServers(_ context.Context, _ string, _ string) ([]stackit.Server, error) {
	return nil, nil
}

type fakeCost struct{}

func (fakeCost) ListCostsForCustomer(_ context.Context, _ string, _, _ time.Time) ([]stackit.CostRecord, error) {
	return nil, nil
}

type fakeSKE struct{}

func (fakeSKE) ListClusterNames(_ context.Context, _ string) ([]string, error) { return nil, nil }

type fakeOS struct{}

func (fakeOS) ListBucketNames(_ context.Context, _ string) ([]string, error) { return nil, nil }

// ---- helpers -----------------------------------------------------------

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		Scope:                   config.ScopeOrganisation,
		OrgID:                   "org-1",
		Regions:                 []string{"eu01"},
		MaxAgeDays:              90,
		SNAMaxAgeDays:           90,
		SafeLabelKey:            "keep",
		SafeLabelValue:          "yes",
		Output:                  config.OutputGoogleChat,
		WebhookURL:              "http://webhook.invalid",
		DryRun:                  false,
		CostAnomalyThresholdPct: 20,
		VolumeCostEurPerGB:      0.0619,
		PublicIPCostEurPerMonth: 4.82,
		GracePeriod:             8 * time.Hour,
	}
}

// webhookServer records every request body and answers with the
// configured status.
type webhookServer struct {
	*httptest.Server
	status int
	mu     sync.Mutex
	bodies []string
}

func newWebhookServer(t *testing.T, status int) *webhookServer {
	t.Helper()
	ws := &webhookServer{status: status}
	ws.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ws.mu.Lock()
		ws.bodies = append(ws.bodies, string(body))
		ws.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(ws.Close)
	return ws
}

func (w *webhookServer) last(t *testing.T) string {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.bodies) == 0 {
		t.Fatal("webhook was never called")
	}
	return w.bodies[len(w.bodies)-1]
}

func (w *webhookServer) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.bodies)
}

func expiredMark() string { return stackit.FormatMark(time.Now().Add(-1 * time.Hour)) }

// testProject returns a recent, active, non-stale project.
func testProject() stackit.Project {
	return stackit.Project{
		ID:             "p1",
		Name:           "Project 1",
		CreationTime:   time.Now().Add(-24 * time.Hour),
		LifecycleState: "ACTIVE",
	}
}

func buildSet(rm *fakeRM, iaas *fakeIaaS) *stackit.Set {
	return &stackit.Set{
		ResourceManager: rm,
		IaaS:            iaas,
		Cost:            fakeCost{},
		SKE:             fakeSKE{},
		ObjectStorage:   fakeOS{},
	}
}

func installFakes(t *testing.T, rm *fakeRM, iaas *fakeIaaS) {
	t.Helper()
	oldClients := newStackitClients
	t.Cleanup(func() { newStackitClients = oldClients })
	newStackitClients = func(...stackit.ConfigurationOption) (*stackit.Set, error) {
		return buildSet(rm, iaas), nil
	}
}

// ---- tests --------------------------------------------------------------

func TestRunScanSendsReportAndMarksCandidates(t *testing.T) {
	rm := newFakeRM()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	iaas := newFakeIaaS()
	iaas.ips["p1/eu01"] = []stackit.PublicIP{
		{ID: "ip-1", Address: "203.0.113.10", ProjectID: "p1", Region: "eu01", AttachedNIC: ""},
	}
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 204)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL

	code := runScan(context.Background(), cfg, testLogger(t))
	if code != ExitOK {
		t.Fatalf("runScan exit code = %d, want %d", code, ExitOK)
	}

	// The idle IP must have been stamped with a mark.
	iaas.mu.Lock()
	mark, marked := iaas.setMarks["ip/p1/eu01/ip-1"]
	iaas.mu.Unlock()
	if !marked || mark == "" {
		t.Fatalf("idle IP was not marked, setMarks=%v", iaas.setMarks)
	}
	deadline, ok := stackit.MarkDeadline(map[string]string{stackit.MarkLabelKey: mark})
	if !ok || time.Until(deadline) < 7*time.Hour {
		t.Fatalf("mark deadline %v should be roughly now + grace period", deadline)
	}

	body := ws.last(t)
	if !strings.Contains(body, "cardsV2") {
		t.Errorf("googlechat payload missing cardsV2: %.200s", body)
	}
	if !strings.Contains(body, "ip-1") {
		t.Errorf("report does not mention the idle IP: %.400s", body)
	}
}

func TestRunScanWebhookFailureIsFatal(t *testing.T) {
	rm := newFakeRM()
	iaas := newFakeIaaS()
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 500)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL

	code := runScan(context.Background(), cfg, testLogger(t))
	if code != ExitFatal {
		t.Fatalf("runScan exit code = %d, want %d on webhook failure", code, ExitFatal)
	}
}

func TestRunDeleteDryRunDeletesNothing(t *testing.T) {
	rm := newFakeRM()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	iaas := newFakeIaaS()
	iaas.ips["p1/eu01"] = []stackit.PublicIP{
		{ID: "ip-1", Address: "203.0.113.10", ProjectID: "p1", Region: "eu01",
			Labels: map[string]string{stackit.MarkLabelKey: expiredMark()}},
	}
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 204)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL
	cfg.DryRun = true

	code := runDelete(context.Background(), cfg, testLogger(t))
	if code != ExitOK {
		t.Fatalf("runDelete exit code = %d, want %d", code, ExitOK)
	}

	iaas.mu.Lock()
	defer iaas.mu.Unlock()
	if len(iaas.deletedIPs) != 0 || len(iaas.deletedVolumes) != 0 {
		t.Fatalf("dry run must not delete anything, deleted IPs=%v volumes=%v", iaas.deletedIPs, iaas.deletedVolumes)
	}
	if ws.count() != 1 {
		t.Fatalf("dry run should send exactly one message (the report), got %d", ws.count())
	}
	if !strings.Contains(ws.last(t), "ip-1") {
		t.Error("dry-run report does not mention the due candidate")
	}
}

func TestRunDeleteDeletesDueCandidatesAndSendsConfirmation(t *testing.T) {
	rm := newFakeRM()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	iaas := newFakeIaaS()
	iaas.ips["p1/eu01"] = []stackit.PublicIP{
		// Due candidate.
		{ID: "ip-1", Address: "203.0.113.10", ProjectID: "p1", Region: "eu01",
			Labels: map[string]string{stackit.MarkLabelKey: expiredMark()}},
		// Still inside the grace period: must survive.
		{ID: "ip-2", Address: "203.0.113.11", ProjectID: "p1", Region: "eu01",
			Labels: map[string]string{stackit.MarkLabelKey: stackit.FormatMark(time.Now().Add(1 * time.Hour))}},
	}
	iaas.volumes["p1/eu01"] = []stackit.Volume{
		// Due candidate with a snapshot that must go first.
		{ID: "vol-1", Name: "data", ProjectID: "p1", Region: "eu01", SizeGB: 100,
			Status: "AVAILABLE", Labels: map[string]string{stackit.MarkLabelKey: expiredMark()},
			CreatedAt: time.Now().Add(-48 * time.Hour)},
		// Attached: never a candidate.
		{ID: "vol-2", Name: "system", ProjectID: "p1", Region: "eu01", SizeGB: 50,
			Status: "IN_USE", ServerID: "srv-1", CreatedAt: time.Now().Add(-48 * time.Hour)},
	}
	iaas.snapshots["p1/eu01"] = []stackit.Snapshot{
		{ID: "snap-1", VolumeID: "vol-1"},
	}
	installFakes(t, rm, iaas)

	ws := newWebhookServer(t, 204)
	cfg := testConfig(t)
	cfg.WebhookURL = ws.URL

	code := runDelete(context.Background(), cfg, testLogger(t))
	if code != ExitOK {
		t.Fatalf("runDelete exit code = %d, want %d", code, ExitOK)
	}

	iaas.mu.Lock()
	defer iaas.mu.Unlock()
	if len(iaas.deletedIPs) != 1 || iaas.deletedIPs[0] != "ip/p1/eu01/ip-1" {
		t.Errorf("deleted IPs = %v, want only the due one", iaas.deletedIPs)
	}
	if len(iaas.deletedVolumes) != 1 || iaas.deletedVolumes[0] != "vol/p1/eu01/vol-1" {
		t.Errorf("deleted volumes = %v, want only the due one", iaas.deletedVolumes)
	}
	if len(iaas.deletedSnaps) != 1 || iaas.deletedSnaps[0] != "snap-1" {
		t.Errorf("deleted snapshots = %v, want the volume's snapshot first", iaas.deletedSnaps)
	}

	if ws.count() != 1 {
		t.Fatalf("delete run should send exactly one message (the confirmation), got %d", ws.count())
	}
	body := ws.last(t)
	if !strings.Contains(body, "203.0.113.10") || !strings.Contains(body, "volume data") {
		t.Errorf("confirmation does not mention the deleted resources: %.400s", body)
	}
}

func TestRunInvalidConfigExitsFatal(t *testing.T) {
	t.Setenv(config.EnvScope, "") // missing required field
	setValidEnvExcept(t, config.EnvScope)
	code := Run(context.Background(), SubcommandScan, "", "error")
	if code != ExitFatal {
		t.Fatalf("Run exit code = %d, want %d for invalid config", code, ExitFatal)
	}
}

func TestRunUnknownSubcommandExitsUsage(t *testing.T) {
	setValidEnv(t)
	code := Run(context.Background(), "frobnicate", "", "error")
	if code != ExitUsage {
		t.Fatalf("Run exit code = %d, want %d for unknown subcommand", code, ExportUsageCode())
	}
}

func TestRunCallbackStoreFailureExitsFatal(t *testing.T) {
	setValidEnv(t)
	t.Setenv(config.EnvCallbackURL, "http://callback.invalid")
	t.Setenv(config.EnvCallbackSecret, "s3cr3t")
	t.Setenv(config.EnvCallbackPort, "8443")
	t.Setenv(config.EnvWhitelistSecretPath, "secret/costguard/whitelist")
	t.Setenv(config.EnvSMURL, "http://127.0.0.1:9")
	t.Setenv(config.EnvSMUsername, "u")
	t.Setenv(config.EnvSMPasswrd, "p")

	oldClients, oldStore := newStackitClients, newSMStore
	t.Cleanup(func() { newStackitClients, newSMStore = oldClients, oldStore })
	rm, iaas := newFakeRM(), newFakeIaaS()
	newStackitClients = func(...stackit.ConfigurationOption) (*stackit.Set, error) {
		return buildSet(rm, iaas), nil
	}
	newSMStore = func(baseURL, username, password, path string, logger *slog.Logger) (*whitelist.Store, error) {
		return nil, errors.New("connection refused")
	}

	code := Run(context.Background(), SubcommandCallback, "", "error")
	if code != ExitFatal {
		t.Fatalf("Run exit code = %d, want %d on store failure", code, ExitFatal)
	}
}

func TestRunCallbackServesAndShutsDownGracefully(t *testing.T) {
	setValidEnv(t)
	t.Setenv(config.EnvCallbackURL, "http://callback.invalid")
	t.Setenv(config.EnvCallbackSecret, "s3cr3t")
	t.Setenv(config.EnvWhitelistSecretPath, "secret/costguard/whitelist")

	// Minimal Vault fake: userpass login + empty whitelist secret.
	vault := httptest.NewServer(newVaultFake())
	t.Cleanup(vault.Close)
	t.Setenv(config.EnvSMURL, vault.URL)
	t.Setenv(config.EnvSMUsername, "u")
	t.Setenv(config.EnvSMPasswrd, "p")

	rm, iaas := newFakeRM(), newFakeIaaS()
	rm.projects["org-1"] = []stackit.Project{testProject()}
	installFakes(t, rm, iaas)

	// Port 0 is not usable for the real listener; pick a fixed free one.
	port := freePort(t)
	t.Setenv(config.EnvCallbackPort, itoa(port))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- Run(ctx, SubcommandCallback, "", "error") }()

	// Wait for the server to come up, then hit /healthz.
	healthURL := "http://127.0.0.1:" + itoa(port) + "/healthz"
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			lastErr = nil
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("healthz never came up: %v", lastErr)
	}

	cancel()
	select {
	case code := <-done:
		if code != ExitOK {
			t.Fatalf("Run exit code = %d, want %d after graceful shutdown", code, ExitOK)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("callback did not shut down in time")
	}
}

// newVaultFake serves the two endpoints the whitelist Store uses: the
// userpass login and the KV v2 data read (secret absent → empty map).
func newVaultFake() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/user/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auth":{"client_token":"root","lease_duration":3600,"policies":["root"]}}`))
	})
	mux.HandleFunc("/v1/secret/data/costguard/whitelist", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":["no secret found at path"]}`))
		case http.MethodPut, http.MethodPost:
			_, _ = w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

// ---- config-env helpers --------------------------------------------------

// setValidEnv fills the environment with a minimal valid configuration.
func setValidEnv(t *testing.T) {
	t.Helper()
	setValidEnvExcept(t)
}

func setValidEnvExcept(t *testing.T, skip ...string) {
	t.Helper()
	skipped := map[string]bool{}
	for _, s := range skip {
		skipped[s] = true
	}
	set := func(k, v string) {
		if !skipped[k] {
			t.Setenv(k, v)
		}
	}
	set(config.EnvScope, config.ScopeOrganisation)
	set(config.EnvOrgID, "org-1")
	set(config.EnvRegions, "eu01")
	set(config.EnvMaxAgeDays, "90")
	set(config.EnvSNAMaxAgeDays, "90")
	set(config.EnvSafeLabelKey, "keep")
	set(config.EnvSafeLabelValue, "yes")
	set(config.EnvOutput, config.OutputGoogleChat)
	set(config.EnvWebhookURL, "http://webhook.invalid")
	set(config.EnvDryRun, "false")
}

// freePort asks the kernel for a free TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func itoa(v int) string { return strconv.Itoa(v) }

// ExportUsageCode exposes the usage exit code for cross-package tests.
func ExportUsageCode() int { return ExitUsage }

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
