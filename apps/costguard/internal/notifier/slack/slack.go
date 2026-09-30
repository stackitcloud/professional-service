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
	"strings"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
)

const (
	maxHeaderChars = 150
	maxTextChars   = 3000
)

type Notifier struct {
	poster notifier.Poster
}

func New(webhookURL string) *Notifier {
	return &Notifier{poster: notifier.NewPoster(webhookURL, "Slack")}
}

func (n *Notifier) Send(ctx context.Context, msg notifier.Message) error {
	return n.poster.Post(ctx, Render(msg))
}

type block = map[string]any

func text(t string) block    { return block{"type": "mrkdwn", "text": t} }
func section(t string) block { return block{"type": "section", "text": text(t)} }
func contextBlock(t string) block {
	return block{"type": "context", "elements": []block{text(t)}}
}

func Render(msg notifier.Message) map[string]any {
	blocks := []block{
		{"type": "header", "text": block{"type": "plain_text", "text": truncate(msg.Title, maxHeaderChars)}},
	}
	if sub := msg.HeaderSubtitle(); sub != "" {
		blocks = append(blocks, contextBlock(escape(sub)))
	}
	for _, a := range msg.Alerts {
		blocks = append(blocks, section(truncate(":warning: *"+escape(a)+"*", maxTextChars)))
	}
	if msg.Intro != "" {
		blocks = append(blocks, section(truncate(escape(msg.Intro), maxTextChars)))
	}
	for _, s := range msg.Sections {
		lines := []string{"*" + escape(s.Title) + "*"}
		for _, l := range s.Lines {
			line := "• " + escape(l.Text)
			if l.Link != "" {
				line += " <" + l.Link + "|open>"
			}
			lines = append(lines, line)
		}
		if s.Omitted > 0 {
			lines = append(lines, "_"+escape(notifier.OmittedText(s.Omitted))+"_")
		}
		for _, chunk := range chunks(lines, maxTextChars) {
			blocks = append(blocks, section(chunk))
		}
	}
	if msg.Footer != "" {
		blocks = append(blocks, contextBlock(escape(msg.Footer)))
	}
	return map[string]any{"text": msg.Notification(), "blocks": blocks}
}

func chunks(lines []string, max int) []string {
	var out []string
	var cur strings.Builder
	for _, l := range lines {
		l = truncate(l, max)
		if cur.Len() > 0 && cur.Len()+1+len(l) > max {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(l)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max - len("…")
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
