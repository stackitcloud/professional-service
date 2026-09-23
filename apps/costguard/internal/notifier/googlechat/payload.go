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

// Package googlechat renders the costguard report as a Google Chat
// Cards V2 message and POSTs it to the space webhook. The
// payload is plain JSON — the chat/v1 API client is intentionally not
// used (the webhook is plain HTTP).
package googlechat

// message is the webhook envelope.
type message struct {
	CardsV2 *cardsV2 `json:"cardsV2,omitempty"`
}

type cardsV2 struct {
	Header   *header   `json:"header,omitempty"`
	Sections []section `json:"sections,omitempty"`
}

type header struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// section is one (optionally collapsible) card section.
type section struct {
	Header              *header    `json:"header,omitempty"`
	CollapsibleSection  bool       `json:"collapsibleSection,omitempty"`
	HeaderBarProperties *headerBar `json:"headerBarProperties,omitempty"`
	Widgets             []widget   `json:"widgets,omitempty"`
}

type headerBar struct {
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

// widget is a oneof: exactly one field is set per widget.
type widget struct {
	TextParagraph *textParagraph `json:"textParagraph,omitempty"`
	KeyValueList  *keyValueList  `json:"keyValueList,omitempty"`
	ButtonList    *buttonList    `json:"buttonList,omitempty"`
	MediaInfo     *mediaInfo     `json:"mediaInfo,omitempty"`
	Divider       *struct{}      `json:"divider,omitempty"`
}

func wText(text string, weight, size, color string) widget {
	t := &textContent{Text: text}
	if weight != "" || size != "" || color != "" {
		t.TextTheme = &textTheme{Weight: weight, Size: size, ColorHex: color}
	}
	return widget{TextParagraph: &textParagraph{Elements: []element{{TextContent: t}}}}
}

type textParagraph struct {
	Elements []element `json:"elements"`
}

type element struct {
	TextContent *textContent `json:"textContent,omitempty"`
}

type textContent struct {
	Text      string     `json:"text"`
	TextTheme *textTheme `json:"textTheme,omitempty"`
}

type textTheme struct {
	Weight   string `json:"weight,omitempty"`
	Size     string `json:"size,omitempty"`
	ColorHex string `json:"colorHex,omitempty"`
}

type keyValueList struct {
	ColumnProperties []colProps `json:"columnProperties,omitempty"`
	Columns          []kvColumn `json:"columns"`
}

type colProps struct {
	Alignment string  `json:"alignment,omitempty"`
	Weight    float64 `json:"weight,omitempty"`
}

type kvColumn struct {
	TopLabel    *kv `json:"topLabel,omitempty"`
	Content     *kv `json:"content,omitempty"`
	BottomLabel *kv `json:"bottomLabel,omitempty"`
}

type kv struct {
	Text string `json:"text"`
}

func kvText(s string) *kv { return &kv{Text: s} }

// kvRow builds one keyValueList column: top label (bold title), content
// (main line), bottom label (sub line).
func kvRow(top, content, bottom string) kvColumn {
	c := kvColumn{TopLabel: kvText(top), Content: kvText(content)}
	if bottom != "" {
		c.BottomLabel = kvText(bottom)
	}
	return c
}

type buttonList struct {
	Buttons []button `json:"buttons"`
}

type button struct {
	Text    string   `json:"text"`
	OnClick *onClick `json:"onClick,omitempty"`
}

type onClick struct {
	OpenLink *openLink `json:"openLink,omitempty"`
}

type openLink struct {
	URL string `json:"url"`
}

func wButton(label, url string) widget {
	return widget{ButtonList: &buttonList{Buttons: []button{{
		Text:    label,
		OnClick: &onClick{OpenLink: &openLink{URL: url}},
	}}}}
}

type mediaInfo struct {
	ContentURL string `json:"contentUrl"`
}

func wDivider() widget { return widget{Divider: &struct{}{}} }

// cardColor is the accent color for section header bars.
const cardColor = "#1a73e8"

// warningColor marks anomaly text (Google Chat palette red).
const warningColor = "#d93025"
