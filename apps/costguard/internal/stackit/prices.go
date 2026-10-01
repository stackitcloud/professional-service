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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PriceList interface {
	Prices(ctx context.Context, regions []string) (*Prices, error)
}

type Offer struct {
	Region string
	Name   string
	Metro  bool
}

type Prices struct {
	Server      map[Offer]float64
	VolumeGB    map[Offer]float64
	VolumeClass map[Offer]float64
	SnapshotGB  map[Offer]float64
	PublicIP    map[Offer]float64
}

func NewPrices() *Prices {
	return &Prices{
		Server:      map[Offer]float64{},
		VolumeGB:    map[Offer]float64{},
		VolumeClass: map[Offer]float64{},
		SnapshotGB:  map[Offer]float64{},
		PublicIP:    map[Offer]float64{},
	}
}

func Metro(zone string) bool {
	return strings.HasSuffix(zone, "-m")
}

func (p *Prices) Monthly(r Resource, zone string) (float64, bool) {
	if p == nil {
		return 0, false
	}
	if zone == "" {
		zone = r.AvailabilityZone
	}
	metro := Metro(zone)
	switch r.Kind {
	case KindServer:
		if r.Status == ServerStatusDeallocated {
			return 0, true
		}
		v, ok := p.Server[Offer{r.Region, r.MachineType, metro}]
		return v, ok
	case KindVolume:
		gb, ok := p.VolumeGB[Offer{r.Region, "", metro}]
		if !ok {
			return 0, false
		}
		total := gb * float64(r.SizeGB)
		if r.PerformanceClass != "" {
			fee, ok := p.VolumeClass[Offer{r.Region, r.PerformanceClass, metro}]
			if !ok {
				return 0, false
			}
			total += fee
		}
		return total, true
	case KindSnapshot:
		gb, ok := p.SnapshotGB[Offer{r.Region, "", metro}]
		return gb * float64(r.SizeGB), ok
	case KindPublicIP:
		v, ok := p.PublicIP[Offer{r.Region, "", false}]
		return v, ok
	case KindNIC, KindSecurityGroup:
		return 0, true
	}
	return 0, false
}

const (
	pimBase        = "https://pim.api.stackit.cloud"
	pimTimeout     = 30 * time.Second
	pimPageSize    = 100
	pimMaxPages    = 50
	maxPIMBody     = 8 << 20
	pimProductIPv4 = "Public IP Address"
)

var pimProducts = []string{"Server", "GPU Server", "Confidential Server", "Block Storage", pimProductIPv4}

var pimPauses = []time.Duration{2 * time.Second, 5 * time.Second}

type pim struct {
	base   string
	client *http.Client
	pauses []time.Duration
}

func newPIM() *pim {
	return &pim{
		base:   pimBase,
		client: &http.Client{Timeout: pimTimeout},
		pauses: pimPauses,
	}
}

type pimPage struct {
	Meta struct {
		NextCursor string `json:"nextCursor"`
	} `json:"meta"`
	Data []pimSKU `json:"data"`
}

type pimSKU struct {
	Name        string `json:"name"`
	ProductName string `json:"productName"`
	Region      string `json:"region"`
	Prices      []struct {
		CurrencyCode string `json:"currencyCode"`
		MonthlyPrice string `json:"monthlyPrice"`
	} `json:"prices"`
	Attributes struct {
		Flavor  string `json:"flavor"`
		Metro   bool   `json:"metro"`
		Class   string `json:"class"`
		Storage string `json:"storage"`
		Type    string `json:"type"`
	} `json:"productSpecificAttributes"`
}

func (c *pim) Prices(ctx context.Context, regions []string) (*Prices, error) {
	results := make([][]pimSKU, len(pimProducts))
	errs := make([]error, len(pimProducts))
	var wg sync.WaitGroup
	for i, product := range pimProducts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = c.list(ctx, product, regions)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	p := NewPrices()
	for _, skus := range results {
		for _, s := range skus {
			p.add(s)
		}
	}
	return p, nil
}

func (p *Prices) add(s pimSKU) {
	if len(s.Prices) == 0 || s.Prices[0].CurrencyCode != "EUR" {
		return
	}
	v, err := strconv.ParseFloat(s.Prices[0].MonthlyPrice, 64)
	if err != nil || v < 0 {
		return
	}
	a := s.Attributes
	switch {
	case s.ProductName == pimProductIPv4:
		if strings.HasPrefix(s.Name, "Public IP Address") {
			p.PublicIP[Offer{s.Region, "", false}] = v
		}
	case s.ProductName == "Block Storage" && a.Storage == "volume" && a.Type == "capacity":
		p.VolumeGB[Offer{s.Region, "", a.Metro}] = v
	case s.ProductName == "Block Storage" && a.Storage == "volume" && a.Type == "performance" && a.Class != "":
		p.VolumeClass[Offer{s.Region, a.Class, a.Metro}] = v
	case s.ProductName == "Block Storage" && a.Storage == "snapshot" && a.Type == "capacity":
		p.SnapshotGB[Offer{s.Region, "", a.Metro}] = v
	case a.Flavor != "" && s.ProductName != "Block Storage":
		p.Server[Offer{s.Region, a.Flavor, a.Metro}] = v
	}
}

func (c *pim) list(ctx context.Context, product string, regions []string) ([]pimSKU, error) {
	var out []pimSKU
	cursor := ""
	for range pimMaxPages {
		q := url.Values{}
		q.Set("productName", product)
		q.Set("language", "en")
		q.Set("pageSize", strconv.Itoa(pimPageSize))
		for _, r := range regions {
			q.Add("region", r)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var page pimPage
		if err := c.get(ctx, c.base+"/v2/skus?"+q.Encode(), &page); err != nil {
			return nil, fmt.Errorf("reading the %s prices from STACKIT's price list: %w", product, err)
		}
		out = append(out, page.Data...)
		if page.Meta.NextCursor == "" || len(page.Data) == 0 {
			return out, nil
		}
		cursor = page.Meta.NextCursor
	}
	return nil, fmt.Errorf("reading the %s prices from STACKIT's price list: more than %d pages", product, pimMaxPages)
}

func (c *pim) get(ctx context.Context, u string, out any) error {
	var last error
	for attempt := 0; ; attempt++ {
		retry, err := c.getOnce(ctx, u, out)
		if err == nil {
			return nil
		}
		last = err
		if !retry || attempt >= len(c.pauses) {
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.pauses[attempt]):
		}
	}
}

func (c *pim) getOnce(ctx context.Context, u string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPIMBody+1))
	if err != nil {
		return true, err
	}
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return retry, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if len(body) > maxPIMBody {
		return false, fmt.Errorf("the answer is larger than %d bytes", maxPIMBody)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, fmt.Errorf("unreadable answer: %w", err)
	}
	return false, nil
}
