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

// Package googlechat renders a message as a Google Chat Cards v2 card for
// a space webhook.
package googlechat

import (
	"context"
	"html"
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

const (
	alertColor = "#d93025"
	mutedColor = "#5f6368"
)

// Notifier posts to a Google Chat space webhook.
type Notifier struct {
	poster notifier.Poster
}

// New returns a Google Chat notifier.
func New(webhookURL string) *Notifier {
	return &Notifier{poster: notifier.NewPoster(webhookURL, "Google Chat")}
}

// Send renders and posts the message.
func (n *Notifier) Send(ctx context.Context, msg notifier.Message) error {
	return n.poster.Post(ctx, Render(msg))
}

type obj = map[string]any

func paragraph(htmlText string) obj {
	return obj{"textParagraph": obj{"text": htmlText}}
}

// Render builds the webhook payload: one card whose text widgets use the
// small HTML subset Google Chat supports.
func Render(msg notifier.Message) map[string]any {
	var sections []obj
	var top []obj
	for _, a := range msg.Alerts {
		top = append(top, paragraph(`<font color="`+alertColor+`"><b>`+html.EscapeString(a)+`</b></font>`))
	}
	if msg.Intro != "" {
		top = append(top, paragraph(html.EscapeString(msg.Intro)))
	}
	if len(top) > 0 {
		sections = append(sections, obj{"widgets": top})
	}
	for _, s := range msg.Sections {
		var lines []string
		for _, l := range s.Lines {
			line := "• " + html.EscapeString(l.Text)
			if l.Link != "" {
				line += ` <a href="` + html.EscapeString(l.Link) + `">open</a>`
			}
			lines = append(lines, line)
		}
		if s.Omitted > 0 {
			lines = append(lines, `<font color="`+mutedColor+`">`+html.EscapeString(notifier.OmittedText(s.Omitted))+`</font>`)
		}
		sections = append(sections, obj{
			"header":  html.EscapeString(s.Title),
			"widgets": []obj{paragraph(strings.Join(lines, "<br>"))},
		})
	}
	if msg.Footer != "" {
		sections = append(sections, obj{"widgets": []obj{
			paragraph(`<font color="` + mutedColor + `">` + html.EscapeString(msg.Footer) + `</font>`),
		}})
	}
	card := obj{"header": obj{"title": msg.Title, "subtitle": msg.Subtitle}, "sections": sections}
	return map[string]any{
		"text":    msg.Title,
		"cardsV2": []obj{{"cardId": "costguard", "card": card}},
	}
}
