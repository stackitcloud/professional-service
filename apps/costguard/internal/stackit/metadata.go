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

// EnvServiceAccountEmail names the service account attached to the server.
// Terraform writes it; the login only uses this account, whatever else
// might be attached.
const EnvServiceAccountEmail = "COSTGUARD_SERVICE_ACCOUNT_EMAIL"

const (
	// metadataTimeout bounds one request to the metadata service. STACKIT
	// documents that such requests "may occasionally take up to 30 seconds".
	metadataTimeout = 40 * time.Second
	// refreshMargin: a token is replaced this long before it expires, so a
	// request never starts with a token that runs out on the way. Tokens
	// live one hour.
	refreshMargin = 5 * time.Minute
	// refreshBackoff: after a failed refresh, the old token is used without
	// asking again for this long, so an outage of the metadata service does
	// not slow down every request.
	refreshBackoff = 30 * time.Second
	// maxMetadataBody caps what is read from the metadata service. The real
	// answers are about 1.5 KB.
	maxMetadataBody = 64 << 10
	// attachPoll is the pause between checks while Ready waits for the
	// service account to be attached.
	attachPoll = 5 * time.Second
)

// Replaced in tests.
var (
	metadataURL                    = "http://169.254.169.254"
	apiTransport http.RoundTripper = http.DefaultTransport
	// metadataPauses are the waits between attempts at the metadata
	// service. Only network errors, 429 and 5xx are retried.
	metadataPauses = []time.Duration{2 * time.Second, 5 * time.Second}
)

var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+@[A-Za-z0-9.-]+$`)

// errNotAttached and errUnreachable are what Ready waits out: right after
// the server was created, its service account is attached a few seconds
// later.
var (
	errNotAttached = errors.New("no service account is attached to this server")
	errUnreachable = errors.New("the metadata service did not answer")
)

// MetadataLogin logs in with the service account attached to the server.
// It gets short-lived tokens from the STACKIT metadata service
// (169.254.169.254) and adds them to the requests of every SDK client, so
// no key or token is stored anywhere. It never calls the metadata
// service's S3 credentials endpoint, which would create an Object Storage
// identity mapping.
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

// NewMetadataLogin returns the login for the attached service account
// with this email.
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
			// Never through a proxy: HTTP(S)_PROXY would otherwise apply
			// to 169.254.169.254 too (Go only exempts loopback).
			Transport: &http.Transport{
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				ResponseHeaderTimeout: metadataTimeout,
				MaxIdleConns:          1,
				IdleConnTimeout:       30 * time.Second,
			},
			// The metadata service does not redirect; a redirect is an
			// error, not something to follow.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		next:   apiTransport,
		now:    time.Now,
		pauses: metadataPauses,
		poll:   attachPoll,
	}, nil
}

// Email is the service account this login uses.
func (m *MetadataLogin) Email() string {
	return m.email
}

// RoundTrip adds the token to a request for a STACKIT API. It refuses
// every other destination, so a redirect or a wrong endpoint can never
// carry the token elsewhere.
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

// Ready checks that a token can be had. While the service account is not
// attached yet, or the metadata service does not answer, it keeps trying
// for up to wait.
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

// Token returns a valid token, from the cache while it has more than
// refreshMargin left. Every call to the metadata service mints a new token,
// so caching matters.
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
			// The old token works for a few more minutes; the next
			// request after the backoff tries again.
			m.retryAfter = now.Add(refreshBackoff)
			return m.token, nil
		}
		return "", err
	}
	m.token, m.validUntil, m.retryAfter = token, until, time.Time{}
	return token, nil
}

// forget drops a token the API refused, so the next request gets a new one.
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

// explain turns the token endpoint's 404, which is the same for "nothing
// attached" and "a different service account attached", into a readable
// error by reading the list of attached service accounts.
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

// get reads path from the metadata service and decodes a 200 answer into
// out. Network errors, 429 and 5xx are retried with m.pauses in between.
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

// waited renders a wait such as "5 minutes" or "30 seconds".
func waited(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		if d == time.Minute {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%d seconds", int(d.Round(time.Second).Seconds()))
}

// stackitAPI reports whether u is a STACKIT API: https on stackit.cloud or
// one of its subdomains.
func stackitAPI(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (host == "stackit.cloud" || strings.HasSuffix(host, ".stackit.cloud"))
}

// closeBody honours the RoundTripper contract: the request body is closed
// even when the request is not sent.
func closeBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}
