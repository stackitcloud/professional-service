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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const saEmail = "costguard-vm-a1b2c3d4@sa.stackit.cloud"

// notFoundBody is what the real metadata service answers for an unknown
// path, an unknown service account and a detached one (spike, 2026-09-29).
const notFoundBody = `{"code": "404 Not Found", "message": "The resource could not be found.<br /><br />\n\n\n", "title": "Not Found"}`

// fakeMetadata answers like STACKIT's metadata service: every token call
// mints a new token.
type fakeMetadata struct {
	mu       sync.Mutex
	attached []string
	now      func() time.Time
	lifetime time.Duration
	// tokenStatus is answered to the next token requests, one per request,
	// before normal service resumes.
	tokenStatus []int
	listStatus  int
	// tokenBody replaces the token answer when set.
	tokenBody string
	minted    int
	requests  []string
}

func newFakeMetadata(now func() time.Time) *fakeMetadata {
	return &fakeMetadata{attached: []string{saEmail}, now: now, lifetime: time.Hour}
}

func (f *fakeMetadata) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	notFound := func() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, notFoundBody)
	}
	path := r.URL.Path
	switch {
	case path == "/stackit/v1/service-accounts":
		if f.listStatus != 0 {
			w.WriteHeader(f.listStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string][]string{"serviceAccountMails": append([]string{}, f.attached...)})
	case strings.HasPrefix(path, "/stackit/v1/service-accounts/") && strings.HasSuffix(path, "/token"):
		if len(f.tokenStatus) > 0 {
			status := f.tokenStatus[0]
			f.tokenStatus = f.tokenStatus[1:]
			if status == http.StatusNotFound {
				notFound()
				return
			}
			w.WriteHeader(status)
			return
		}
		email := strings.TrimSuffix(strings.TrimPrefix(path, "/stackit/v1/service-accounts/"), "/token")
		found := false
		for _, a := range f.attached {
			found = found || a == email
		}
		if !found {
			notFound()
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if f.tokenBody != "" {
			_, _ = io.WriteString(w, f.tokenBody)
			return
		}
		f.minted++
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":      fmt.Sprintf("metadata-token-%d", f.minted),
			"validUntil": f.now().Add(f.lifetime).UTC().Format(time.RFC3339Nano),
		})
	default:
		notFound()
	}
}

func (f *fakeMetadata) mintedTokens() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.minted
}

func (f *fakeMetadata) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

// apiRecorder stands in for the STACKIT APIs behind the login.
type apiRecorder struct {
	mu      sync.Mutex
	auth    []string
	status  []int
	answers func(req *http.Request) (int, string)
}

func (a *apiRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	a.mu.Lock()
	a.auth = append(a.auth, req.Header.Get("Authorization"))
	status, body := http.StatusOK, `{}`
	if len(a.status) > 0 {
		status, a.status = a.status[0], a.status[1:]
	}
	answers := a.answers
	a.mu.Unlock()
	if answers != nil {
		status, body = answers(req)
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (a *apiRecorder) seen() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.auth...)
}

// clock is a settable time source.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type loginEnv struct {
	login *MetadataLogin
	meta  *fakeMetadata
	api   *apiRecorder
	clock *clock
}

func newLoginEnv(t *testing.T) *loginEnv {
	t.Helper()
	c := &clock{t: time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)}
	meta := newFakeMetadata(c.now)
	ts := httptest.NewServer(meta)
	t.Cleanup(ts.Close)
	login, err := NewMetadataLogin(saEmail)
	if err != nil {
		t.Fatal(err)
	}
	api := &apiRecorder{}
	login.base, login.next, login.now = ts.URL, api, c.now
	login.pauses, login.poll = []time.Duration{time.Millisecond, time.Millisecond}, time.Millisecond
	return &loginEnv{login: login, meta: meta, api: api, clock: c}
}

func (e *loginEnv) call(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := e.login.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	return resp, err
}

const iaasURL = "https://iaas.api.stackit.cloud/v2/projects/p1/regions/eu01/servers"

func TestMetadataLoginCachesTheToken(t *testing.T) {
	e := newLoginEnv(t)
	for i := 0; i < 3; i++ {
		if _, err := e.call(t, iaasURL); err != nil {
			t.Fatal(err)
		}
		e.clock.add(10 * time.Minute)
	}
	if got := e.api.seen(); len(got) != 3 || got[0] != "Bearer metadata-token-1" || got[2] != "Bearer metadata-token-1" {
		t.Errorf("Authorization = %v, want the first token three times", got)
	}
	if n := e.meta.count("GET /stackit/v1/service-accounts/" + saEmail + "/token"); n != 1 {
		t.Errorf("token requests = %d, want 1 (every call mints a new token)", n)
	}
}

func TestMetadataLoginRefreshesBeforeTheTokenExpires(t *testing.T) {
	e := newLoginEnv(t)
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	e.clock.add(time.Hour - refreshMargin + time.Second)
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	if got := e.api.seen(); got[1] != "Bearer metadata-token-2" {
		t.Errorf("Authorization = %v, want a new token inside the refresh margin", got)
	}
}

func TestMetadataLoginKeepsTheOldTokenWhileARefreshFails(t *testing.T) {
	e := newLoginEnv(t)
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	e.clock.add(time.Hour - 2*time.Minute)
	e.meta.tokenStatus = []int{500, 500, 500}
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatalf("the old token is still valid for 2 minutes: %v", err)
	}
	// Within the backoff, the metadata service is not asked again.
	before := e.meta.count("GET")
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	if after := e.meta.count("GET"); after != before {
		t.Errorf("metadata requests during the backoff: %d, want none", after-before)
	}
	if got := e.api.seen(); got[1] != "Bearer metadata-token-1" || got[2] != "Bearer metadata-token-1" {
		t.Errorf("Authorization = %v", got)
	}
	// After the backoff it tries again and gets a fresh token.
	e.clock.add(refreshBackoff)
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	if got := e.api.seen(); got[3] != "Bearer metadata-token-2" {
		t.Errorf("Authorization = %v, want a new token after the backoff", got)
	}
}

func TestMetadataLoginFailsOnceTheOldTokenExpired(t *testing.T) {
	e := newLoginEnv(t)
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	e.clock.add(time.Hour)
	e.meta.tokenStatus = []int{503, 503, 503}
	_, err := e.call(t, iaasURL)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("err = %v, want the metadata error once the old token expired", err)
	}
	if n := len(e.api.seen()); n != 1 {
		t.Errorf("API requests = %d, want none without a token", n)
	}
}

func TestMetadataLoginExplainsA404(t *testing.T) {
	for name, tc := range map[string]struct {
		attached    []string
		tokenStatus []int
		listStatus  int
		want        string
		notAttached bool
	}{
		"nothing attached": {
			attached: nil, want: "no service account is attached to this server: Terraform attaches " + saEmail, notAttached: true,
		},
		"another account": {
			attached: []string{"someone-else@sa.stackit.cloud"},
			want:     "this server has someone-else@sa.stackit.cloud attached, not costguard's service account " + saEmail,
		},
		"attached but no token": {
			attached: []string{saEmail}, tokenStatus: []int{404},
			want: saEmail + " is attached to this server, but the metadata service issued no token for it",
		},
		"list fails too": {
			tokenStatus: []int{404}, listStatus: http.StatusForbidden,
			want: "answered the list of service accounts with HTTP 403",
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := newLoginEnv(t)
			e.meta.attached, e.meta.tokenStatus, e.meta.listStatus = tc.attached, tc.tokenStatus, tc.listStatus
			_, err := e.login.Token(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if errors.Is(err, errNotAttached) != tc.notAttached {
				t.Errorf("errors.Is(err, errNotAttached) = %v", !tc.notAttached)
			}
		})
	}
}

func TestMetadataLoginRetriesTransientAnswers(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.tokenStatus = []int{503, 429}
	token, err := e.login.Token(context.Background())
	if err != nil || token != "metadata-token-1" {
		t.Fatalf("Token = %q, %v", token, err)
	}
	if n := e.meta.count("GET /stackit/v1/service-accounts/"); n != 3 {
		t.Errorf("token requests = %d, want 3", n)
	}
}

func TestMetadataLoginDoesNotRetryOtherAnswers(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.tokenStatus = []int{403}
	_, err := e.login.Token(context.Background())
	if err == nil || !strings.Contains(err.Error(), "answered the token request with HTTP 403") {
		t.Fatalf("err = %v", err)
	}
	if n := e.meta.count("GET"); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestMetadataLoginReportsAnUnreachableService(t *testing.T) {
	e := newLoginEnv(t)
	ts := httptest.NewServer(http.NotFoundHandler())
	ts.Close()
	e.login.base = ts.URL
	_, err := e.login.Token(context.Background())
	if !errors.Is(err, errUnreachable) || !strings.Contains(err.Error(), "the metadata service did not answer") {
		t.Fatalf("err = %v", err)
	}
}

func TestMetadataLoginRejectsBadAnswers(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"not JSON":        {`<html>`, "not the expected JSON"},
		"no token":        {`{"validUntil": "2026-09-29T09:00:00Z"}`, "without a token"},
		"bad validUntil":  {`{"token": "t", "validUntil": "tomorrow"}`, `unreadable validUntil "tomorrow"`},
		"expired":         {`{"token": "t", "validUntil": "2026-09-29T07:59:59Z"}`, "a token that expired at 2026-09-29T07:59:59Z"},
		"far too large":   {`{"token": "` + strings.Repeat("x", maxMetadataBody) + `"}`, "larger than 65536 bytes"},
		"nanosecond time": {`{"token": "t", "validUntil": "2026-09-29T09:08:35.715434666Z"}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			e := newLoginEnv(t)
			e.meta.tokenBody = tc.body
			_, err := e.login.Token(context.Background())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), strings.Repeat("x", 100)) {
				t.Error("the error repeats the answer")
			}
		})
	}
}

func TestMetadataLoginFollowsNoRedirects(t *testing.T) {
	e := newLoginEnv(t)
	elsewhere := httptest.NewServer(e.meta)
	t.Cleanup(elsewhere.Close)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(redirect.Close)
	e.login.base = redirect.URL
	_, err := e.login.Token(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("err = %v, want the redirect refused", err)
	}
	if n := e.meta.count("GET"); n != 0 {
		t.Errorf("the redirect was followed (%d requests)", n)
	}
}

func TestMetadataClientIgnoresProxies(t *testing.T) {
	login, err := NewMetadataLogin(saEmail)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := login.client.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil {
		t.Errorf("the metadata client must not use a proxy: %#v", login.client.Transport)
	}
	if login.base != "http://169.254.169.254" || login.next != http.DefaultTransport {
		t.Errorf("base = %s, next = %T", login.base, login.next)
	}
}

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}

func TestMetadataLoginSendsTheTokenOnlyToSTACKIT(t *testing.T) {
	for _, url := range []string{
		"http://iaas.api.stackit.cloud/v2/projects",
		"https://evil.example/v2/projects",
		"https://stackit.cloud.evil.example/",
		"https://evilstackit.cloud/",
		"https://127.0.0.1/",
	} {
		e := newLoginEnv(t)
		body := &closeTracker{Reader: strings.NewReader("{}")}
		req, err := http.NewRequest(http.MethodPost, url, body)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.login.RoundTrip(req)
		if err == nil || !strings.Contains(err.Error(), "only to https://*.stackit.cloud") {
			t.Errorf("%s: err = %v", url, err)
		}
		if !body.closed {
			t.Errorf("%s: request body not closed", url)
		}
		if len(e.api.seen()) != 0 || e.meta.count("GET") != 0 {
			t.Errorf("%s: sent anyway", url)
		}
	}
	e := newLoginEnv(t)
	for _, url := range []string{iaasURL, "https://STACKIT.cloud/", "https://resource-manager.api.stackit.cloud/v2/projects"} {
		if _, err := e.call(t, url); err != nil {
			t.Errorf("%s: %v", url, err)
		}
	}
}

func TestMetadataLoginClosesTheBodyWhenThereIsNoToken(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.attached = nil
	body := &closeTracker{Reader: strings.NewReader("{}")}
	req, err := http.NewRequest(http.MethodPatch, iaasURL, body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.login.RoundTrip(req); !errors.Is(err, errNotAttached) {
		t.Fatalf("err = %v", err)
	}
	if !body.closed {
		t.Error("request body not closed")
	}
}

func TestMetadataLoginDropsARefusedToken(t *testing.T) {
	e := newLoginEnv(t)
	e.api.status = []int{http.StatusUnauthorized}
	resp, err := e.call(t, iaasURL)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("resp = %v, err = %v", resp, err)
	}
	if _, err := e.call(t, iaasURL); err != nil {
		t.Fatal(err)
	}
	if got := e.api.seen(); got[1] != "Bearer metadata-token-2" {
		t.Errorf("Authorization = %v, want a new token after a 401", got)
	}
}

func TestMetadataLoginMintsOneTokenForConcurrentRequests(t *testing.T) {
	e := newLoginEnv(t)
	var wg sync.WaitGroup
	var failed atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, iaasURL, nil)
			if resp, err := e.login.RoundTrip(req); err != nil {
				failed.Add(1)
			} else {
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	if minted := e.meta.mintedTokens(); failed.Load() != 0 || minted != 1 {
		t.Errorf("failed = %d, minted = %d, want 0 and 1", failed.Load(), minted)
	}
}

func TestNewMetadataLoginChecksTheEmail(t *testing.T) {
	for _, email := range []string{"", "   ", "no-at-sign", "a/b@sa.stackit.cloud", "a@sa.stackit.cloud/../x", "a b@sa.stackit.cloud", "a@sa.stackit.cloud?x"} {
		if _, err := NewMetadataLogin(email); err == nil || !strings.Contains(err.Error(), EnvServiceAccountEmail) {
			t.Errorf("%q: err = %v", email, err)
		}
	}
	login, err := NewMetadataLogin("  " + saEmail + "\n")
	if err != nil || login.Email() != saEmail {
		t.Fatalf("login = %v, err = %v", login, err)
	}
}

func TestReadyWaitsForTheAttachment(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.attached = nil
	calls := 0
	e.login.now = func() time.Time {
		// Attach the account on the third check, as right after boot.
		calls++
		if calls == 3 {
			e.meta.mu.Lock()
			e.meta.attached = []string{saEmail}
			e.meta.mu.Unlock()
		}
		return e.clock.now()
	}
	if err := e.login.Ready(context.Background(), 5*time.Minute); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if n := e.meta.count("GET /stackit/v1/service-accounts/"); n < 2 {
		t.Errorf("token requests = %d, want it to have waited", n)
	}
}

func TestReadyGivesUpAfterTheWait(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.attached = nil
	e.login.now = func() time.Time {
		e.clock.add(time.Minute)
		return e.clock.now()
	}
	err := e.login.Ready(context.Background(), 5*time.Minute)
	if !errors.Is(err, errNotAttached) || !strings.Contains(err.Error(), "(still after waiting 5 minutes)") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadyWithoutWaitAndForLastingProblems(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.attached = nil
	if err := e.login.Ready(context.Background(), 0); !errors.Is(err, errNotAttached) || strings.Contains(err.Error(), "waiting") {
		t.Fatalf("err = %v, want an immediate error", err)
	}
	e = newLoginEnv(t)
	e.meta.attached = []string{"someone-else@sa.stackit.cloud"}
	if err := e.login.Ready(context.Background(), time.Hour); err == nil || !strings.Contains(err.Error(), "someone-else") {
		t.Fatalf("err = %v", err)
	}
	if n := e.meta.count("GET /stackit/v1/service-accounts/"); n != 1 {
		t.Errorf("token requests = %d, want no waiting for a different account", n)
	}
}

func TestReadyStopsWhenCancelled(t *testing.T) {
	e := newLoginEnv(t)
	e.meta.attached = nil
	e.login.poll = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	done := make(chan error, 1)
	go func() { done <- e.login.Ready(ctx, time.Hour) }()
	select {
	case err := <-done:
		if !errors.Is(err, errNotAttached) {
			t.Errorf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ready did not stop when cancelled")
	}
}

func TestWaited(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Minute:  "5 minutes",
		time.Minute:      "1 minute",
		90 * time.Second: "90 seconds",
		30 * time.Second: "30 seconds",
	} {
		if got := waited(d); got != want {
			t.Errorf("waited(%s) = %q, want %q", d, got, want)
		}
	}
}
