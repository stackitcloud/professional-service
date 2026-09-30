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

const (
	LabelDelete = "delete"
	LabelDoNotDelete = "do-not-delete"
	LabelTrue = "true"
)

func Requested(labels map[string]string) bool {
	return strings.EqualFold(labels[LabelDelete], LabelTrue)
}

func Protected(labels map[string]string) bool {
	v, ok := labels[LabelDoNotDelete]
	return ok && !strings.EqualFold(v, "false")
}

func StatusCode(err error) (int, bool) {
	var apiErr *oapierror.GenericOpenAPIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.StatusCode, true
	}
	return 0, false
}

func IsNotFound(err error) bool {
	code, ok := StatusCode(err)
	return ok && code == 404
}

const maxDescribed = 200

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

var notEnabledPhrases = []string{"not enabled", "not activated", "not been enabled", "not been activated"}

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
