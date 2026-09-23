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

package notifier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

// protectPath is the callback endpoint a button points at.
const protectPath = "/protect"

// Button params, shared verbatim with the callback verifier (internal/
// callback/auth.go). Keeping the names here as constants is the single
// source of truth so a drift can't silently break button validation.
const (
	ParamID      = "id"
	ParamType    = "type"
	ParamProject = "project"
	ParamRegion  = "region"
	ParamExp     = "exp"
	ParamSig     = "sig"
)

// ProtectRequest identifies the resource a protect button targets.
// Project and Region are required for IaaS types (volume, publicip) —
// the IaaS APIs address resources per project/region — and empty for
// project/folder types. All fields ride inside the HMAC, so a click for
// a different resource than the one warned about is impossible.
type ProtectRequest struct {
	ID       string
	Type     whitelist.ResourceType
	Project  string
	Region   string
	Deadline time.Time
}

// Button builds signed "Do not delete" URLs. A nil
// Button renders no buttons — the caller treats that as "static text
// only", which is exactly what happens when COSTGUARD_CALLBACK_URL is
// unset.
type Button struct {
	// BaseURL is the public origin of the callback server, e.g.
	// "https://costguard-callback.example.com". No trailing slash.
	BaseURL string
	// Secret is the HMAC key (COSTGUARD_CALLBACK_SECRET).
	Secret []byte
}

// NewButton returns a Button, or nil when the callback URL or secret is
// missing (buttons disabled).
func NewButton(baseURL, secret string) *Button {
	if strings.TrimSpace(baseURL) == "" || secret == "" {
		return nil
	}
	return &Button{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Secret:  []byte(secret),
	}
}

// Enabled reports whether buttons should be rendered.
func (b *Button) Enabled() bool { return b != nil }

// Endpoint returns the callback /protect URL (no query string). Teams
// Action.Http buttons POST to this endpoint.
func (b *Button) Endpoint() string {
	return b.BaseURL + protectPath
}

// Sign computes the button signature: HMAC-SHA256 over
// "id|type|exp|project|region". Exported so the callback server (a
// different package) verifies with the identical primitive — there is
// exactly one signing scheme.
func Sign(secret []byte, req ProtectRequest, exp int64) string {
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintf(mac, "%s|%s|%d|%s|%s", req.ID, string(req.Type), exp, req.Project, req.Region)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySig reports whether sig is the valid signature for the given
// parameters. It uses hmac.Equal for a constant-time comparison.
func VerifySig(secret []byte, id, typ, project, region string, exp int64, sig string) bool {
	req := ProtectRequest{ID: id, Type: whitelist.ResourceType(typ), Project: project, Region: region}
	want := Sign(secret, req, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

// URL builds the signed GET /protect link for one resource. It expires at
// the request deadline (the candidate's deletion deadline): a
// click after that is rejected with 410 Gone by the callback.
func (b *Button) URL(req ProtectRequest) string {
	exp := req.Deadline.Unix()
	q := url.Values{}
	q.Set(ParamID, req.ID)
	q.Set(ParamType, string(req.Type))
	q.Set(ParamProject, req.Project)
	q.Set(ParamRegion, req.Region)
	q.Set(ParamExp, strconv.FormatInt(exp, 10))
	q.Set(ParamSig, Sign(b.Secret, req, exp))
	return b.BaseURL + protectPath + "?" + q.Encode()
}

// Body builds the JSON body the Teams Action.Http button POSTs to
// /protect. It carries the same fields the GET link sends as query
// params.
func (b *Button) Body(req ProtectRequest) (string, error) {
	exp := req.Deadline.Unix()
	payload, err := json.Marshal(map[string]any{
		ParamID:      req.ID,
		ParamType:    string(req.Type),
		ParamProject: req.Project,
		ParamRegion:  req.Region,
		ParamExp:     exp,
		ParamSig:     Sign(b.Secret, req, exp),
	})
	if err != nil {
		return "", fmt.Errorf("encoding button body: %w", err)
	}
	return string(payload), nil
}
