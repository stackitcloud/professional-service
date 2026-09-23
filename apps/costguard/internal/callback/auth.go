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

package callback

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/notifier"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// protectRequest is the parsed protect request, common to the GET (query
// params) and POST (JSON body) variants.
type protectRequest struct {
	ID      string
	Type    whitelist.ResourceType
	Project string
	Region  string
	Exp     int64
	Sig     string
}

// parseRequest extracts and validates the protect request parameters.
// It returns a non-zero status and a plain-text message on any problem.
// The raw query string / body is never echoed back or logged.
func (s *Server) parseRequest(method string, r *http.Request) (protectRequest, int, string) {
	var raw map[string]any
	var err error
	if method == http.MethodPost {
		r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
		dec := json.NewDecoder(r.Body)
		if err = dec.Decode(&raw); err != nil {
			return protectRequest{}, http.StatusBadRequest, "invalid JSON body"
		}
	} else {
		q := r.URL.Query()
		raw = make(map[string]any, len(q))
		for k, v := range q {
			if len(v) > 0 {
				raw[k] = v[0]
			}
		}
	}

	req := protectRequest{}
	req.ID = strOf(raw, notifier.ParamID)
	typeStr := strOf(raw, notifier.ParamType)
	req.Project = strOf(raw, notifier.ParamProject)
	req.Region = strOf(raw, notifier.ParamRegion)
	req.Sig = strOf(raw, notifier.ParamSig)

	if req.ID == "" {
		return req, http.StatusBadRequest, "missing parameter: " + notifier.ParamID
	}
	if typeStr == "" {
		return req, http.StatusBadRequest, "missing parameter: " + notifier.ParamType
	}
	typ, ok := whitelist.ParseResourceType(typeStr)
	if !ok {
		return req, http.StatusBadRequest, "unknown resource type: " + typeStr
	}
	req.Type = typ
	if (typ == whitelist.TypeVolume || typ == whitelist.TypePublicIP) && (req.Project == "" || req.Region == "") {
		return req, http.StatusBadRequest, "missing parameter: project and region are required for " + string(typ)
	}
	if expStr := strOf(raw, notifier.ParamExp); expStr == "" {
		return req, http.StatusBadRequest, "missing parameter: " + notifier.ParamExp
	} else if exp, perr := strconv.ParseInt(expStr, 10, 64); perr != nil {
		return req, http.StatusBadRequest, "invalid parameter: " + notifier.ParamExp
	} else {
		req.Exp = exp
	}
	if req.Sig == "" {
		return req, http.StatusBadRequest, "missing parameter: " + notifier.ParamSig
	}
	return req, 0, ""
}

// strOf reads a string field from a loosely-typed parameter map (JSON
// numbers arrive as float64; the callback only needs string values).
func strOf(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return ""
	default:
		return ""
	}
}
