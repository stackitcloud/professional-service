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

package stackit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const EnvServiceAccountEmail = "COSTGUARD_SERVICE_ACCOUNT_EMAIL"

const (
	metadataTimeout = 40 * time.Second
	refreshMargin   = 5 * time.Minute
	refreshBackoff  = 30 * time.Second
	maxMetadataBody = 64 << 10
	attachPoll      = 5 * time.Second
)

var (
	metadataURL                      = "http://169.254.169.254"
	apiTransport   http.RoundTripper = http.DefaultTransport
	metadataPauses                   = []time.Duration{2 * time.Second, 5 * time.Second}
)

var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+@[A-Za-z0-9.-]+$`)

var (
	errNotAttached = errors.New("no service account is attached to this server")
	errUnreachable = errors.New("the metadata service did not answer")
)

type MetadataLogin struct {
	email  string
	base   string
	client *http.Client
	next   http.RoundTripper
	now    func() time.Time
	pauses []time.Duration
	poll   time.Duration

	mu         sync.Mutex
	token      string
	validUntil time.Time
	retryAfter time.Time
}

func NewMetadataLogin(email string) (*MetadataLogin, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("%s is not set: costguard needs the email of the service account attached to this server", EnvServiceAccountEmail)
	}
	if !emailPattern.MatchString(email) {
		return nil, fmt.Errorf("%s is not a service account email (got %q)", EnvServiceAccountEmail, email)
	}
	return &MetadataLogin{
		email: email,
		base:  metadataURL,
		client: &http.Client{
			Timeout: metadataTimeout,
			Transport: &http.Transport{
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				ResponseHeaderTimeout: metadataTimeout,
				MaxIdleConns:          1,
				IdleConnTimeout:       30 * time.Second,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		next:   apiTransport,
		now:    time.Now,
		pauses: metadataPauses,
		poll:   attachPoll,
	}, nil
}

func (m *MetadataLogin) Email() string {
	return m.email
}

func (m *MetadataLogin) RoundTrip(req *http.Request) (*http.Response, error) {
	if !stackitAPI(req.URL) {
		closeBody(req)
		return nil, fmt.Errorf("costguard sends its STACKIT token only to https://*.stackit.cloud, not to %s://%s", req.URL.Scheme, req.URL.Host)
	}
	token, err := m.Token(req.Context())
	if err != nil {
		closeBody(req)
		return nil, err
	}
	out := req.Clone(req.Context())
	out.Header.Set("Authorization", "Bearer "+token)
	resp, err := m.next.RoundTrip(out)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		m.forget(token)
	}
	return resp, err
}

func (m *MetadataLogin) Ready(ctx context.Context, wait time.Duration) error {
	deadline := m.now().Add(wait)
	for {
		_, err := m.Token(ctx)
		if err == nil {
			return nil
		}
		transient := errors.Is(err, errNotAttached) || errors.Is(err, errUnreachable)
		if !transient || ctx.Err() != nil {
			return err
		}
		if !m.now().Before(deadline) {
			if wait > 0 {
				return fmt.Errorf("%w (still after waiting %s)", err, waited(wait))
			}
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(m.poll):
		}
	}
}

func (m *MetadataLogin) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	stillValid := m.token != "" && now.Before(m.validUntil)
	if stillValid && (now.Before(m.validUntil.Add(-refreshMargin)) || now.Before(m.retryAfter)) {
		return m.token, nil
	}
	token, until, err := m.fetch(ctx)
	if err != nil {
		if stillValid {
			m.retryAfter = now.Add(refreshBackoff)
			return m.token, nil
		}
		return "", err
	}
	m.token, m.validUntil, m.retryAfter = token, until, time.Time{}
	return token, nil
}

func (m *MetadataLogin) forget(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == token {
		m.token, m.validUntil, m.retryAfter = "", time.Time{}, time.Time{}
	}
}

func (m *MetadataLogin) fetch(ctx context.Context) (string, time.Time, error) {
	var answer struct {
		Token      string `json:"token"`
		ValidUntil string `json:"validUntil"`
	}
	status, err := m.get(ctx, "/stackit/v1/service-accounts/"+url.PathEscape(m.email)+"/token", &answer)
	switch {
	case err != nil:
		return "", time.Time{}, err
	case status == http.StatusNotFound:
		return "", time.Time{}, m.explain(ctx)
	case status != http.StatusOK:
		return "", time.Time{}, fmt.Errorf("the metadata service answered the token request with HTTP %d", status)
	case answer.Token == "":
		return "", time.Time{}, errors.New("the metadata service answered the token request without a token")
	}
	until, err := time.Parse(time.RFC3339Nano, answer.ValidUntil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("the metadata service answered the token request with an unreadable validUntil %q", answer.ValidUntil)
	}
	if !until.After(m.now()) {
		return "", time.Time{}, fmt.Errorf("the metadata service returned a token that expired at %s", until.UTC().Format(time.RFC3339))
	}
	return answer.Token, until, nil
}

func (m *MetadataLogin) explain(ctx context.Context) error {
	var list struct {
		ServiceAccountMails []string `json:"serviceAccountMails"`
	}
	status, err := m.get(ctx, "/stackit/v1/service-accounts", &list)
	switch {
	case err != nil:
		return err
	case status != http.StatusOK:
		return fmt.Errorf("the metadata service has no token for %s (HTTP 404) and answered the list of service accounts with HTTP %d", m.email, status)
	case len(list.ServiceAccountMails) == 0:
		return fmt.Errorf("%w: Terraform attaches %s right after creating the server; run terraform apply again", errNotAttached, m.email)
	}
	for _, mail := range list.ServiceAccountMails {
		if strings.EqualFold(mail, m.email) {
			return fmt.Errorf("%s is attached to this server, but the metadata service issued no token for it (HTTP 404)", m.email)
		}
	}
	return fmt.Errorf("this server has %s attached, not costguard's service account %s; run terraform apply to attach it again",
		strings.Join(list.ServiceAccountMails, ", "), m.email)
}

func (m *MetadataLogin) get(ctx context.Context, path string, out any) (int, error) {
	for attempt := 0; ; attempt++ {
		status, retry, err := m.getOnce(ctx, path, out)
		if !retry || attempt >= len(m.pauses) || ctx.Err() != nil {
			return status, err
		}
		select {
		case <-ctx.Done():
			return status, err
		case <-time.After(m.pauses[attempt]):
		}
	}
}

func (m *MetadataLogin) getOnce(ctx context.Context, path string, out any) (status int, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.base+path, nil)
	if err != nil {
		return 0, false, fmt.Errorf("building the metadata request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("%w (%s): %v", errUnreachable, m.base, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBody+1))
	switch {
	case err != nil:
		return resp.StatusCode, true, fmt.Errorf("%w (%s): reading the answer: %v", errUnreachable, m.base, err)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return resp.StatusCode, true, fmt.Errorf("the metadata service answered with HTTP %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return resp.StatusCode, false, nil
	case len(body) > maxMetadataBody:
		return resp.StatusCode, false, fmt.Errorf("the metadata service's answer is larger than %d bytes", maxMetadataBody)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return resp.StatusCode, false, fmt.Errorf("the metadata service's answer is not the expected JSON: %v", err)
	}
	return resp.StatusCode, false, nil
}

func waited(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		if d == time.Minute {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%d seconds", int(d.Round(time.Second).Seconds()))
}

func stackitAPI(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (host == "stackit.cloud" || strings.HasSuffix(host, ".stackit.cloud"))
}

func closeBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}
