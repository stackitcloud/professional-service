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

	albv2 "github.com/stackitcloud/stackit-sdk-go/services/alb/v2api"
	lbv2 "github.com/stackitcloud/stackit-sdk-go/services/loadbalancer/v2api"
	objectstoragev2 "github.com/stackitcloud/stackit-sdk-go/services/objectstorage/v2api"
	skev2 "github.com/stackitcloud/stackit-sdk-go/services/ske/v2api"
)

// Services probes the managed services costguard needs to know about. A
// service that is not enabled in a project (see NotEnabled) counts as
// "nothing there".
type Services interface {
	// SKEClusters returns the names of the SKE clusters in a project and
	// region.
	SKEClusters(ctx context.Context, projectID, region string) ([]string, error)
	// Buckets returns the names of the object storage buckets in a project
	// and region.
	Buckets(ctx context.Context, projectID, region string) ([]string, error)
	// LoadBalancerAddresses maps the external address of every network and
	// application load balancer in a project and region to a description
	// such as "network load balancer web".
	LoadBalancerAddresses(ctx context.Context, projectID, region string) (map[string]string, error)
}

// lbMaxPages bounds load balancer pagination.
const lbMaxPages = 100

type services struct {
	ske skev2.DefaultAPI
	obj objectstoragev2.DefaultAPI
	lb  lbv2.DefaultAPI
	alb albv2.DefaultAPI
}

func (s *services) SKEClusters(ctx context.Context, projectID, region string) ([]string, error) {
	resp, err := s.ske.ListClusters(ctx, projectID, region).Execute()
	if NotEnabled(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing SKE clusters in %s/%s: %w", projectID, region, err)
	}
	var names []string
	for _, c := range resp.Items {
		names = append(names, c.GetName())
	}
	return names, nil
}

func (s *services) Buckets(ctx context.Context, projectID, region string) ([]string, error) {
	resp, err := s.obj.ListBuckets(ctx, projectID, region).Execute()
	if NotEnabled(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing buckets in %s/%s: %w", projectID, region, err)
	}
	var names []string
	for _, b := range resp.Buckets {
		names = append(names, b.Name)
	}
	return names, nil
}

func (s *services) LoadBalancerAddresses(ctx context.Context, projectID, region string) (map[string]string, error) {
	out := map[string]string{}
	pageID := ""
	for i := 0; ; i++ {
		if i == lbMaxPages {
			return nil, fmt.Errorf("listing load balancers in %s/%s: more than %d pages", projectID, region, lbMaxPages)
		}
		req := s.lb.ListLoadBalancers(ctx, projectID, region)
		if pageID != "" {
			req = req.PageId(pageID)
		}
		resp, err := req.Execute()
		if NotEnabled(err) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing load balancers in %s/%s: %w", projectID, region, err)
		}
		for _, lb := range resp.LoadBalancers {
			if addr := lb.GetExternalAddress(); addr != "" {
				out[addr] = "network load balancer " + lb.GetName()
			}
		}
		if pageID = resp.GetNextPageId(); pageID == "" {
			break
		}
	}
	pageID = ""
	for i := 0; ; i++ {
		if i == lbMaxPages {
			return nil, fmt.Errorf("listing application load balancers in %s/%s: more than %d pages", projectID, region, lbMaxPages)
		}
		req := s.alb.ListLoadBalancers(ctx, projectID, region)
		if pageID != "" {
			req = req.PageId(pageID)
		}
		resp, err := req.Execute()
		if NotEnabled(err) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing application load balancers in %s/%s: %w", projectID, region, err)
		}
		for _, lb := range resp.LoadBalancers {
			if addr := lb.GetExternalAddress(); addr != "" {
				out[addr] = "application load balancer " + lb.GetName()
			}
		}
		if pageID = resp.GetNextPageId(); pageID == "" {
			break
		}
	}
	return out, nil
}
