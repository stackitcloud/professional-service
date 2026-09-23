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
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

func TestNewButtonDisabledWithoutURL(t *testing.T) {
	if b := NewButton("", "secret"); b != nil {
		t.Errorf("NewButton with empty URL = %+v, want nil", b)
	}
	if b := NewButton("https://cb.example", ""); b != nil {
		t.Errorf("NewButton with empty secret = %+v, want nil", b)
	}
}

func TestNewButtonTrimsTrailingSlash(t *testing.T) {
	b := NewButton("https://cb.example.com/", "secret")
	if b == nil {
		t.Fatal("NewButton = nil")
	}
	if b.BaseURL != "https://cb.example.com" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", b.BaseURL)
	}
}

func TestSignIsDeterministicAndInputSensitive(t *testing.T) {
	secret := []byte("s3cr3t")
	base := ProtectRequest{ID: "vol-1", Type: whitelist.TypeVolume, Project: "pp", Region: "eu01"}
	a := Sign(secret, base, 1000)
	b := Sign(secret, base, 1000)
	if a != b {
		t.Errorf("Sign not deterministic: %q vs %q", a, b)
	}
	if Sign(secret, ProtectRequest{ID: "vol-2", Type: whitelist.TypeVolume, Project: "pp", Region: "eu01"}, 1000) == a {
		t.Error("changing id did not change the signature")
	}
	if Sign(secret, ProtectRequest{ID: "vol-1", Type: whitelist.TypePublicIP, Project: "pp", Region: "eu01"}, 1000) == a {
		t.Error("changing type did not change the signature")
	}
	if Sign(secret, base, 1001) == a {
		t.Error("changing exp did not change the signature")
	}
	if Sign(secret, ProtectRequest{ID: "vol-1", Type: whitelist.TypeVolume, Project: "other", Region: "eu01"}, 1000) == a {
		t.Error("changing project did not change the signature")
	}
	if Sign(secret, ProtectRequest{ID: "vol-1", Type: whitelist.TypeVolume, Project: "pp", Region: "us01"}, 1000) == a {
		t.Error("changing region did not change the signature")
	}
	if Sign([]byte("other"), base, 1000) == a {
		t.Error("changing secret did not change the signature")
	}
}

func TestVerifySig(t *testing.T) {
	secret := []byte("s3cr3t")
	sig := Sign(secret, ProtectRequest{ID: "vol-1", Type: whitelist.TypeVolume, Project: "pp", Region: "eu01"}, 1234)
	if !VerifySig(secret, "vol-1", "volume", "pp", "eu01", 1234, sig) {
		t.Error("VerifySig rejected a valid signature")
	}
	if VerifySig(secret, "vol-1", "volume", "pp", "eu01", 1235, sig) {
		t.Error("VerifySig accepted a stale exp")
	}
	if VerifySig(secret, "vol-2", "volume", "pp", "eu01", 1234, sig) {
		t.Error("VerifySig accepted a tampered id")
	}
	if VerifySig(secret, "vol-1", "volume", "other", "eu01", 1234, sig) {
		t.Error("VerifySig accepted a tampered project")
	}
	if VerifySig([]byte("nope"), "vol-1", "volume", "pp", "eu01", 1234, sig) {
		t.Error("VerifySig accepted the wrong secret")
	}
	if VerifySig(secret, "vol-1", "volume", "pp", "eu01", 1234, "deadbeef") {
		t.Error("VerifySig accepted a garbage signature")
	}
}

func TestButtonURLFormat(t *testing.T) {
	b := NewButton("https://cb.example.com", "s3cr3t")
	deadline := time.Unix(1800000000, 0).UTC()
	u := b.URL(ProtectRequest{ID: "vol-1", Type: whitelist.TypeVolume, Project: "pp-1", Region: "eu01", Deadline: deadline})
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("URL parse: %v", err)
	}
	if parsed.Host != "cb.example.com" || parsed.Path != "/protect" {
		t.Errorf("host/path = %s %s, want cb.example.com /protect", parsed.Host, parsed.Path)
	}
	q := parsed.Query()
	if q.Get(ParamID) != "vol-1" {
		t.Errorf("id = %q", q.Get(ParamID))
	}
	if q.Get(ParamType) != "volume" {
		t.Errorf("type = %q", q.Get(ParamType))
	}
	if q.Get(ParamProject) != "pp-1" || q.Get(ParamRegion) != "eu01" {
		t.Errorf("project/region = %q/%q", q.Get(ParamProject), q.Get(ParamRegion))
	}
	if q.Get(ParamExp) != "1800000000" {
		t.Errorf("exp = %q", q.Get(ParamExp))
	}
	if !VerifySig(b.Secret, "vol-1", "volume", "pp-1", "eu01", 1800000000, q.Get(ParamSig)) {
		t.Error("URL signature did not verify")
	}
}

func TestButtonBody(t *testing.T) {
	b := NewButton("https://cb.example.com", "s3cr3t")
	deadline := time.Unix(1800000000, 0).UTC()
	body, err := b.Body(ProtectRequest{ID: "ip-1", Type: whitelist.TypePublicIP, Project: "pp-1", Region: "eu01", Deadline: deadline})
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("Body is not JSON: %v (%s)", err, body)
	}
	if m[ParamID] != "ip-1" || m[ParamType] != "publicip" {
		t.Errorf("body = %v", m)
	}
	if m[ParamProject] != "pp-1" || m[ParamRegion] != "eu01" {
		t.Errorf("body project/region = %v/%v", m[ParamProject], m[ParamRegion])
	}
	if m[ParamExp].(float64) != 1800000000 {
		t.Errorf("body exp = %v", m[ParamExp])
	}
	if !VerifySig(b.Secret, "ip-1", "publicip", "pp-1", "eu01", 1800000000, m[ParamSig].(string)) {
		t.Error("body signature did not verify")
	}
}

func TestButtonEndpoint(t *testing.T) {
	b := NewButton("https://cb.example.com/", "s3cr3t")
	if b.Endpoint() != "https://cb.example.com/protect" {
		t.Errorf("Endpoint = %q", b.Endpoint())
	}
}

func TestButtonDeadlineCandidatesUseMark(t *testing.T) {
	gen := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	mark := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	if got := ButtonDeadline(gen, &mark); !got.Equal(mark) {
		t.Errorf("ButtonDeadline = %v, want the mark deadline", got)
	}
}

func TestButtonDeadlineReportOnlyUsesCycle(t *testing.T) {
	gen := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	want := gen.Add(7 * 24 * time.Hour)
	if got := ButtonDeadline(gen, nil); !got.Equal(want) {
		t.Errorf("ButtonDeadline = %v, want %v (one cycle)", got, want)
	}
}

func TestFormatEUR(t *testing.T) {
	cases := map[float64]string{
		0:      "€0.00",
		6.19:   "€6.19",
		18.935: "€18.93", // binary value is 18.934999…; display uses %.2f
		1234.5: "€1234.50",
		-3.5:   "€-3.50",
	}
	for in, want := range cases {
		if got := FormatEUR(in); got != want {
			t.Errorf("FormatEUR(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDeadline(t *testing.T) {
	if got := FormatDeadline(time.Time{}); got != "n/a" {
		t.Errorf("FormatDeadline(zero) = %q, want n/a", got)
	}
	in := time.Date(2026, 9, 22, 20, 0, 0, 0, time.UTC)
	if got := FormatDeadline(in); got != "2026-09-22 20:00 UTC" {
		t.Errorf("FormatDeadline = %q", got)
	}
}

func TestGroupProjectsByFolder(t *testing.T) {
	in := []report.StaleProject{
		{ID: "p1", Name: "a", ParentFolderID: "f1", ParentFolderName: "Team A"},
		{ID: "p2", Name: "b", ParentFolderID: "f2", ParentFolderName: "Team B"},
		{ID: "p3", Name: "c", ParentFolderID: "f1", ParentFolderName: "Team A"},
		{ID: "p4", Name: "d", ParentFolderID: "", ParentFolderName: ""},
	}
	got := GroupProjectsByFolder(in)
	if len(got) != 3 {
		t.Fatalf("groups = %d, want 3", len(got))
	}
	if got[0].FolderName != "Team A" || len(got[0].Projects) != 2 {
		t.Errorf("group 0 = %+v", got[0])
	}
	if got[0].Projects[0].ID != "p1" || got[0].Projects[1].ID != "p3" {
		t.Errorf("group 0 order = %+v", got[0].Projects)
	}
	if got[1].FolderName != "Team B" || len(got[1].Projects) != 1 {
		t.Errorf("group 1 = %+v", got[1])
	}
	if got[2].FolderName != "" || len(got[2].Projects) != 1 {
		t.Errorf("group 2 (root) = %+v", got[2])
	}
}

func TestGroupProjectsByFolderEmpty(t *testing.T) {
	if got := GroupProjectsByFolder(nil); len(got) != 0 {
		t.Errorf("groups = %+v, want none", got)
	}
}
