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

// Project is the bot's view of a resource manager project.
type Project struct {
	ID                  string
	Name                string
	ParentContainerID   string
	ParentContainerName string
	CreationTime        time.Time
	Labels              map[string]string
	LifecycleState      string
}

// Folder is the bot's view of a resource manager folder.
type Folder struct {
	ID                string
	Name              string
	ParentContainerID string
	Labels            map[string]string
	CreationTime      time.Time
}

// ResourceManager abstracts the read-only Resource Manager surface the
// bot needs: the org-tree walk (ListProjects/ListFolders) and the
// existence checks used by the callback (GetProject/GetFolder). There is
// deliberately no project or folder deletion in v1.
type ResourceManager interface {
	// ListProjects lists the ACTIVE projects directly inside the
	// container (organisation or folder).
	ListProjects(ctx context.Context, containerID string) ([]Project, error)
	// ListFolders lists the folders directly inside the container.
	ListFolders(ctx context.Context, containerID string) ([]Folder, error)
	// GetProject fetches one project by ID (callback existence check).
	GetProject(ctx context.Context, projectID string) (*Project, error)
	// GetFolder fetches one folder by ID (callback existence check).
	GetFolder(ctx context.Context, folderID string) (*Folder, error)
}

type resourceManager struct {
	client *resourcemanagerv0.APIClient
}

func newResourceManager(client *resourcemanagerv0.APIClient) ResourceManager {
	return &resourceManager{client: client}
}

func (r *resourceManager) ListProjects(ctx context.Context, containerID string) ([]Project, error) {
	resp, err := r.client.DefaultAPI.ListProjects(ctx).ContainerParentId(containerID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing projects in container %s: %w", containerID, err)
	}
	projects := make([]Project, 0)
	if resp == nil || resp.Items == nil {
		return projects, nil
	}
	for _, p := range resp.Items {
		projects = append(projects, Project{
			ID:                p.GetProjectId(),
			Name:              p.GetName(),
			ParentContainerID: p.GetParent().ContainerId,
			// The API does not return parent names; the org-tree walk fills
			// ParentContainerName from the container it listed.
			ParentContainerName: "",
			CreationTime:        p.GetCreationTime(),
			Labels:              copyStringLabels(p.GetLabels()),
			LifecycleState:      string(p.GetLifecycleState()),
		})
	}
	return projects, nil
}

func (r *resourceManager) ListFolders(ctx context.Context, containerID string) ([]Folder, error) {
	resp, err := r.client.DefaultAPI.ListFolders(ctx).ContainerParentId(containerID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing folders in container %s: %w", containerID, err)
	}
	folders := make([]Folder, 0)
	if resp == nil || resp.Items == nil {
		return folders, nil
	}
	for _, f := range resp.Items {
		folders = append(folders, Folder{
			ID:                f.GetFolderId(),
			Name:              f.GetName(),
			ParentContainerID: f.GetParent().ContainerId,
			Labels:            copyStringLabels(f.GetLabels()),
			CreationTime:      f.GetCreationTime(),
		})
	}
	return folders, nil
}

func (r *resourceManager) GetProject(ctx context.Context, projectID string) (*Project, error) {
	resp, err := r.client.DefaultAPI.GetProject(ctx, projectID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting project %s: %w", projectID, err)
	}
	p := Project{
		ID:                resp.GetProjectId(),
		Name:              resp.GetName(),
		ParentContainerID: resp.GetParent().ContainerId,
		// The API does not return parent names; the org-tree walk fills
		// ParentContainerName from the container it listed.
		ParentContainerName: "",
		CreationTime:        resp.GetCreationTime(),
		Labels:              copyStringLabels(resp.GetLabels()),
		LifecycleState:      string(resp.GetLifecycleState()),
	}
	return &p, nil
}

func (r *resourceManager) GetFolder(ctx context.Context, folderID string) (*Folder, error) {
	resp, err := r.client.DefaultAPI.GetFolderDetails(ctx, folderID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting folder %s: %w", folderID, err)
	}
	f := Folder{
		ID:                resp.GetFolderId(),
		Name:              resp.GetName(),
		ParentContainerID: resp.GetParent().ContainerId,
		Labels:            copyStringLabels(resp.GetLabels()),
		CreationTime:      resp.GetCreationTime(),
	}
	return &f, nil
}

func copyStringLabels(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
