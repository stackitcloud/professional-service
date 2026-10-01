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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func sku(product, name, region, monthly string, attrs map[string]any) map[string]any {
	if attrs == nil {
		attrs = map[string]any{}
	}
	return map[string]any{
		"name": name, "productName": product, "region": region,
		"prices":                    []map[string]any{{"currencyCode": "EUR", "monthlyPrice": monthly}},
		"productSpecificAttributes": attrs,
	}
}

type fakePIM struct {
	mu       sync.Mutex
	pages    map[string][][]map[string]any
	queries  []string
	fail     map[string][]int
	authSeen bool
}

func (f *fakePIM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "" {
		f.authSeen = true
	}
	q := r.URL.Query()
	product := q.Get("productName")
	f.queries = append(f.queries, r.URL.RawQuery)
	if codes := f.fail[product]; len(codes) > 0 {
		f.fail[product] = codes[1:]
		w.WriteHeader(codes[0])
		return
	}
	pages := f.pages[product]
	i := 0
	if c := q.Get("cursor"); c != "" {
		i = int(c[0] - '0')
	}
	page := map[string]any{"meta": map[string]any{"pageSize": 100}, "data": []any{}}
	if i < len(pages) {
		page["data"] = pages[i]
		page["meta"].(map[string]any)["nextCursor"] = string(rune('0' + i + 1))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func realisticPIM() *fakePIM {
	return &fakePIM{fail: map[string][]int{}, pages: map[string][][]map[string]any{
		"Server": {
			{
				sku("Server", "Tiny Server-t2i.1-EU01", "eu01", "10.1755742328", map[string]any{"flavor": "t2i.1", "metro": false}),
				sku("Server", "Tiny Server-t2i.1-EU01-m", "eu01", "12.6673227288", map[string]any{"flavor": "t2i.1", "metro": true}),
			},
			{sku("Server", "General Purpose Server-g2i.8-EU02", "eu02", "309.33", map[string]any{"flavor": "g2i.8", "metro": false})},
		},
		"GPU Server": {{sku("GPU Server", "GPU Server-n1.28d.g2-EU01", "eu01", "4000", map[string]any{"flavor": "n1.28d.g2"})}},
		"Block Storage": {{
			sku("Block Storage", "Block Storage for disk volumes Premium-Capacity-EU01", "eu01", "0.065349936", map[string]any{"storage": "volume", "type": "capacity", "metro": false}),
			sku("Block Storage", "Block Storage for disk volumes Premium-Capacity-EU01-m", "eu01", "0.096198912", map[string]any{"storage": "volume", "type": "capacity", "metro": true}),
			sku("Block Storage", "Block Storage for disk volumes Premium-Performance 1-EU01", "eu01", "7.3200000024", map[string]any{"storage": "volume", "type": "performance", "class": "storage_premium_perf1", "metro": false}),
			sku("Block Storage", "Block Storage for snapshots Premium-EU01", "eu01", "0.0168645024", map[string]any{"storage": "snapshot", "type": "capacity", "metro": false}),
			sku("Block Storage", "Block Storage for backups Premium-EU01", "eu01", "0.03", map[string]any{"storage": "backup", "type": "capacity"}),
			sku("Block Storage", "Block Storage for MongoDB Premium-Performance 2-EU01", "eu01", "14.5", map[string]any{"storage": "mongodb", "type": "performance", "class": "storage_premium_perf2_mongodb"}),
		}},
		"Public IP Address": {{
			sku("Public IP Address", "Router-IP-EU01", "eu01", "99", nil),
			sku("Public IP Address", "Public IP Address (IPv4)-EU01", "eu01", "2.9200000032", nil),
			{"name": "Public IP Address (IPv4)-EU02", "productName": "Public IP Address", "region": "eu02",
				"prices": []map[string]any{{"currencyCode": "USD", "monthlyPrice": "3"}}},
			sku("Public IP Address", "Public IP Address (IPv4)-EU03", "eu03", "not a number", nil),
		}},
	}}
}

func testPIM(t *testing.T, f *fakePIM) *pim {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &pim{base: srv.URL, client: srv.Client(), pauses: []time.Duration{0, 0}}
}

func TestPIMReadsThePriceList(t *testing.T) {
	f := realisticPIM()
	p, err := testPIM(t, f).Prices(context.Background(), []string{"eu01", "eu02"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"server t2i.1":       p.Server[Offer{"eu01", "t2i.1", false}],
		"server t2i.1 metro": p.Server[Offer{"eu01", "t2i.1", true}],
		"server g2i.8 eu02":  p.Server[Offer{"eu02", "g2i.8", false}],
		"gpu":                p.Server[Offer{"eu01", "n1.28d.g2", false}],
		"volume GB":          p.VolumeGB[Offer{"eu01", "", false}],
		"volume GB metro":    p.VolumeGB[Offer{"eu01", "", true}],
		"perf1":              p.VolumeClass[Offer{"eu01", "storage_premium_perf1", false}],
		"snapshot GB":        p.SnapshotGB[Offer{"eu01", "", false}],
		"public IP":          p.PublicIP[Offer{"eu01", "", false}],
	}
	for name, v := range map[string]float64{
		"server t2i.1": 10.1755742328, "server t2i.1 metro": 12.6673227288, "server g2i.8 eu02": 309.33, "gpu": 4000,
		"volume GB": 0.065349936, "volume GB metro": 0.096198912, "perf1": 7.3200000024, "snapshot GB": 0.0168645024,
		"public IP": 2.9200000032,
	} {
		if want[name] != v {
			t.Errorf("%s = %v, want %v", name, want[name], v)
		}
	}
	if len(p.VolumeClass) != 1 || len(p.SnapshotGB) != 1 || len(p.PublicIP) != 1 || len(p.Server) != 4 {
		t.Errorf("only the SKUs costguard prices: %+v", p)
	}
	if f.authSeen {
		t.Error("the price list is public: no token may be sent to it")
	}
	var server []string
	for _, q := range f.queries {
		if strings.Contains(q, "productName=Server&") {
			server = append(server, q)
		}
		if !strings.Contains(q, "region=eu01&region=eu02") || !strings.Contains(q, "pageSize=100") || !strings.Contains(q, "language=en") {
			t.Errorf("query = %s", q)
		}
	}
	if len(server) != 3 || strings.Contains(server[0], "cursor") || !strings.Contains(server[1], "cursor=1") || !strings.Contains(server[2], "cursor=2") {
		t.Errorf("paging = %v", server)
	}
}

func TestPIMRetriesRateLimitsAndServerErrors(t *testing.T) {
	f := realisticPIM()
	f.fail["Block Storage"] = []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}
	p, err := testPIM(t, f).Prices(context.Background(), []string{"eu01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.VolumeGB) == 0 {
		t.Error("the third attempt should have answered")
	}
}

func TestPIMFailures(t *testing.T) {
	f := realisticPIM()
	f.fail["Server"] = []int{http.StatusNotFound}
	_, err := testPIM(t, f).Prices(context.Background(), []string{"eu01"})
	if err == nil || !strings.Contains(err.Error(), "reading the Server prices from STACKIT's price list: HTTP 404") {
		t.Errorf("err = %v", err)
	}

	f = realisticPIM()
	f.fail["Public IP Address"] = []int{500, 500, 500}
	if _, err := testPIM(t, f).Prices(context.Background(), []string{"eu01"}); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("gives up after three attempts: %v", err)
	}

	for name, body := range map[string]string{
		"not JSON":  "<html>",
		"too large": strings.Repeat(" ", maxPIMBody+1),
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		c := &pim{base: srv.URL, client: srv.Client()}
		if _, err := c.Prices(context.Background(), []string{"eu01"}); err == nil {
			t.Errorf("%s: want an error", name)
		}
		srv.Close()
	}

	endless := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"nextCursor":"x"},"data":[{"name":"n"}]}`))
	}))
	defer endless.Close()
	if _, err := (&pim{base: endless.URL, client: endless.Client()}).Prices(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "more than 50 pages") {
		t.Errorf("paging must stop: %v", err)
	}

	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	if _, err := (&pim{base: gone.URL, client: gone.Client(), pauses: []time.Duration{0}}).Prices(context.Background(), nil); err == nil {
		t.Error("an unreachable price list is an error")
	}

	if _, err := (&pim{base: "http://in valid", client: http.DefaultClient}).Prices(context.Background(), nil); err == nil {
		t.Error("a bad base URL is an error")
	}
}

func TestPIMStopsWaitingWhenTheRunEnds(t *testing.T) {
	f := realisticPIM()
	f.fail["Server"] = []int{503}
	c := testPIM(t, f)
	c.pauses = []time.Duration{time.Hour}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Prices(ctx, []string{"eu01"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
}

func TestNewPIM(t *testing.T) {
	c := newPIM()
	if c.base != "https://pim.api.stackit.cloud" || c.client.Timeout != pimTimeout || c.client.Transport != nil || len(c.pauses) != 2 {
		t.Errorf("newPIM = %+v", c)
	}
}

func TestMonthly(t *testing.T) {
	p := NewPrices()
	p.Server[Offer{"eu01", "g2i.4", false}] = 147.3
	p.Server[Offer{"eu01", "g2i.4", true}] = 290
	p.VolumeGB[Offer{"eu01", "", false}] = 0.065
	p.VolumeGB[Offer{"eu01", "", true}] = 0.1
	p.VolumeClass[Offer{"eu01", "storage_premium_perf1", false}] = 7.32
	p.SnapshotGB[Offer{"eu01", "", false}] = 0.02
	p.SnapshotGB[Offer{"eu01", "", true}] = 0.03
	p.PublicIP[Offer{"eu01", "", false}] = 2.92

	for _, tc := range []struct {
		name string
		r    Resource
		zone string
		want float64
		ok   bool
	}{
		{"server", Resource{Kind: KindServer, Region: "eu01", MachineType: "g2i.4", AvailabilityZone: "eu01-1"}, "", 147.3, true},
		{"metro server", Resource{Kind: KindServer, Region: "eu01", MachineType: "g2i.4", AvailabilityZone: "eu01-m"}, "", 290, true},
		{"deallocated server", Resource{Kind: KindServer, Region: "eu01", MachineType: "g2i.4", Status: ServerStatusDeallocated}, "", 0, true},
		{"unknown flavor", Resource{Kind: KindServer, Region: "eu01", MachineType: "x9.9"}, "", 0, false},
		{"volume", Resource{Kind: KindVolume, Region: "eu01", SizeGB: 100, PerformanceClass: "storage_premium_perf1", AvailabilityZone: "eu01-2"}, "", 13.82, true},
		{"volume without class", Resource{Kind: KindVolume, Region: "eu01", SizeGB: 10}, "", 0.65, true},
		{"unknown class", Resource{Kind: KindVolume, Region: "eu01", SizeGB: 10, PerformanceClass: "storage_premium_perf99"}, "", 0, false},
		{"volume in an unknown region", Resource{Kind: KindVolume, Region: "eu09", SizeGB: 10}, "", 0, false},
		{"snapshot", Resource{Kind: KindSnapshot, Region: "eu01", SizeGB: 50}, "", 1, true},
		{"snapshot of a metro volume", Resource{Kind: KindSnapshot, Region: "eu01", SizeGB: 50}, "eu01-m", 1.5, true},
		{"public IP", Resource{Kind: KindPublicIP, Region: "eu01"}, "", 2.92, true},
		{"public IP elsewhere", Resource{Kind: KindPublicIP, Region: "eu02"}, "", 0, false},
		{"NIC", Resource{Kind: KindNIC, Region: "eu01"}, "", 0, true},
		{"security group", Resource{Kind: KindSecurityGroup, Region: "eu01"}, "", 0, true},
		{"unknown kind", Resource{Kind: "image", Region: "eu01"}, "", 0, false},
	} {
		got, ok := p.Monthly(tc.r, tc.zone)
		if ok != tc.ok || (got-tc.want) > 1e-9 || (tc.want-got) > 1e-9 {
			t.Errorf("%s: %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
	var none *Prices
	if _, ok := none.Monthly(Resource{Kind: KindNIC}, ""); ok {
		t.Error("without a price list nothing is priced")
	}
}
