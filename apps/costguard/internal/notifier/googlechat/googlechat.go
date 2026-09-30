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
	"html"
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

const (
	alertColor = "#d93025"
	mutedColor = "#5f6368"
)

type Notifier struct {
	poster notifier.Poster
}

func New(webhookURL string) *Notifier {
	return &Notifier{poster: notifier.NewPoster(webhookURL, "Google Chat")}
}

func (n *Notifier) Send(ctx context.Context, msg notifier.Message) error {
	return n.poster.Post(ctx, Render(msg))
}

type obj = map[string]any

func paragraph(htmlText string) obj {
	return obj{"textParagraph": obj{"text": htmlText}}
}

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
			line := l.Marker() + html.EscapeString(l.Text)
			if l.Link != "" {
				line += ` <a href="` + html.EscapeString(l.Link) + `">open</a>`
			}
			lines = append(lines, line)
		}
		if s.Omitted > 0 {
			lines = append(lines, `<font color="`+mutedColor+`">`+html.EscapeString(notifier.OmittedText(s.Omitted))+`</font>`)
		}
		sec := obj{"widgets": []obj{paragraph(strings.Join(lines, "<br>"))}}
		if s.Title != "" {
			sec["header"] = html.EscapeString(s.Title)
		}
		if s.Folded {
			sec["collapsible"] = true
			sec["uncollapsibleWidgetsCount"] = 0
		}
		sections = append(sections, sec)
	}
	if msg.Footer != "" {
		sections = append(sections, obj{"widgets": []obj{
			paragraph(`<font color="` + mutedColor + `">` + html.EscapeString(msg.Footer) + `</font>`),
		}})
	}
	card := obj{"header": obj{"title": msg.Title, "subtitle": msg.Subtitle}, "sections": sections}
	return map[string]any{
		"fallbackText": msg.Title,
		"cardsV2":      []obj{{"cardId": "costguard", "card": card}},
	}
}
