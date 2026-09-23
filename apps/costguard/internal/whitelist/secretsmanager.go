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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

// WhitelistDataKey is the single KV v2 data key under which the whitelist
// object (JSON, keyed by resource ID) is stored — one KV v2 secret, one
// object.
const WhitelistDataKey = "costguard-whitelist"

// maxCASAttempts bounds the compare-and-swap conflict retry loop in
// Append: two people clicking "Do not delete" at the same instant is the
// expected contention, anything beyond three retries is a real problem.
const maxCASAttempts = 3

// Store is a Vault KV v2 client bound to the whitelist path on the
// dedicated Secrets Manager instance. The instance-level IAM scope
// is the permission boundary: the callback's credential has no reach
// beyond this instance.
type Store struct {
	kv     *vaultapi.KVv2
	path   string
	logger *slog.Logger
}

// NewStore connects to the dedicated Secrets Manager instance with the
// dedicated SM user (userpass baseline) and binds to the whitelist
// path. path is the full KV v2 path including the mount, e.g.
// "secret/costguard/whitelist" (mount "secret", secret path
// "costguard/whitelist").
func NewStore(baseURL, username, password, path string, logger *slog.Logger) (*Store, error) {
	client, err := vaultapi.NewClient(&vaultapi.Config{
		Address: baseURL,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Secrets Manager client: %w", err)
	}
	if err := login(client, username, password); err != nil {
		return nil, err
	}
	mount, secretPath, err := splitPath(path)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Debug("whitelist store bound", "mount", mount, "path", secretPath)
	}
	return &Store{kv: client.KVv2(mount), path: secretPath, logger: logger}, nil
}

func login(client *vaultapi.Client, username, password string) error {
	secret, err := client.Logical().Write("auth/user/login", map[string]interface{}{
		"username": username,
		"password": password,
	})
	if err != nil {
		return fmt.Errorf("Secrets Manager userpass login: %w", err)
	}
	if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
		return fmt.Errorf("Secrets Manager userpass login: no client token in response")
	}
	client.SetToken(secret.Auth.ClientToken)
	return nil
}

// splitPath splits a KV v2 path into its mount and secret path parts.
func splitPath(path string) (mount, secretPath string, err error) {
	parts := strings.Split(strings.TrimSpace(strings.Trim(path, "/")), "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("whitelist secret path %q must be of the form <mount>/<path>", path)
	}
	return parts[0], strings.Join(parts[1:], "/"), nil
}

// read fetches the whitelist object and its KV v2 version for CAS. A
// missing secret yields an empty map and version 0 (create-if-absent).
func (s *Store) read(ctx context.Context) (map[string]Entry, int, error) {
	sec, err := s.kv.Get(ctx, s.path)
	if err != nil {
		if errors.Is(err, vaultapi.ErrSecretNotFound) {
			return map[string]Entry{}, 0, nil
		}
		return nil, 0, fmt.Errorf("reading whitelist secret: %w", err)
	}
	entries := map[string]Entry{}
	if sec.Data != nil {
		raw, ok := sec.Data[WhitelistDataKey]
		if !ok {
			if s.logger != nil {
				s.logger.Warn("whitelist secret has no data key, treating as empty", "key", WhitelistDataKey)
			}
			return entries, secVersion(sec), nil
		}
		str, ok := raw.(string)
		if !ok {
			return nil, 0, fmt.Errorf("whitelist data key %q must hold a JSON string", WhitelistDataKey)
		}
		if str != "" {
			if err := json.Unmarshal([]byte(str), &entries); err != nil {
				return nil, 0, fmt.Errorf("parsing whitelist JSON: %w", err)
			}
		}
	}
	return entries, secVersion(sec), nil
}

func secVersion(sec *vaultapi.KVSecret) int {
	if sec == nil || sec.VersionMetadata == nil {
		return 0
	}
	return sec.VersionMetadata.Version
}

// Load reads the current whitelist; a missing secret is an empty map, not
// an error.
func (s *Store) Load(ctx context.Context) (map[string]Entry, error) {
	entries, _, err := s.read(ctx)
	return entries, err
}

// Append adds {id: entry} using a read-modify-write with KV v2
// compare-and-swap: concurrent clicks cannot lose entries.
// It is idempotent — an existing entry is preserved with its original
// savedBy/savedAt. On a CAS conflict (another write landed first)
// the operation re-reads and retries, up to maxCASAttempts.
func (s *Store) Append(ctx context.Context, id string, e Entry) error {
	var lastErr error
	for attempt := 1; attempt <= maxCASAttempts; attempt++ {
		entries, version, err := s.read(ctx)
		if err != nil {
			return err
		}
		if _, exists := entries[id]; exists {
			return nil
		}
		entries[id] = e
		raw, err := json.Marshal(entries)
		if err != nil {
			return fmt.Errorf("encoding whitelist JSON: %w", err)
		}
		_, lastErr = s.kv.Put(ctx, s.path, map[string]interface{}{WhitelistDataKey: string(raw)}, vaultapi.WithCheckAndSet(version))
		if lastErr == nil {
			return nil
		}
		if !isConflict(lastErr) {
			return fmt.Errorf("appending to whitelist: %w", lastErr)
		}
		if s.logger != nil {
			s.logger.Debug("whitelist CAS conflict, retrying", "attempt", attempt, "id", id)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 50 * time.Millisecond):
		}
	}
	return fmt.Errorf("appending to whitelist after %d CAS attempts: %w", maxCASAttempts, lastErr)
}

// isConflict reports whether a KV v2 Put failed because another writer
// bumped the version first. Vault signals a lost CAS as an "invalid
// index" error (HTTP 400); some gateways surface it as 409.
func isConflict(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid index") || strings.Contains(msg, "409") || strings.Contains(msg, "conflict")
}
