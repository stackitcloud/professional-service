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
	"fmt"
	"net/http"
	"time"

	"github.com/stackitcloud/stackit-sdk-go/core/runtime"
	costv3 "github.com/stackitcloud/stackit-sdk-go/services/cost/v3api"
)

const dateLayout = "2006-01-02"

type Cost interface {
	ProjectCosts(ctx context.Context, organizationID string, from, to time.Time) (map[string]float64, error)
	DailyCosts(ctx context.Context, organizationID string, from, to time.Time) (*DailyCosts, error)
}

type DailyCosts struct {
	Projects     map[string]ProjectDays
	LastModified time.Time
}

type ProjectDays struct {
	Name string
	EUR  map[string]float64
}

type cost struct {
	api costv3.DefaultAPI
}

func newCost(client *costv3.APIClient) Cost {
	return &cost{api: client.DefaultAPI}
}

func (c *cost) ProjectCosts(ctx context.Context, organizationID string, from, to time.Time) (map[string]float64, error) {
	resp, err := c.api.ListCostsForCustomer(ctx, organizationID).
		From(from.Format(dateLayout)).
		To(to.Format(dateLayout)).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("listing costs %s .. %s: %w", from.Format(dateLayout), to.Format(dateLayout), err)
	}
	out := make(map[string]float64, len(resp))
	for _, p := range resp {
		var projectID string
		var cents float64
		switch {
		case p.ProjectCostWithDetailedServices != nil:
			projectID, cents = p.ProjectCostWithDetailedServices.ProjectId, p.ProjectCostWithDetailedServices.TotalCharge
		case p.ProjectCostWithSummarizedServices != nil:
			projectID, cents = p.ProjectCostWithSummarizedServices.ProjectId, p.ProjectCostWithSummarizedServices.TotalCharge
		case p.SummarizedProjectCost != nil:
			projectID, cents = p.SummarizedProjectCost.ProjectId, p.SummarizedProjectCost.TotalCharge
		default:
			continue
		}
		if projectID != "" {
			out[projectID] += cents / 100
		}
	}
	return out, nil
}

func (c *cost) DailyCosts(ctx context.Context, organizationID string, from, to time.Time) (*DailyCosts, error) {
	var httpResp *http.Response
	resp, err := c.api.ListCostsForCustomer(runtime.WithCaptureHTTPResponse(ctx, &httpResp), organizationID).
		From(from.Format(dateLayout)).
		To(to.Format(dateLayout)).
		Depth(costv3.LISTCOSTSFORCUSTOMERDEPTHPARAMETER_PROJECT).
		Granularity(costv3.LISTCOSTSFORCUSTOMERGRANULARITYPARAMETER_DAILY).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("listing daily costs %s .. %s: %w", from.Format(dateLayout), to.Format(dateLayout), err)
	}
	out := &DailyCosts{Projects: make(map[string]ProjectDays, len(resp))}
	if httpResp != nil {
		if t, err := http.ParseTime(httpResp.Header.Get("Last-Modified")); err == nil {
			out.LastModified = t.UTC()
		}
	}
	for _, p := range resp {
		pc, err := reportData(p)
		if err != nil {
			return nil, err
		}
		if pc.id == "" {
			continue
		}
		if len(pc.days) == 0 && pc.totalCents != 0 {
			return nil, fmt.Errorf("the Cost API returned a total but no daily costs for project %s (%s)", pc.name, pc.id)
		}
		days := out.Projects[pc.id]
		if days.EUR == nil {
			days = ProjectDays{Name: pc.name, EUR: map[string]float64{}}
		}
		for _, d := range pc.days {
			day := d.TimePeriod.GetStart()
			if _, err := time.Parse(dateLayout, day); err != nil {
				return nil, fmt.Errorf("the Cost API returned the day %q for project %s, expected YYYY-MM-DD", day, pc.id)
			}
			days.EUR[day] += d.Charge / 100
		}
		out.Projects[pc.id] = days
	}
	return out, nil
}

type projectCost struct {
	id, name   string
	totalCents float64
	days       []costv3.ReportData
}

func reportData(p costv3.ProjectCost) (projectCost, error) {
	var pc projectCost
	var extra map[string]any
	switch {
	case p.ProjectCostWithReports != nil:
		r := p.ProjectCostWithReports
		return projectCost{id: r.ProjectId, name: r.ProjectName, totalCents: r.TotalCharge, days: r.ReportData}, nil
	case p.ProjectCostWithDetailedServices != nil:
		r := p.ProjectCostWithDetailedServices
		pc, extra = projectCost{id: r.ProjectId, name: r.ProjectName, totalCents: r.TotalCharge}, r.AdditionalProperties
	case p.ProjectCostWithSummarizedServices != nil:
		r := p.ProjectCostWithSummarizedServices
		pc, extra = projectCost{id: r.ProjectId, name: r.ProjectName, totalCents: r.TotalCharge}, r.AdditionalProperties
	case p.SummarizedProjectCost != nil:
		r := p.SummarizedProjectCost
		pc, extra = projectCost{id: r.ProjectId, name: r.ProjectName, totalCents: r.TotalCharge}, r.AdditionalProperties
	default:
		return pc, nil
	}
	raw, ok := extra["reportData"]
	if !ok || raw == nil {
		return pc, nil
	}
	data, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(data, &pc.days)
	}
	if err != nil {
		return pc, fmt.Errorf("reading the daily costs of project %s: %w", pc.id, err)
	}
	return pc, nil
}
