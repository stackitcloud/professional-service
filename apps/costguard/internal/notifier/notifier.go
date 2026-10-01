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

type Message struct {
	Title    string
	Subtitle string
	Alerts   []string
	Intro    string
	Sections []Section
	Footer   string
}

type Section struct {
	Title   string
	Lines   []Line
	Omitted int
	Folded  bool
}

type Line struct {
	Text   string
	Link   string
	Number int
}

func (l Line) Marker() string {
	if l.Number > 0 {
		return fmt.Sprintf("%d. ", l.Number)
	}
	return "• "
}

func OmittedText(n int) string {
	return fmt.Sprintf("… and %d more; they will be listed in the next runs.", n)
}

type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

type Poster struct {
	URL    string
	Client *http.Client
	Name   string
}

const attemptTimeout = 12 * time.Second

var RetryPauses = []time.Duration{2 * time.Second, 5 * time.Second}

func NewPoster(url, name string) Poster {
	return Poster{URL: url, Client: &http.Client{Timeout: attemptTimeout}, Name: name}
}

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

func (p Poster) post(ctx context.Context, data []byte) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("building the %s request: %w", p.Name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
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
