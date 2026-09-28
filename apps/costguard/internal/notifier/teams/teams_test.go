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

package teams

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
		Title:    "costguard: 1 resource(s) deleted",
		Subtitle: "Scope: Teams",
		Alerts:   []string{"blocked"},
		Intro:    "intro",
		Sections: []notifier.Section{{
			Title:   "Deleted: 1",
			Lines:   []notifier.Line{{Text: "server web", Link: "https://portal/s"}, {Text: "plain"}},
			Omitted: 4,
		}},
		Footer: "costguard v0.1.0",
	}
}

func TestRenderIsAnAdaptiveCardMessage(t *testing.T) {
	data, _ := json.Marshal(Render(sample()))
	var doc struct {
		Type        string `json:"type"`
		Attachments []struct {
			ContentType string  `json:"contentType"`
			ContentURL  *string `json:"contentUrl"`
			Content     struct {
				Type    string `json:"type"`
				Version string `json:"version"`
				Body    []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					Wrap     bool   `json:"wrap"`
					IsSubtle bool   `json:"isSubtle"`
					Color    string `json:"color"`
				} `json:"body"`
			} `json:"content"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("shape: %v\n%s", err, data)
	}
	att := doc.Attachments[0]
	if doc.Type != "message" || att.ContentType != "application/vnd.microsoft.card.adaptive" || att.Content.Type != "AdaptiveCard" || att.Content.Version != "1.4" {
		t.Errorf("envelope = %s", data)
	}
	var texts []string
	for _, b := range att.Content.Body {
		if b.Type != "TextBlock" || !b.Wrap {
			t.Errorf("block = %+v", b)
		}
		texts = append(texts, b.Text)
	}
	got := strings.Join(texts, "\n")
	want := "costguard: 1 resource(s) deleted\nScope: Teams\n⚠ blocked\nintro\nDeleted: 1\n• server web [open](https://portal/s)\n• plain\n… and 4 more; they will be listed in the next runs.\ncostguard v0.1.0"
	if got != want {
		t.Errorf("texts =\n%s\nwant\n%s", got, want)
	}
	if !att.Content.Body[1].IsSubtle || att.Content.Body[2].Color != "Attention" {
		t.Errorf("styles = %+v", att.Content.Body[:3])
	}
}

func TestSend(t *testing.T) {
	var got string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()
	if err := New(ts.URL).Send(context.Background(), notifier.Message{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"type":"message"`) {
		t.Errorf("body = %s", got)
	}
}
