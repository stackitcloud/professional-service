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

//go:build !dev

package stackit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// offerOtherLogins puts every credential the SDK would pick up by itself
// into the environment. A release build must ignore all of them.
func offerOtherLogins(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	key := filepath.Join(dir, "key.json")
	if err := os.WriteFile(key, []byte(`{"id": "k", "credentials": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	federated := filepath.Join(dir, "federated-token")
	if err := os.WriteFile(federated, []byte("federated-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("STACKIT_SERVICE_ACCOUNT_KEY_PATH", key)
	t.Setenv("STACKIT_SERVICE_ACCOUNT_TOKEN", "env-token")
	t.Setenv("STACKIT_FEDERATED_TOKEN_FILE", federated)
	t.Setenv("STACKIT_SERVICE_ACCOUNT_EMAIL", "env@sa.stackit.cloud")
}

func TestReleaseBuildNeedsTheServiceAccountEmail(t *testing.T) {
	offerOtherLogins(t)
	t.Setenv(EnvServiceAccountEmail, "")
	_, err := New()
	if err == nil {
		t.Fatal("want an error without the service account email")
	}
	if msg := err.Error(); !strings.Contains(msg, EnvServiceAccountEmail) || strings.Contains(msg, "key") {
		t.Errorf("error = %q, want only the missing email", msg)
	}
}

// A release build logs in with the metadata token and nothing else, even
// when the environment offers a key, a token and workload identity.
func TestReleaseBuildLogsInWithTheMetadataTokenOnly(t *testing.T) {
	offerOtherLogins(t)
	t.Setenv(EnvServiceAccountEmail, saEmail)
	c := &clock{t: time.Now()}
	meta := newFakeMetadata(c.now)
	ts := httptest.NewServer(meta)
	t.Cleanup(ts.Close)
	api := &apiRecorder{answers: func(req *http.Request) (int, string) {
		if req.URL.Host != "resource-manager.api.stackit.cloud" {
			return http.StatusNotFound, `{}`
		}
		body, _ := json.Marshal(project("p1", "shop", "f1", nil))
		return http.StatusOK, string(body)
	}}
	oldURL, oldTransport := metadataURL, apiTransport
	metadataURL, apiTransport = ts.URL, api
	t.Cleanup(func() { metadataURL, apiTransport = oldURL, oldTransport })

	set, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Login.Ready(context.Background(), 0); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	p, err := set.ResourceManager.GetProject(context.Background(), "p1")
	if err != nil || p.Name != "shop" {
		t.Fatalf("GetProject = %v, %v", p, err)
	}
	if got := api.seen(); len(got) != 1 || got[0] != "Bearer metadata-token-1" {
		t.Errorf("Authorization = %v, want the metadata token only", got)
	}
	if meta.mintedTokens() != 1 {
		t.Errorf("minted = %d, want one token shared by all clients", meta.mintedTokens())
	}
}
