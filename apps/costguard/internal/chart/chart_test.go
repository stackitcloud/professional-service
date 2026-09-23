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

package chart

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

func sampleDaily(n int) []report.DailyCost {
	daily := make([]report.DailyCost, 0, n)
	start := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		daily = append(daily, report.DailyCost{
			Date:    start.AddDate(0, 0, i).Format("2006-01-02"),
			CostEUR: 10 + float64(i)*0.5,
		})
	}
	return daily
}

func TestRenderProducesPNG(t *testing.T) {
	buf, err := Render(sampleDaily(30))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// PNG magic bytes.
	if !bytes.HasPrefix(buf.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
	if buf.Len() < 1000 {
		t.Errorf("suspiciously small PNG: %d bytes", buf.Len())
	}
}

func TestRenderEmpty(t *testing.T) {
	if _, err := Render(nil); err == nil {
		t.Error("Render(nil) = nil error, want error")
	}
}

func TestRenderUnparseableDates(t *testing.T) {
	daily := []report.DailyCost{
		{Date: "not-a-date", CostEUR: 1},
		{Date: "also-bad", CostEUR: 2},
	}
	if _, err := Render(daily); err == nil {
		t.Error("Render with no parseable dates = nil error, want error")
	}
}

func TestNewUploader(t *testing.T) {
	s := config.S3Config{Endpoint: "https://s3.eu01.stackit.cloud", Region: "eu01", AccessKey: "ak", SecretKey: "sk", Bucket: "charts"}
	u, err := NewUploader(context.Background(), s)
	if err != nil {
		t.Fatalf("NewUploader: %v", err)
	}
	if u == nil || u.bucket != "charts" {
		t.Errorf("uploader = %+v", u)
	}
}

// TestUploadAndPresignRoundTrip runs the real S3 client against an
// httptest endpoint: the PUT must arrive with the right content type, and
// the presigned URL must be valid for 7 days (604800 s).
func TestUploadAndPresignRoundTrip(t *testing.T) {
	var putCount int
	var putKey, putContentType string
	var putBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.Method {
		case http.MethodPut:
			putCount++
			putKey = r.URL.Path
			putContentType = r.Header.Get("Content-Type")
			putBody = body
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	s := config.S3Config{Endpoint: srv.URL, Region: "eu01", AccessKey: "ak", SecretKey: "sk", Bucket: "charts"}
	u, err := NewUploader(context.Background(), s)
	if err != nil {
		t.Fatalf("NewUploader: %v", err)
	}
	png, err := Render(sampleDaily(30))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	key := "costguard/2026-09-22/chart.png"
	url, err := u.UploadAndPresign(context.Background(), key, png)
	if err != nil {
		t.Fatalf("UploadAndPresign: %v", err)
	}
	if putCount != 1 {
		t.Errorf("PUT count = %d, want 1", putCount)
	}
	// Path-style addressing: /<bucket>/<key>.
	if putKey != "/charts/"+key {
		t.Errorf("PUT path = %q, want /charts/%s", putKey, key)
	}
	if putContentType != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", putContentType)
	}
	if !bytes.Equal(putBody, png.Bytes()) {
		t.Error("uploaded body does not match the rendered PNG")
	}
	if !strings.HasPrefix(url, srv.URL) {
		t.Errorf("presigned URL = %q, want it to target the configured endpoint", url)
	}
	if !strings.Contains(url, key) {
		t.Errorf("presigned URL = %q, want the object key", url)
	}
	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Errorf("presigned URL = %q, want a signature", url)
	}
	if !strings.Contains(url, "X-Amz-Expires=604800") {
		t.Errorf("presigned URL = %q, want a 7-day expiry (604800s)", url)
	}
}

func TestUploadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	s := config.S3Config{Endpoint: srv.URL, Region: "eu01", AccessKey: "ak", SecretKey: "sk", Bucket: "charts"}
	u, err := NewUploader(context.Background(), s)
	if err != nil {
		t.Fatalf("NewUploader: %v", err)
	}
	if _, err := u.UploadAndPresign(context.Background(), "k.png", bytes.NewBufferString("x")); err == nil {
		t.Error("UploadAndPresign = nil error, want upload failure")
	}
}
