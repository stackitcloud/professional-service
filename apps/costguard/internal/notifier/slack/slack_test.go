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

package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

func sample() notifier.Message {
	return notifier.Message{
		Title:    "costguard: 2 resources will be deleted Tuesday",
		Subtitle: "Scope: a<b",
		Alerts:   []string{"blocked & broken"},
		Intro:    "Keep things with do-not-delete=true.",
		Sections: []notifier.Section{{
			Title:   "Idle public IPs: 1",
			Lines:   []notifier.Line{{Text: "public IP 192.0.2.1 · shop (eu01)", Link: "https://portal/x"}, {Text: "no link"}},
			Omitted: 3,
		}},
		Footer: "costguard v0.1.0",
	}
}

func TestRenderFollowsBlockKitRules(t *testing.T) {
	p := Render(sample())
	blocks := p["blocks"].([]block)
	if p["text"] != sample().Title {
		t.Errorf("fallback text = %v", p["text"])
	}
	head := blocks[0]["text"].(block)
	if blocks[0]["type"] != "header" || head["type"] != "plain_text" {
		t.Errorf("header must be plain_text: %v", blocks[0])
	}
	data, _ := json.Marshal(p)
	s := string(data)
	for _, want := range []string{
		`Scope: a\u0026lt;b`,
		`:warning: *blocked \u0026amp; broken*`,
		`• public IP 192.0.2.1 · shop (eu01) \u003chttps://portal/x|open\u003e`,
		`• no link`,
		`_… and 3 more; they will be listed in the next runs._`,
		`"type":"context"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("payload lacks %s:\n%s", want, s)
		}
	}
}

func TestRenderOrganizationAndFoldedSections(t *testing.T) {
	m := sample()
	m.Organization = "Acme"
	m.Sections[0].Folded = true
	p := Render(m)
	if p["text"] != "Acme: costguard: 2 resources will be deleted Tuesday" {
		t.Errorf("fallback text = %v", p["text"])
	}
	data, _ := json.Marshal(p)
	for _, want := range []string{`Acme · Scope: a\u0026lt;b`, `• no link`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("payload lacks %s:\n%s", want, data)
		}
	}
}

func TestRenderRespectsLimits(t *testing.T) {
	msg := notifier.Message{Title: strings.Repeat("ü", 200)}
	for i := 0; i < 60; i++ {
		msg.Sections = append(msg.Sections, notifier.Section{Title: "s", Lines: []notifier.Line{{Text: strings.Repeat("x", 4000)}}})
	}
	blocks := Render(msg)["blocks"].([]block)
	if len(blocks) != 121 {
		t.Errorf("blocks = %d", len(blocks))
	}
	if title := blocks[0]["text"].(block)["text"].(string); len(title) > maxHeaderChars || !strings.HasSuffix(title, "…") {
		t.Errorf("header not truncated on a rune boundary: %d %q", len(title), title[len(title)-5:])
	}
	for _, b := range blocks {
		if b["type"] == "section" {
			if n := len(b["text"].(block)["text"].(string)); n > maxTextChars {
				t.Errorf("section text too long: %d", n)
			}
		}
	}
	for _, b := range blocks {
		if strings.Contains(fmt.Sprint(b), "shortened") {
			t.Errorf("no block may be dropped or replaced: %v", b)
		}
	}
}

func TestChunks(t *testing.T) {
	got := chunks([]string{"aaaa", "bbbb", "cc"}, 9)
	if len(got) != 2 || got[0] != "aaaa\nbbbb" || got[1] != "cc" {
		t.Errorf("chunks = %q", got)
	}
}

func TestSend(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
	}))
	defer ts.Close()
	if err := New(ts.URL).Send(context.Background(), sample()); err != nil {
		t.Fatal(err)
	}
	if body["blocks"] == nil {
		t.Errorf("body = %v", body)
	}
}
