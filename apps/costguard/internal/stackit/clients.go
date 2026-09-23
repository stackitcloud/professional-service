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

// Package stackit wraps the official STACKIT SDK clients behind narrow
// interfaces (7.3): every external STACKIT API boundary goes
// through here, so unit tests never hit real APIs. Client initialisation
// uses the SDK's own authentication (service account key file, token, or
// workload identity via environment variables — the bot never handles key
// material itself).
package stackit

import (
	"fmt"

	coreconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
	costv3 "github.com/stackitcloud/stackit-sdk-go/services/cost/v3api"
	iaasv2 "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
	objectstoragev1 "github.com/stackitcloud/stackit-sdk-go/services/objectstorage/v1api"
	resourcemanagerv0 "github.com/stackitcloud/stackit-sdk-go/services/resourcemanager/v0api"
	skev1 "github.com/stackitcloud/stackit-sdk-go/services/ske/v1api"
)

// ConfigurationOption is the SDK core configuration option type,
// re-exported so callers (and tests) can pass endpoint/auth options
// without importing the SDK core package directly.
type ConfigurationOption = coreconfig.ConfigurationOption

// Set bundles all SDK-backed interfaces used by the bot.
type Set struct {
	ResourceManager ResourceManager
	IaaS            IaaS
	Cost            Cost
	SKE             SKE
	ObjectStorage   ObjectStorage
}

// New builds the SDK client set. With no options the clients use the
// SDK's default authentication (env-driven). Tests inject
// config.WithEndpoint + config.WithoutAuthentication to point the clients
// at an httptest server.
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
	ske, err := skev1.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing SKE client: %w", err)
	}
	obj, err := objectstoragev1.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("initializing object storage client: %w", err)
	}
	return &Set{
		ResourceManager: newResourceManager(rm),
		IaaS:            newIaaS(iaas),
		Cost:            newCost(cost),
		SKE:             newSKE(ske),
		ObjectStorage:   newObjectStorage(obj),
	}, nil
}
