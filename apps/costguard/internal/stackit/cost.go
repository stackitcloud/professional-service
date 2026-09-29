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

// dateLayout is the granularity of the Cost API's from/to parameters.
const dateLayout = "2006-01-02"

// Cost is the billing data source.
type Cost interface {
	// ProjectCosts returns each project's total charge in EUR for the
	// inclusive date range. Projects without charges may be missing.
	ProjectCosts(ctx context.Context, organizationID string, from, to time.Time) (map[string]float64, error)
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
