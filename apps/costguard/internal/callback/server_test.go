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

package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	oapierror "github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

const (
	tSecret  = "callback-secret"
	tProject = "55555555-5555-5555-5555-555555555555"
	tFolder  = "33333333-3333-3333-3333-333333333333"
	tVolume  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	tIP      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tRegion  = "eu01"
)

var tNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// ---- fakes ----

type fakeRM struct {
	mu           sync.Mutex
	projects     map[string]error
	folders      map[string]error
	getProjectFn func(id string) error
}

func newFakeRM() *fakeRM {
	return &fakeRM{projects: map[string]error{}, folders: map[string]error{}}
}

func (f *fakeRM) ListProjects(context.Context, string) ([]stackit.Project, error) {
	return nil, nil
}
func (f *fakeRM) ListFolders(context.Context, string) ([]stackit.Folder, error) {
	return nil, nil
}
func (f *fakeRM) GetProject(_ context.Context, id string) (*stackit.Project, error) {
	if f.projects[id] != nil {
		return nil, f.projects[id]
	}
	return &stackit.Project{ID: id, Name: "p"}, nil
}
func (f *fakeRM) GetFolder(_ context.Context, id string) (*stackit.Folder, error) {
	if f.folders[id] != nil {
		return nil, f.folders[id]
	}
	return &stackit.Folder{ID: id, Name: "f"}, nil
}

type fakeIaaS struct {
	mu      sync.Mutex
	volumes map[string]error
	ips     map[string]error
}

func newFakeIaaS() *fakeIaaS {
	return &fakeIaaS{volumes: map[string]error{}, ips: map[string]error{}}
}

func (f *fakeIaaS) ListNetworkAreas(context.Context, string) ([]stackit.NetworkArea, error) {
	return nil, nil
}
func (f *fakeIaaS) GetNetworkArea(context.Context, string, string) (*stackit.NetworkArea, error) {
	return nil, nil
}
func (f *fakeIaaS) ListPublicIPs(context.Context, string, string) ([]stackit.PublicIP, error) {
	return nil, nil
}
func (f *fakeIaaS) GetPublicIP(_ context.Context, _, _, id string) (*stackit.PublicIP, error) {
	if f.ips[id] != nil {
		return nil, f.ips[id]
	}
	return &stackit.PublicIP{ID: id, Address: "1.2.3.4"}, nil
}
func (f *fakeIaaS) SetPublicIPMark(context.Context, string, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ClearPublicIPMark(context.Context, string, string, string) error {
	return nil
}
func (f *fakeIaaS) DeletePublicIP(context.Context, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ListVolumes(context.Context, string, string) ([]stackit.Volume, error) {
	return nil, nil
}
func (f *fakeIaaS) GetVolume(_ context.Context, _, _, id string) (*stackit.Volume, error) {
	if f.volumes[id] != nil {
		return nil, f.volumes[id]
	}
	return &stackit.Volume{ID: id, Name: "v"}, nil
}
func (f *fakeIaaS) SetVolumeMark(context.Context, string, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ClearVolumeMark(context.Context, string, string, string) error {
	return nil
}
func (f *fakeIaaS) DeleteVolume(context.Context, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ListSnapshots(context.Context, string, string) ([]stackit.Snapshot, error) {
	return nil, nil
}
func (f *fakeIaaS) DeleteSnapshot(context.Context, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ListServers(context.Context, string, string) ([]stackit.Server, error) {
	return nil, nil
}

type fakeStore struct {
	mu      sync.Mutex
	entries map[string]whitelist.Entry
	err     error
}

func newFakeStore() *fakeStore {
	return &fakeStore{entries: map[string]whitelist.Entry{}}
}

func (f *fakeStore) Append(_ context.Context, id string, e whitelist.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.entries[id]; ok {
		// Idempotency: keep the original entry.
		return nil
	}
	f.entries[id] = e
	return nil
}

// ---- test helpers ----

type logSink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *logSink) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}
func (l *logSink) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func newTestServer(t *testing.T, rm *fakeRM, iaas *fakeIaaS, store *fakeStore) (*Server, *logSink) {
	t.Helper()
	sink := &logSink{}
	logger := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := New(config.Config{}, rm, iaas, store, tSecret, logger)
	srv.Now = func() time.Time { return tNow }
	return srv, sink
}

// signedQuery builds a valid signed query string for a protect GET.
func signedQuery(req notifier.ProtectRequest) string {
	exp := req.Deadline.Unix()
	q := url.Values{}
	q.Set(notifier.ParamID, req.ID)
	q.Set(notifier.ParamType, string(req.Type))
	q.Set(notifier.ParamProject, req.Project)
	q.Set(notifier.ParamRegion, req.Region)
	q.Set(notifier.ParamExp, fmt.Sprintf("%d", exp))
	q.Set(notifier.ParamSig, notifier.Sign([]byte(tSecret), req, exp))
	return q.Encode()
}

func doGet(t *testing.T, srv *Server, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(body)
}

func doPost(t *testing.T, srv *Server, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(b)
}

func signedBody(req notifier.ProtectRequest) string {
	exp := req.Deadline.Unix()
	b, _ := json.Marshal(map[string]any{
		notifier.ParamID:      req.ID,
		notifier.ParamType:    string(req.Type),
		notifier.ParamProject: req.Project,
		notifier.ParamRegion:  req.Region,
		notifier.ParamExp:     exp,
		notifier.ParamSig:     notifier.Sign([]byte(tSecret), req, exp),
	})
	return string(b)
}

// ---- tests ----

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t, newFakeRM(), newFakeIaaS(), newFakeStore())
	code, body := doGet(t, srv, "/healthz")
	if code != 200 || body != "ok" {
		t.Errorf("healthz = %d %q, want 200 ok", code, body)
	}
}

func TestProtectGETProjectSuccess(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}

	code, body := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != 200 {
		t.Fatalf("code = %d body=%q, want 200", code, body)
	}
	if !strings.Contains(body, "protected") {
		t.Errorf("body = %q", body)
	}
	e, ok := store.entries[tProject]
	if !ok {
		t.Fatal("no whitelist entry stored")
	}
	if e.Type != whitelist.TypeProject || e.SavedBy != "protect-button" {
		t.Errorf("entry = %+v", e)
	}
	if !e.SavedAt.Equal(tNow) {
		t.Errorf("savedAt = %v, want the server clock", e.SavedAt)
	}
}

func TestProtectPOSTVolumeSuccess(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tVolume, Type: whitelist.TypeVolume, Project: tProject, Region: tRegion, Deadline: tNow.Add(time.Hour)}

	code, body := doPost(t, srv, signedBody(req))
	if code != 200 {
		t.Fatalf("code = %d body=%q, want 200", code, body)
	}
	if _, ok := store.entries[tVolume]; !ok {
		t.Error("volume not stored in whitelist")
	}
}

func TestProtectIPAndFolderTypes(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)

	ipReq := notifier.ProtectRequest{ID: tIP, Type: whitelist.TypePublicIP, Project: tProject, Region: tRegion, Deadline: tNow.Add(time.Hour)}
	if code, body := doGet(t, srv, "/protect?"+signedQuery(ipReq)); code != 200 {
		t.Errorf("IP protect = %d %q", code, body)
	}
	folderReq := notifier.ProtectRequest{ID: tFolder, Type: whitelist.TypeFolder, Deadline: tNow.Add(time.Hour)}
	if code, body := doGet(t, srv, "/protect?"+signedQuery(folderReq)); code != 200 {
		t.Errorf("folder protect = %d %q", code, body)
	}
	if len(store.entries) != 2 {
		t.Errorf("entries = %v, want 2", store.entries)
	}
}

func TestProtectInvalidSignature(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}
	q := url.Values{}
	q.Set(notifier.ParamID, req.ID)
	q.Set(notifier.ParamType, string(req.Type))
	q.Set(notifier.ParamExp, fmt.Sprintf("%d", req.Deadline.Unix()))
	q.Set(notifier.ParamSig, "forged-signature")

	code, _ := doGet(t, srv, "/protect?"+q.Encode())
	if code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", code)
	}
	if len(store.entries) != 0 {
		t.Error("forged request must not write to the whitelist")
	}
}

func TestProtectTamperedID(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)
	// Sign for the project, then tamper the id to another resource: the
	// signature must not verify (the id is inside the HMAC).
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}
	q := url.Values{}
	q.Set(notifier.ParamID, tFolder)
	q.Set(notifier.ParamType, string(req.Type))
	q.Set(notifier.ParamExp, fmt.Sprintf("%d", req.Deadline.Unix()))
	q.Set(notifier.ParamSig, notifier.Sign([]byte(tSecret), req, req.Deadline.Unix()))

	code, _ := doGet(t, srv, "/protect?"+q.Encode())
	if code != http.StatusForbidden {
		t.Errorf("code = %d, want 403 for tampered id", code)
	}
}

func TestProtectExpired(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(-time.Hour)}

	code, body := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != http.StatusGone {
		t.Fatalf("code = %d, want 410", code)
	}
	if !strings.Contains(body, "expired") {
		t.Errorf("body = %q, want expiry explanation", body)
	}
	if len(store.entries) != 0 {
		t.Error("expired button must not write to the whitelist")
	}
}

func TestProtectMissingParams(t *testing.T) {
	srv, _ := newTestServer(t, newFakeRM(), newFakeIaaS(), newFakeStore())
	cases := []struct {
		name  string
		query string
	}{
		{"no params", ""},
		{"missing id", "type=project&exp=123&sig=x"},
		{"missing type", "id=p1&exp=123&sig=x"},
		{"bad type", "id=p1&type=widget&exp=123&sig=x"},
		{"missing exp", "id=p1&type=project&sig=x"},
		{"bad exp", "id=p1&type=project&exp=abc&sig=x"},
		{"missing sig", "id=p1&type=project&exp=123"},
		{"volume without project", "id=v1&type=volume&region=eu01&exp=123&sig=x"},
		{"volume without region", "id=v1&type=volume&project=p1&exp=123&sig=x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _ := doGet(t, srv, "/protect?"+tc.query)
			if code != http.StatusBadRequest {
				t.Errorf("code = %d, want 400", code)
			}
		})
	}
}

func TestProtectPOSTInvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t, newFakeRM(), newFakeIaaS(), newFakeStore())
	code, body := doPost(t, srv, "{not json")
	if code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 (%q)", code, body)
	}
}

func TestProtectPOSTOversizedBody(t *testing.T) {
	srv, _ := newTestServer(t, newFakeRM(), newFakeIaaS(), newFakeStore())
	big := bytes.Repeat([]byte("x"), maxBodyBytes+1)
	code, _ := doPost(t, srv, string(big))
	if code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 for oversized body", code)
	}
}

func TestProtectResourceGone(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	iaas.volumes[tVolume] = &oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "not found"}
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tVolume, Type: whitelist.TypeVolume, Project: tProject, Region: tRegion, Deadline: tNow.Add(time.Hour)}

	code, body := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != http.StatusGone {
		t.Fatalf("code = %d, want 410", code)
	}
	if !strings.Contains(body, "no longer exists") {
		t.Errorf("body = %q", body)
	}
	if len(store.entries) != 0 {
		t.Error("gone resource must not be whitelisted")
	}
}

func TestProtectProjectGone(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	rm.projects[tProject] = &oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "not found"}
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}

	code, _ := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != http.StatusGone {
		t.Errorf("code = %d, want 410", code)
	}
}

func TestProtectStoreFailure(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	store.err = fmt.Errorf("vault down")
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}

	code, _ := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", code)
	}
}

func TestProtectExistenceCheckUnavailable(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	rm.projects[tProject] = &oapierror.GenericOpenAPIError{StatusCode: 500, ErrorMessage: "rm down"}
	srv, _ := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}

	code, _ := doGet(t, srv, "/protect?"+signedQuery(req))
	if code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", code)
	}
	if len(store.entries) != 0 {
		t.Error("failed existence check must not whitelist")
	}
}

func TestProtectQueryNeverLogged(t *testing.T) {
	rm, iaas, store := newFakeRM(), newFakeIaaS(), newFakeStore()
	srv, sink := newTestServer(t, rm, iaas, store)
	req := notifier.ProtectRequest{ID: tProject, Type: whitelist.TypeProject, Deadline: tNow.Add(time.Hour)}
	qs := signedQuery(req)

	doGet(t, srv, "/protect?"+qs)
	logged := sink.String()
	if strings.Contains(logged, "sig=") || strings.Contains(logged, "sig%3D") {
		t.Errorf("signature leaked into logs: %s", logged)
	}
	if !strings.Contains(logged, tProject) {
		t.Errorf("resource id should still be logged for auditing: %s", logged)
	}
}

func TestProtectUnknownRoute(t *testing.T) {
	srv, _ := newTestServer(t, newFakeRM(), newFakeIaaS(), newFakeStore())
	code, _ := doGet(t, srv, "/nope")
	if code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", code)
	}
}
