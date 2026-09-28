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

// Package stackit wraps the official STACKIT SDK clients behind small
// interfaces, so the rest of costguard never sees SDK types and tests never
// touch real APIs. Authentication is the SDK's own (workload identity, a
// service account key file or a token, all from the environment).
package stackit

import (
	"fmt"

	coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
	albv2 "github.com/stackitcloud/stackit-sdk-go/services/alb/v2api"
	costv3 "github.com/stackitcloud/stackit-sdk-go/services/cost/v3api"
	iaasv2 "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
	lbv2 "github.com/stackitcloud/stackit-sdk-go/services/loadbalancer/v2api"
	objectstoragev2 "github.com/stackitcloud/stackit-sdk-go/services/objectstorage/v2api"
	resourcemanagerv0 "github.com/stackitcloud/stackit-sdk-go/services/resourcemanager/v0api"
	skev2 "github.com/stackitcloud/stackit-sdk-go/services/ske/v2api"
)

// ConfigurationOption re-exports the SDK option type for callers and tests.
type ConfigurationOption = coreconfig.ConfigurationOption

// Set bundles everything costguard calls.
type Set struct {
	ResourceManager ResourceManager
	IaaS            IaaS
	Cost            Cost
	Services        Services
}

// New builds the client set. Without options the SDK authenticates from
// the environment.
func New(opts ...ConfigurationOption) (*Set, error) {
	rm, err := resourcemanagerv0.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing resource manager client: %w", err)
	}
	iaas, err := iaasv2.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing IaaS client: %w", err)
	}
	cost, err := costv3.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing cost client: %w", err)
	}
	ske, err := skev2.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing SKE client: %w", err)
	}
	obj, err := objectstoragev2.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing object storage client: %w", err)
	}
	lb, err := lbv2.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing load balancer client: %w", err)
	}
	alb, err := albv2.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing application load balancer client: %w", err)
	}
	return &Set{
		ResourceManager: newResourceManager(rm),
		IaaS:            newIaaS(iaas),
		Cost:            newCost(cost),
		Services: &services{
			ske: ske.DefaultAPI,
			obj: obj.DefaultAPI,
			lb:  lb.DefaultAPI,
			alb: alb.DefaultAPI,
		},
	}, nil
}
