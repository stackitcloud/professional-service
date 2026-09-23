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
	"fmt"
	"time"

	costv3 "github.com/stackitcloud/stackit-sdk-go/services/cost/v3api"
)

// dateLayout is the granularity of the cost API's from/to parameters.
const dateLayout = "2006-01-02"

// CostRecord is one project's charge for the requested date range.
type CostRecord struct {
	ProjectID   string
	ProjectName string
	ChargeEUR   float64
}

// Cost abstracts the billing data source.
type Cost interface {
	// ListCostsForCustomer returns the per-project charges for the date
	// range [from, to] (inclusive, day granularity).
	ListCostsForCustomer(ctx context.Context, customerAccountID string, from, to time.Time) ([]CostRecord, error)
}

type cost struct {
	client *costv3.APIClient
}

func newCost(client *costv3.APIClient) Cost {
	return &cost{client: client}
}

// ListCostsForCustomer calls the cost API once per requested range. The
// API returns range totals per project, not per-day series, so per-day
// data is assembled by the caller issuing one call per day. The
// per-project breakdown is preserved so both daily totals and per-project
// window totals can be aggregated.
func (c *cost) ListCostsForCustomer(ctx context.Context, customerAccountID string, from, to time.Time) ([]CostRecord, error) {
	resp, err := c.client.DefaultAPI.ListCostsForCustomer(ctx, customerAccountID).
		From(from.Format(dateLayout)).
		To(to.Format(dateLayout)).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("listing costs for %s .. %s: %w", from.Format(dateLayout), to.Format(dateLayout), err)
	}
	records := make([]CostRecord, 0, len(resp))
	for _, p := range resp {
		var projectID, projectName string
		var chargeCents float64
		switch {
		case p.ProjectCostWithDetailedServices != nil:
			projectID = p.ProjectCostWithDetailedServices.ProjectId
			projectName = p.ProjectCostWithDetailedServices.ProjectName
			chargeCents = p.ProjectCostWithDetailedServices.TotalCharge
		case p.ProjectCostWithSummarizedServices != nil:
			projectID = p.ProjectCostWithSummarizedServices.ProjectId
			projectName = p.ProjectCostWithSummarizedServices.ProjectName
			chargeCents = p.ProjectCostWithSummarizedServices.TotalCharge
		case p.SummarizedProjectCost != nil:
			projectID = p.SummarizedProjectCost.ProjectId
			projectName = p.SummarizedProjectCost.ProjectName
			chargeCents = p.SummarizedProjectCost.TotalCharge
		default:
			continue
		}
		if projectID == "" {
			continue
		}
		// TotalCharge is documented as "value in cents" (verified against
		// the SDK model): divide by 100.
		records = append(records, CostRecord{
			ProjectID:   projectID,
			ProjectName: projectName,
			ChargeEUR:   chargeCents / 100.0,
		})
	}
	return records, nil
}
