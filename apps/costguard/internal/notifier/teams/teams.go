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
	"fmt"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

type Notifier struct {
	poster notifier.Poster
}

func New(webhookURL string) *Notifier {
	return &Notifier{poster: notifier.NewPoster(webhookURL, "Teams")}
}

func (n *Notifier) Send(ctx context.Context, msg notifier.Message) error {
	return n.poster.Post(ctx, Render(msg))
}

type obj = map[string]any

func textBlock(text string, props obj) obj {
	b := obj{"type": "TextBlock", "text": text, "wrap": true}
	for k, v := range props {
		b[k] = v
	}
	return b
}

func Render(msg notifier.Message) map[string]any {
	body := []obj{textBlock(msg.Title, obj{"size": "Large", "weight": "Bolder"})}
	if msg.Subtitle != "" {
		body = append(body, textBlock(msg.Subtitle, obj{"isSubtle": true, "spacing": "None"}))
	}
	for _, a := range msg.Alerts {
		body = append(body, textBlock("⚠ "+a, obj{"color": "Attention", "weight": "Bolder"}))
	}
	if msg.Intro != "" {
		body = append(body, textBlock(msg.Intro, nil))
	}
	for i, s := range msg.Sections {
		if s.Title != "" {
			body = append(body, textBlock(s.Title, obj{"weight": "Bolder", "spacing": "Medium"}))
		}
		var lines []obj
		for _, l := range s.Lines {
			line := l.Marker() + l.Text
			if l.Link != "" {
				line += " [open](" + l.Link + ")"
			}
			lines = append(lines, textBlock(line, obj{"spacing": "None"}))
		}
		if s.Omitted > 0 {
			lines = append(lines, textBlock(notifier.OmittedText(s.Omitted), obj{"isSubtle": true, "spacing": "None"}))
		}
		if !s.Folded {
			if s.Title == "" && len(lines) > 0 {
				lines[0]["spacing"] = "Medium"
			}
			body = append(body, lines...)
			continue
		}
		id := fmt.Sprintf("section-%d", i)
		toggle := obj{"type": "Action.ToggleVisibility", "targetElements": []string{id + "-more", id, id + "-less"}}
		link := func(suffix, text string, visible bool) obj {
			return obj{"type": "Container", "id": id + suffix, "isVisible": visible, "spacing": "None", "selectAction": toggle,
				"items": []obj{textBlock(text, obj{"color": "Accent", "spacing": "None"})}}
		}
		body = append(body,
			link("-more", "Show more", true),
			obj{"type": "Container", "id": id, "isVisible": false, "spacing": "None", "items": lines},
			link("-less", "Show less", false),
		)
	}
	if msg.Footer != "" {
		body = append(body, textBlock(msg.Footer, obj{"isSubtle": true, "size": "Small", "spacing": "Medium"}))
	}
	return map[string]any{
		"type": "message",
		"attachments": []obj{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"contentUrl":  nil,
			"content": obj{
				"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
				"type":    "AdaptiveCard",
				"version": "1.4",
				"msteams": obj{"width": "Full"},
				"body":    body,
			},
		}},
	}
}
