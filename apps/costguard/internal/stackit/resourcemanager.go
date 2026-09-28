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

// Container is a project or folder as costguard sees it.
type Container struct {
	ID        string
	Name      string
	ParentID  string
	Labels    map[string]string
	CreatedAt time.Time
	// LifecycleState is set for projects only (e.g. ACTIVE, DELETING).
	LifecycleState string
	// Ancestors lists the folders above the container, nearest first. It
	// is only filled by GetProject and GetFolder.
	Ancestors []Ancestor
}

// Ancestor is one folder above a container. The organization is not
// included.
type Ancestor struct {
	ID   string
	Name string
}

// ResourceManager is the read-only Resource Manager surface.
type ResourceManager interface {
	// ListProjects lists all projects directly inside the container,
	// following pagination.
	ListProjects(ctx context.Context, parentID string) ([]Container, error)
	// ListFolders lists all folders directly inside the container,
	// following pagination.
	ListFolders(ctx context.Context, parentID string) ([]Container, error)
	// GetProject fetches one project including its ancestors.
	GetProject(ctx context.Context, id string) (*Container, error)
	// GetFolder fetches one folder including its ancestors.
	GetFolder(ctx context.Context, id string) (*Container, error)
}

// rmPageSize is the page size we ask for. The API caps it and echoes the
// limit it applied, so pagination follows the echoed value.
const rmPageSize = 100

// rmMaxPages bounds pagination against a misbehaving API.
const rmMaxPages = 1000

type resourceManager struct {
	client *resourcemanagerv0.APIClient
}

func newResourceManager(client *resourcemanagerv0.APIClient) ResourceManager {
	return &resourceManager{client: client}
}

// paginate calls page with increasing offsets until a short or empty page.
// page returns the number of items and the limit the API applied.
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

// folderAncestors orders the folder entries of the parents list nearest
// first by following parent links from the direct parent. The API does not
// document the order of the list, so it is not relied on.
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
