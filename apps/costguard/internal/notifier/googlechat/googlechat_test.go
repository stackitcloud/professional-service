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

package googlechat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

func sample() notifier.Message {
	return notifier.Message{
		Title:    "costguard report",
		Subtitle: "Scope: Teams",
		Alerts:   []string{"a <b> alert"},
		Intro:    "Nothing was changed.",
		Sections: []notifier.Section{{
			Title:   "Idle public IPs: 1",
			Lines:   []notifier.Line{{Text: "public IP 192.0.2.1", Link: "https://portal/x?a=1&b=2"}, {Text: "plain"}},
			Omitted: 2,
		}},
		Footer: "costguard v0.1.0",
	}
}

func TestRenderIsCardsV2(t *testing.T) {
	p := Render(sample())
	data, _ := json.Marshal(p)

	// Decode into the documented shape: cardsV2 is an array of
	// {cardId, card}; section headers are strings.
	var doc struct {
		Text    string `json:"text"`
		CardsV2 []struct {
			CardID string `json:"cardId"`
			Card   struct {
				Header struct {
					Title    string `json:"title"`
					Subtitle string `json:"subtitle"`
				} `json:"header"`
				Sections []struct {
					Header  string `json:"header"`
					Widgets []struct {
						TextParagraph struct {
							Text string `json:"text"`
						} `json:"textParagraph"`
					} `json:"widgets"`
				} `json:"sections"`
			} `json:"card"`
		} `json:"cardsV2"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("payload does not match the Cards v2 shape: %v\n%s", err, data)
	}
	card := doc.CardsV2[0].Card
	if doc.CardsV2[0].CardID != "costguard" || card.Header.Title != "costguard report" || card.Header.Subtitle != "Scope: Teams" {
		t.Errorf("header = %+v", doc.CardsV2[0])
	}
	if len(card.Sections) != 3 {
		t.Fatalf("sections = %+v", card.Sections)
	}
	top := card.Sections[0].Widgets
	if !strings.Contains(top[0].TextParagraph.Text, "a &lt;b&gt; alert") || top[1].TextParagraph.Text != "Nothing was changed." {
		t.Errorf("top = %+v", top)
	}
	list := card.Sections[1]
	if list.Header != "Idle public IPs: 1" {
		t.Errorf("section header = %q", list.Header)
	}
	want := `• public IP 192.0.2.1 <a href="https://portal/x?a=1&amp;b=2">open</a><br>• plain<br>`
	if !strings.HasPrefix(list.Widgets[0].TextParagraph.Text, want) || !strings.Contains(list.Widgets[0].TextParagraph.Text, "… and 2 more") {
		t.Errorf("lines = %q", list.Widgets[0].TextParagraph.Text)
	}
	if !strings.Contains(card.Sections[2].Widgets[0].TextParagraph.Text, "costguard v0.1.0") {
		t.Errorf("footer = %+v", card.Sections[2])
	}
}

func TestRenderMinimal(t *testing.T) {
	p := Render(notifier.Message{Title: "t"})
	card := p["cardsV2"].([]obj)[0]["card"].(obj)
	if sections, _ := card["sections"].([]obj); len(sections) != 0 {
		t.Errorf("sections = %v", sections)
	}
}

func TestSend(t *testing.T) {
	var got string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	defer ts.Close()
	if err := New(ts.URL).Send(context.Background(), sample()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"cardsV2":[`) {
		t.Errorf("body = %s", got)
	}
}
