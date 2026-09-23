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

package stackit

import (
	"fmt"
	"time"
)

// MarkLabelKey is the bot-applied mark label on deletion candidates
// (detached volumes, idle public IPs). It carries the deletion
// deadline as its value and is the only shared state between the scan and
// delete runs: state lives on the resource, so the stateless
// CronJobs stay crash-proof and the deadline is visible in the portal.
const MarkLabelKey = "costguard-delete-after"

const markLabelKey = MarkLabelKey

// markValueLayout is compact UTC without colons, e.g. 20260922T091503Z.
// Colons are invalid in label values on both the resource manager and the
// IaaS, so raw RFC3339 is rejected by the APIs.
// The compact form is portal-readable and lexicographically sortable.
const markValueLayout = "20060102T150405Z0700"

// FormatMark renders a deadline as a mark label value (UTC).
func FormatMark(deadline time.Time) string {
	return deadline.UTC().Format(markValueLayout)
}

// ParseMark parses a mark label value back into a UTC deadline.
func ParseMark(value string) (time.Time, error) {
	t, err := time.Parse(markValueLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing mark label value %q: %w", value, err)
	}
	return t, nil
}

// MarkDeadline extracts the deletion deadline from a resource's labels.
// It returns ok=false when the mark is absent or malformed — callers must
// treat that as "not deletable" (fail-closed).
func MarkDeadline(labels map[string]string) (time.Time, bool) {
	value, ok := labels[MarkLabelKey]
	if !ok || value == "" {
		return time.Time{}, false
	}
	t, err := ParseMark(value)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
