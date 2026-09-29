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

// Package notifier turns reports into chat-neutral messages (Compose) and
// posts them. The slack, googlechat and teams packages render a Message in
// their platform's format.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message is a chat-neutral message.
type Message struct {
	Title    string
	Subtitle string
	// Alerts are shown first and highlighted.
	Alerts []string
	// Intro is a short paragraph under the alerts.
	Intro    string
	Sections []Section
	Footer   string
}

// Section is a titled list.
type Section struct {
	Title string
	Lines []Line
	// Omitted counts entries not listed in this message; they come up in
	// the next runs. Nothing that is acted on is ever omitted.
	Omitted int
}

// Line is one list entry with an optional link.
type Line struct {
	Text string
	Link string
}

// OmittedText is the text renderers show for omitted entries.
func OmittedText(n int) string {
	return fmt.Sprintf("… and %d more; they will be listed in the next runs.", n)
}

// Notifier delivers a message.
type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

// Poster posts JSON payloads to a webhook.
type Poster struct {
	URL    string
	Client *http.Client
	// Name is the platform name used in errors.
	Name string
}

// attemptTimeout bounds one POST; three attempts plus the pauses fit into
// the 45 s send budget of a run.
const attemptTimeout = 12 * time.Second

// RetryPauses are the waits between attempts. Only rate limiting (429),
// server errors (5xx) and network errors are retried: a one-second hiccup
// of the chat service must not cost a run's message (and, with delete on,
// that run's labels). A retry after a timeout that did arrive after all posts
// the message twice, which is the lesser evil. Tests shorten the pauses.
var RetryPauses = []time.Duration{2 * time.Second, 5 * time.Second}

// NewPoster returns a Poster with a per-attempt timeout.
func NewPoster(url, name string) Poster {
	return Poster{URL: url, Client: &http.Client{Timeout: attemptTimeout}, Name: name}
}

// Post sends payload as JSON and fails on any non-2xx answer, after
// retrying the transient ones.
func (p Poster) Post(ctx context.Context, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding the %s message: %w", p.Name, err)
	}
	for attempt := 0; ; attempt++ {
		retry, err := p.post(ctx, data)
		if err == nil || !retry || attempt >= len(RetryPauses) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(RetryPauses[attempt]):
		}
	}
}

// post makes one attempt and says whether a failure is worth retrying.
func (p Poster) post(ctx context.Context, data []byte) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("building the %s request: %w", p.Name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		// The URL is a secret: never include it in the error.
		return ctx.Err() == nil, fmt.Errorf("posting to the %s webhook failed", p.Name)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		transient := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return transient, fmt.Errorf("the %s webhook answered %s: %s", p.Name, resp.Status, bytes.TrimSpace(body))
	}
	return false, nil
}
