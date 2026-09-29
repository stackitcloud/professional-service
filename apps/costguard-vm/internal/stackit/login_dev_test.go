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

//go:build dev

package stackit

import (
	"context"
	"path/filepath"
	"testing"
)

func clearLogins(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("STACKIT_CREDENTIALS_PATH", filepath.Join(dir, "none.json"))
	for _, v := range []string{"STACKIT_SERVICE_ACCOUNT_KEY_PATH", "STACKIT_SERVICE_ACCOUNT_KEY", "STACKIT_PRIVATE_KEY_PATH",
		"STACKIT_PRIVATE_KEY", "STACKIT_SERVICE_ACCOUNT_TOKEN", "STACKIT_FEDERATED_TOKEN_FILE", EnvServiceAccountEmail} {
		t.Setenv(v, "")
	}
}

// A dev build takes the SDK's own logins and needs no attached service
// account.
func TestDevBuildUsesTheSDKLogin(t *testing.T) {
	clearLogins(t)
	t.Setenv("STACKIT_SERVICE_ACCOUNT_TOKEN", "dev-token")
	set, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Login.Ready(context.Background(), 0); err != nil {
		t.Errorf("Ready: %v", err)
	}
}

func TestDevBuildWithoutCredentials(t *testing.T) {
	clearLogins(t)
	if _, err := New(); err == nil {
		t.Fatal("want the SDK's error without any credential")
	}
}
