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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"
)

// The two labels users steer costguard with.
const (
	// LabelDelete=true asks for deletion in the next delete run. costguard
	// sets it on cleanup candidates; users may set it on any IaaS resource.
	LabelDelete = "delete"
	// LabelDoNotDelete=true protects a resource. On a project or folder it
	// skips the whole subtree.
	LabelDoNotDelete = "do-not-delete"
	// LabelTrue is the value costguard writes.
	LabelTrue = "true"
)

// Requested reports whether the resource carries delete=true.
func Requested(labels map[string]string) bool {
	return strings.EqualFold(labels[LabelDelete], LabelTrue)
}

// Protected reports whether the resource carries do-not-delete. Any value
// other than "false" protects: when in doubt, keep the resource.
func Protected(labels map[string]string) bool {
	v, ok := labels[LabelDoNotDelete]
	return ok && !strings.EqualFold(v, "false")
}

// StatusCode extracts the HTTP status from an SDK error.
func StatusCode(err error) (int, bool) {
	var apiErr *oapierror.GenericOpenAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode, true
	}
	return 0, false
}

// IsNotFound reports whether err is an HTTP 404.
func IsNotFound(err error) bool {
	code, ok := StatusCode(err)
	return ok && code == 404
}

// maxDescribed caps the API message quoted by Describe.
const maxDescribed = 200

// Describe renders an error for a chat message. API errors become
// "HTTP 403: <the message the API sent>" instead of the SDK's text with the
// raw response body; other errors keep their text.
func Describe(err error) string {
	var apiErr *oapierror.GenericOpenAPIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return err.Error()
	}
	out := fmt.Sprintf("HTTP %d", apiErr.StatusCode)
	var body struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(apiErr.Body, &body) == nil && strings.TrimSpace(body.Message) != "" {
		msg := []rune(strings.TrimSpace(body.Message))
		if len(msg) > maxDescribed {
			msg = append(msg[:maxDescribed], '…')
		}
		out += ": " + string(msg)
	}
	return out
}

// notEnabledPhrases are the wordings that say a service is not enabled for
// a project. ❓ Not verified against the real APIs yet (README_internal.md,
// check L): replace them with the exact answers once they are known.
var notEnabledPhrases = []string{"not enabled", "not activated", "not been enabled", "not been activated"}

// NotEnabled reports whether err means that a service is not enabled for
// the project or region: an HTTP 404, or a 4xx answer that says so. Such a
// project has nothing of that service, so callers treat it as empty. Any
// other 403 stays an error on purpose: it can also mean that costguard may
// not look, and "no load balancer" must never be concluded from that.
func NotEnabled(err error) bool {
	var apiErr *oapierror.GenericOpenAPIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	switch {
	case apiErr.StatusCode == 404:
		return true
	case apiErr.StatusCode < 400 || apiErr.StatusCode > 499 || apiErr.StatusCode == 401:
		return false
	}
	text := strings.ToLower(apiErr.ErrorMessage + " " + string(apiErr.Body))
	for _, phrase := range notEnabledPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func copyLabels(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// stringLabels converts the IaaS label type to map[string]string,
// dropping non-string values (which well-formed labels never have).
func stringLabels(in map[string]interface{}) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
