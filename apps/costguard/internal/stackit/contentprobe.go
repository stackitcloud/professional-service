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

	objectstoragev1 "github.com/stackitcloud/stackit-sdk-go/services/objectstorage/v1api"
	skev1 "github.com/stackitcloud/stackit-sdk-go/services/ske/v1api"
)

// SKE abstracts the read-only SKE surface used by the project content
// probe.
type SKE interface {
	// ListClusterNames returns the names of all clusters in the project.
	ListClusterNames(ctx context.Context, projectID string) ([]string, error)
}

type ske struct {
	client *skev1.APIClient
}

func newSKE(client *skev1.APIClient) SKE {
	return &ske{client: client}
}

func (s *ske) ListClusterNames(ctx context.Context, projectID string) ([]string, error) {
	resp, err := s.client.DefaultAPI.ListClusters(ctx, projectID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing SKE clusters in project %s: %w", projectID, err)
	}
	names := make([]string, 0)
	if resp == nil || resp.Items == nil {
		return names, nil
	}
	for _, c := range resp.Items {
		if n := c.GetName(); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// ObjectStorage abstracts the read-only object storage surface used by
// the project content probe.
type ObjectStorage interface {
	// ListBucketNames returns the names of all buckets in the project.
	ListBucketNames(ctx context.Context, projectID string) ([]string, error)
}

type objectStorage struct {
	client *objectstoragev1.APIClient
}

func newObjectStorage(client *objectstoragev1.APIClient) ObjectStorage {
	return &objectStorage{client: client}
}

func (o *objectStorage) ListBucketNames(ctx context.Context, projectID string) ([]string, error) {
	resp, err := o.client.DefaultAPI.ListBuckets(ctx, projectID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing buckets in project %s: %w", projectID, err)
	}
	names := make([]string, 0)
	if resp == nil || resp.Buckets == nil {
		return names, nil
	}
	for _, b := range resp.Buckets {
		if b.Name != "" {
			names = append(names, b.Name)
		}
	}
	return names, nil
}
