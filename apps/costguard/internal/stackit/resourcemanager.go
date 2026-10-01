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

	resourcemanagerv0 "github.com/stackitcloud/stackit-sdk-go/services/resourcemanager/v0api"
)

type Container struct {
	ID             string
	Name           string
	ParentID       string
	Labels         map[string]string
	CreatedAt      time.Time
	LifecycleState string
	Ancestors      []Ancestor
}

type Ancestor struct {
	ID   string
	Name string
}

type ResourceManager interface {
	ListProjects(ctx context.Context, parentID string) ([]Container, error)
	ListFolders(ctx context.Context, parentID string) ([]Container, error)
	GetProject(ctx context.Context, id string) (*Container, error)
	GetFolder(ctx context.Context, id string) (*Container, error)
	OrganizationName(ctx context.Context, id string) (string, error)
}

const rmPageSize = 100

const rmMaxPages = 1000

type resourceManager struct {
	client *resourcemanagerv0.APIClient
}

func newResourceManager(client *resourcemanagerv0.APIClient) ResourceManager {
	return &resourceManager{client: client}
}

func paginate(what string, page func(offset int) (items int, limit float32, err error)) error {
	offset := 0
	for i := 0; i < rmMaxPages; i++ {
		n, limit, err := page(offset)
		if err != nil {
			return err
		}
		if n == 0 || (limit > 0 && n < int(limit)) {
			return nil
		}
		offset += n
	}
	return fmt.Errorf("listing %s: more than %d pages", what, rmMaxPages)
}

func (r *resourceManager) ListProjects(ctx context.Context, parentID string) ([]Container, error) {
	var out []Container
	err := paginate("projects in "+parentID, func(offset int) (int, float32, error) {
		resp, err := r.client.DefaultAPI.ListProjects(ctx).
			ContainerParentId(parentID).
			Limit(rmPageSize).
			Offset(float32(offset)).
			Execute()
		if err != nil {
			return 0, 0, fmt.Errorf("listing projects in %s: %w", parentID, err)
		}
		for _, p := range resp.Items {
			out = append(out, Container{
				ID:             p.GetProjectId(),
				Name:           p.GetName(),
				ParentID:       p.GetParent().Id,
				Labels:         copyLabels(p.GetLabels()),
				CreatedAt:      p.GetCreationTime(),
				LifecycleState: string(p.GetLifecycleState()),
			})
		}
		return len(resp.Items), resp.Limit, nil
	})
	return out, err
}

func (r *resourceManager) ListFolders(ctx context.Context, parentID string) ([]Container, error) {
	var out []Container
	err := paginate("folders in "+parentID, func(offset int) (int, float32, error) {
		resp, err := r.client.DefaultAPI.ListFolders(ctx).
			ContainerParentId(parentID).
			Limit(rmPageSize).
			Offset(float32(offset)).
			Execute()
		if err != nil {
			return 0, 0, fmt.Errorf("listing folders in %s: %w", parentID, err)
		}
		for _, f := range resp.Items {
			out = append(out, Container{
				ID:        f.GetFolderId(),
				Name:      f.GetName(),
				ParentID:  f.GetParent().Id,
				Labels:    copyLabels(f.GetLabels()),
				CreatedAt: f.GetCreationTime(),
			})
		}
		return len(resp.Items), resp.Limit, nil
	})
	return out, err
}

func (r *resourceManager) GetProject(ctx context.Context, id string) (*Container, error) {
	p, err := r.client.DefaultAPI.GetProject(ctx, id).IncludeParents(true).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting project %s: %w", id, err)
	}
	return &Container{
		ID:             p.GetProjectId(),
		Name:           p.GetName(),
		ParentID:       p.GetParent().Id,
		Labels:         copyLabels(p.GetLabels()),
		CreatedAt:      p.GetCreationTime(),
		LifecycleState: string(p.GetLifecycleState()),
		Ancestors:      folderAncestors(p.GetParents(), p.GetParent().Id),
	}, nil
}

func (r *resourceManager) GetFolder(ctx context.Context, id string) (*Container, error) {
	f, err := r.client.DefaultAPI.GetFolderDetails(ctx, id).IncludeParents(true).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting folder %s: %w", id, err)
	}
	return &Container{
		ID:        f.GetFolderId(),
		Name:      f.GetName(),
		ParentID:  f.GetParent().Id,
		Labels:    copyLabels(f.GetLabels()),
		CreatedAt: f.GetCreationTime(),
		Ancestors: folderAncestors(f.GetParents(), f.GetParent().Id),
	}, nil
}

func (r *resourceManager) OrganizationName(ctx context.Context, id string) (string, error) {
	o, err := r.client.DefaultAPI.GetOrganization(ctx, id).Execute()
	if err != nil {
		return "", fmt.Errorf("getting organization %s: %w", id, err)
	}
	return o.GetName(), nil
}

func folderAncestors(parents []resourcemanagerv0.ParentListInner, directParent string) []Ancestor {
	byID := make(map[string]resourcemanagerv0.ParentListInner, len(parents))
	for _, p := range parents {
		byID[p.Id] = p
	}
	var out []Ancestor
	for id := directParent; len(out) <= len(parents); {
		p, ok := byID[id]
		if !ok || p.Type != resourcemanagerv0.PARENTLISTINNERTYPE_FOLDER {
			break
		}
		out = append(out, Ancestor{ID: p.Id, Name: p.Name})
		if p.ParentId == nil {
			break
		}
		id = *p.ParentId
	}
	return out
}
