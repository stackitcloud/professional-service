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

// dateLayout is the granularity of the Cost API's from/to parameters.
const dateLayout = "2006-01-02"

// Cost is the billing data source.
type Cost interface {
	// ProjectCosts returns each project's total charge in EUR for the
	// inclusive date range. Projects without charges may be missing.
	ProjectCosts(ctx context.Context, organizationID string, from, to time.Time) (map[string]float64, error)
	// DailyCosts returns each project's charge per UTC day for the
	// inclusive date range (at most 92 days). Days and projects without
	// charges may be missing.
	DailyCosts(ctx context.Context, organizationID string, from, to time.Time) (*DailyCosts, error)
}

// DailyCosts is one Cost API answer with a charge per project and day.
type DailyCosts struct {
	// Projects maps project IDs to their charges. Deleted projects are
	// included as long as they had charges in the range.
	Projects map[string]ProjectDays
	// LastModified is when STACKIT last updated its cost data (the
	// Last-Modified header); zero when the answer did not say.
	LastModified time.Time
}

// ProjectDays is one project's charges.
type ProjectDays struct {
	Name string
	// EUR maps a UTC day (2006-01-02) to the charge in EUR, discounts
	// included.
	EUR map[string]float64
}

type cost struct {
	api costv3.DefaultAPI
}

func newCost(client *costv3.APIClient) Cost {
	return &cost{api: client.DefaultAPI}
}

// ProjectCosts makes one call for the whole range: the API returns range
// totals per project. The organization ID is the customer account ID.
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
			// TotalCharge is in cents.
			out[projectID] += cents / 100
		}
	}
	return out, nil
}

// DailyCosts makes one call with daily granularity. STACKIT updates the
// data once a day (the previous day is in after 07:30 UTC) and may still
// correct it during the month.
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
			// Charges are in cents.
			days.EUR[day] += d.Charge / 100
		}
		out.Projects[pc.id] = days
	}
	return out, nil
}

// projectCost is the part of a ProjectCost that DailyCosts needs.
type projectCost struct {
	id, name   string
	totalCents float64
	days       []costv3.ReportData
}

// reportData extracts a project's daily charges. The SDK decodes the
// answer's anyOf into the first variant whose required fields are present,
// and all four share them, so ProjectCostWithDetailedServices always wins
// and the report data ends up untyped in its additional properties. It is
// decoded from there (or taken from ProjectCostWithReports, should a later
// SDK fill that).
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
