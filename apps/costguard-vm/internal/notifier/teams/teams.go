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

// Package teams renders a message as an Adaptive Card for a Teams
// Workflows webhook ("Post to a channel when a webhook request is
// received").
package teams

import (
	"context"

	"github.com/stackitcloud/professional-service/apps/costguard-vm/internal/notifier"
)

// Notifier posts to a Teams Workflows webhook.
type Notifier struct {
	poster notifier.Poster
}

// New returns a Teams notifier.
func New(webhookURL string) *Notifier {
	return &Notifier{poster: notifier.NewPoster(webhookURL, "Teams")}
}

// Send renders and posts the message.
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

// Render builds the webhook payload.
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
	for _, s := range msg.Sections {
		body = append(body, textBlock(s.Title, obj{"weight": "Bolder", "spacing": "Medium"}))
		for _, l := range s.Lines {
			line := "• " + l.Text
			if l.Link != "" {
				line += " [open](" + l.Link + ")"
			}
			body = append(body, textBlock(line, obj{"spacing": "None"}))
		}
		if s.Omitted > 0 {
			body = append(body, textBlock(notifier.OmittedText(s.Omitted), obj{"isSubtle": true, "spacing": "None"}))
		}
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
