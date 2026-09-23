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

package whitelist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeVault is a minimal KV v2 server: login + one secret at
// secret/data/test/wl with a version counter and CAS enforcement.
type fakeVault struct {
	mu       sync.Mutex
	version  int
	data     string // JSON string under WhitelistDataKey
	deleted  bool
	conflict int // force this many CAS conflicts before succeeding
}

func (f *fakeVault) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/user/login", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		_ = json.Unmarshal(body, &req)
		if req["username"] != "user" || req["password"] != "pass" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": "test-token"}})
	})
	read := func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.deleted {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":["secret not found"]}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data":     map[string]any{WhitelistDataKey: f.data},
				"metadata": map[string]any{"version": f.version},
			},
		})
	}
	mux.HandleFunc("/v1/secret/data/test/wl", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			read(w, r)
		case http.MethodPost, http.MethodPut:
			f.doPut(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/secret/metadata/test/wl", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == http.MethodDelete {
			f.deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"version": f.version})
	})
	return mux
}

func (f *fakeVault) doPut(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conflict > 0 {
		// Simulate another writer winning the CAS race.
		f.conflict--
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":["invalid index"]}`))
		return
	}
	cas, _ := strconv.Atoi(r.URL.Query().Get("cas"))
	if !f.deleted && f.version != cas {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":["invalid index"]}`))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	data, _ := payload["data"].(map[string]any)
	raw, _ := data[WhitelistDataKey].(string)
	f.data = raw
	f.version++
	f.deleted = false
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"data":     map[string]any{WhitelistDataKey: raw},
			"metadata": map[string]any{"version": f.version},
		},
	})
}

func newTestStore(t *testing.T, f *fakeVault) *Store {
	t.Helper()
	ts := httptest.NewServer(f.handler())
	t.Cleanup(ts.Close)
	s, err := NewStore(ts.URL, "user", "pass", "secret/test/wl", nil)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestSplitPath(t *testing.T) {
	cases := []struct {
		path      string
		mount     string
		secret    string
		wantError bool
	}{
		{"secret/a/b", "secret", "a/b", false},
		{"secret", "", "", true},
		{"/secret/", "", "", true},
		{"secret/ ", "", "", true},
	}
	for _, tc := range cases {
		mount, secret, err := splitPath(tc.path)
		if tc.wantError {
			if err == nil {
				t.Errorf("splitPath(%q): expected error", tc.path)
			}
			continue
		}
		if err != nil || mount != tc.mount || secret != tc.secret {
			t.Errorf("splitPath(%q) = (%q, %q, %v)", tc.path, mount, secret, err)
		}
	}
}

func TestLoadMissingSecretIsEmpty(t *testing.T) {
	f := &fakeVault{}
	s := newTestStore(t, f)
	entries, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %v, want empty", entries)
	}
}

func TestLoadExistingSecret(t *testing.T) {
	f := &fakeVault{version: 3, data: `{"vol-1":{"type":"volume","savedBy":"manual"}}`}
	s := newTestStore(t, f)
	entries, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	e, ok := entries["vol-1"]
	if !ok || e.Type != TypeVolume || e.SavedBy != "manual" {
		t.Errorf("entries = %v", entries)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	f := &fakeVault{version: 1, data: `{not json`}
	s := newTestStore(t, f)
	if _, err := s.Load(context.Background()); err == nil {
		t.Error("expected parse error")
	}
}

func TestLoadWrongDataType(t *testing.T) {
	// The data key must hold a JSON string; a raw object is an error.
	f := &fakeVault{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": "t"}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"data": map[string]any{WhitelistDataKey: map[string]any{"a": 1}}},
			})
		}
	}))
	t.Cleanup(ts.Close)
	s, err := NewStore(ts.URL, "user", "pass", "secret/test/wl", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(context.Background()); err == nil {
		t.Error("expected type error for non-string data key")
	}
	_ = f
}

func TestAppendCreatesEntry(t *testing.T) {
	f := &fakeVault{}
	s := newTestStore(t, f)
	entry := Entry{Type: TypeVolume, SavedBy: "chat:tester", SavedAt: time.Now().UTC().Truncate(time.Second)}
	if err := s.Append(context.Background(), "vol-1", entry); err != nil {
		t.Fatalf("Append: %v", err)
	}
	entries, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := entries["vol-1"]; !ok || got.SavedBy != "chat:tester" || got.Type != TypeVolume {
		t.Errorf("entries = %v", entries)
	}
}

func TestAppendIdempotentKeepsOriginalMetadata(t *testing.T) {
	f := &fakeVault{version: 1, data: `{"vol-1":{"type":"volume","savedBy":"original","savedAt":"2026-01-01T00:00:00Z"}}`}
	s := newTestStore(t, f)
	before, _ := s.Load(context.Background())
	entry := Entry{Type: TypeVolume, SavedBy: "someone-else", SavedAt: time.Now().UTC()}
	if err := s.Append(context.Background(), "vol-1", entry); err != nil {
		t.Fatalf("Append: %v", err)
	}
	after, _ := s.Load(context.Background())
	if after["vol-1"].SavedBy != before["vol-1"].SavedBy {
		t.Errorf("original metadata must be preserved, got %v", after["vol-1"])
	}
	if after["vol-1"].SavedAt != before["vol-1"].SavedAt {
		t.Errorf("original savedAt must be preserved, got %v", after["vol-1"].SavedAt)
	}
}

func TestAppendRetriesOnCASConflict(t *testing.T) {
	// Two conflicts, then the write lands — still one entry.
	f := &fakeVault{conflict: 2}
	s := newTestStore(t, f)
	entry := Entry{Type: TypePublicIP, SavedBy: "chat:a", SavedAt: time.Now().UTC().Truncate(time.Second)}
	if err := s.Append(context.Background(), "ip-1", entry); err != nil {
		t.Fatalf("Append: %v", err)
	}
	entries, _ := s.Load(context.Background())
	if _, ok := entries["ip-1"]; !ok {
		t.Errorf("entries = %v", entries)
	}
}

func TestAppendGivesUpAfterMaxAttempts(t *testing.T) {
	f := &fakeVault{conflict: 99}
	s := newTestStore(t, f)
	entry := Entry{Type: TypeVolume, SavedBy: "x"}
	err := s.Append(context.Background(), "vol-1", entry)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d CAS attempts", maxCASAttempts)) {
		t.Fatalf("expected CAS exhaustion error, got %v", err)
	}
}

func TestAppendNonConflictError(t *testing.T) {
	// A hard SM error (500) must surface immediately, not be retried.
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/auth/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": "t"}})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data":     map[string]any{WhitelistDataKey: "{}"},
				"metadata": map[string]any{"version": 1},
			}})
		default:
			calls++
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(ts.Close)
	s, err := NewStore(ts.URL, "user", "pass", "secret/test/wl", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append(context.Background(), "vol-1", Entry{Type: TypeVolume}); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("hard error must not be retried, got %d writes", calls)
	}
}

func TestNewStoreBadPath(t *testing.T) {
	if _, err := NewStore("http://unused.invalid", "u", "p", "noseparator", nil); err == nil {
		t.Error("expected path error")
	}
}

func TestNewStoreLoginFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(ts.Close)
	if _, err := NewStore(ts.URL, "wrong", "wrong", "secret/test/wl", nil); err == nil {
		t.Error("expected login error")
	}
}
