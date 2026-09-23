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

	iaasv2 "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
)

// PublicIP is the bot's view of an IaaS public IP.
type PublicIP struct {
	ID          string
	Address     string
	ProjectID   string
	Region      string
	Labels      map[string]string
	AttachedNIC string
}

// Volume is the bot's view of an IaaS block storage volume.
type Volume struct {
	ID        string
	Name      string
	ProjectID string
	Region    string
	SizeGB    int64
	Status    string
	ServerID  string
	Labels    map[string]string
	CreatedAt time.Time
}

// NetworkArea is the bot's view of an IaaS network area (SNA).
type NetworkArea struct {
	ID           string
	Name         string
	ProjectCount int64
	Labels       map[string]string
	CreatedAt    time.Time
}

// Snapshot is the bot's view of an IaaS volume snapshot.
type Snapshot struct {
	ID       string
	Name     string
	VolumeID string
}

// Server is the bot's view of an IaaS server (content probe only).
type Server struct {
	ID     string
	Name   string
	Status string
}

// IaaS abstracts the IaaS surface the bot needs. Deletion is limited to
// public IPs and volumes in v1: there is no network-area or project
// deletion method on this interface on purpose.
type IaaS interface {
	// ListNetworkAreas lists all SNAs of the organisation (hygiene scan).
	ListNetworkAreas(ctx context.Context, organizationID string) ([]NetworkArea, error)
	// GetNetworkArea fetches one SNA (callback existence check).
	GetNetworkArea(ctx context.Context, organizationID, areaID string) (*NetworkArea, error)

	// ListPublicIPs lists the public IPs of one project in one region.
	ListPublicIPs(ctx context.Context, projectID, region string) ([]PublicIP, error)
	// GetPublicIP fetches one public IP (re-validation / callback).
	GetPublicIP(ctx context.Context, projectID, region, ipID string) (*PublicIP, error)
	// SetPublicIPMark sets the mark label value on the IP.
	SetPublicIPMark(ctx context.Context, projectID, region, ipID, value string) error
	// ClearPublicIPMark removes the mark label from the IP.
	ClearPublicIPMark(ctx context.Context, projectID, region, ipID string) error
	// DeletePublicIP deletes one public IP.
	DeletePublicIP(ctx context.Context, projectID, region, ipID string) error

	// ListVolumes lists the volumes of one project in one region.
	ListVolumes(ctx context.Context, projectID, region string) ([]Volume, error)
	// GetVolume fetches one volume (re-validation / callback).
	GetVolume(ctx context.Context, projectID, region, volumeID string) (*Volume, error)
	// SetVolumeMark sets the mark label value on the volume.
	SetVolumeMark(ctx context.Context, projectID, region, volumeID, value string) error
	// ClearVolumeMark removes the mark label from the volume.
	ClearVolumeMark(ctx context.Context, projectID, region, volumeID string) error
	// DeleteVolume deletes one volume.
	DeleteVolume(ctx context.Context, projectID, region, volumeID string) error

	// ListSnapshots lists all snapshots of one project in one region; the
	// caller filters by VolumeID (snapshots must be deleted
	// before their volume).
	ListSnapshots(ctx context.Context, projectID, region string) ([]Snapshot, error)
	// DeleteSnapshot deletes one snapshot.
	DeleteSnapshot(ctx context.Context, projectID, region, snapshotID string) error

	// ListServers lists the servers of one project in one region
	// (hygiene content probe).
	ListServers(ctx context.Context, projectID, region string) ([]Server, error)
}

type iaas struct {
	client *iaasv2.APIClient
}

func newIaaS(client *iaasv2.APIClient) IaaS {
	return &iaas{client: client}
}

func (a *iaas) ListNetworkAreas(ctx context.Context, organizationID string) ([]NetworkArea, error) {
	resp, err := a.client.DefaultAPI.ListNetworkAreas(ctx, organizationID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing network areas of organisation %s: %w", organizationID, err)
	}
	areas := make([]NetworkArea, 0)
	if resp == nil || resp.Items == nil {
		return areas, nil
	}
	for _, area := range resp.Items {
		count := int64(0)
		if area.ProjectCount != nil {
			count = *area.ProjectCount
		}
		areas = append(areas, NetworkArea{
			ID:           area.GetId(),
			Name:         area.GetName(),
			ProjectCount: count,
			Labels:       stringLabels(area.GetLabels()),
			CreatedAt:    area.GetCreatedAt(),
		})
	}
	return areas, nil
}

func (a *iaas) GetNetworkArea(ctx context.Context, organizationID, areaID string) (*NetworkArea, error) {
	area, err := a.client.DefaultAPI.GetNetworkArea(ctx, organizationID, areaID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting network area %s: %w", areaID, err)
	}
	count := int64(0)
	if area.ProjectCount != nil {
		count = *area.ProjectCount
	}
	na := NetworkArea{
		ID:           area.GetId(),
		Name:         area.GetName(),
		ProjectCount: count,
		Labels:       stringLabels(area.GetLabels()),
		CreatedAt:    area.GetCreatedAt(),
	}
	return &na, nil
}

func (a *iaas) ListPublicIPs(ctx context.Context, projectID, region string) ([]PublicIP, error) {
	resp, err := a.client.DefaultAPI.ListPublicIPs(ctx, projectID, region).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing public IPs in %s/%s: %w", projectID, region, err)
	}
	ips := make([]PublicIP, 0)
	if resp == nil || resp.Items == nil {
		return ips, nil
	}
	for _, ip := range resp.Items {
		nic := ""
		if !iaasv2.IsNil(ip.NetworkInterface) && ip.GetNetworkInterface() != "" {
			nic = ip.GetNetworkInterface()
		}
		ips = append(ips, PublicIP{
			ID:          ip.GetId(),
			Address:     ip.GetIp(),
			ProjectID:   projectID,
			Region:      region,
			Labels:      stringLabels(ip.GetLabels()),
			AttachedNIC: nic,
		})
	}
	return ips, nil
}

func (a *iaas) GetPublicIP(ctx context.Context, projectID, region, ipID string) (*PublicIP, error) {
	ip, err := a.client.DefaultAPI.GetPublicIP(ctx, projectID, region, ipID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting public IP %s: %w", ipID, err)
	}
	nic := ""
	if !iaasv2.IsNil(ip.NetworkInterface) && ip.GetNetworkInterface() != "" {
		nic = ip.GetNetworkInterface()
	}
	out := PublicIP{
		ID:          ip.GetId(),
		Address:     ip.GetIp(),
		ProjectID:   projectID,
		Region:      region,
		Labels:      stringLabels(ip.GetLabels()),
		AttachedNIC: nic,
	}
	return &out, nil
}

// updateIPLabels sends a label patch for the mark key: a non-nil value
// sets it, nil removes it (SDK semantics: null value deletes the key).
func (a *iaas) updateIPLabels(ctx context.Context, projectID, region, ipID string, mark map[string]interface{}) error {
	_, err := a.client.DefaultAPI.UpdatePublicIP(ctx, projectID, region, ipID).
		UpdatePublicIPPayload(iaasv2.UpdatePublicIPPayload{Labels: mark}).
		Execute()
	if err != nil {
		return fmt.Errorf("updating labels of public IP %s: %w", ipID, err)
	}
	return nil
}

func (a *iaas) SetPublicIPMark(ctx context.Context, projectID, region, ipID, value string) error {
	return a.updateIPLabels(ctx, projectID, region, ipID, map[string]interface{}{markLabelKey: value})
}

func (a *iaas) ClearPublicIPMark(ctx context.Context, projectID, region, ipID string) error {
	return a.updateIPLabels(ctx, projectID, region, ipID, map[string]interface{}{markLabelKey: nil})
}

func (a *iaas) DeletePublicIP(ctx context.Context, projectID, region, ipID string) error {
	if err := a.client.DefaultAPI.DeletePublicIP(ctx, projectID, region, ipID).Execute(); err != nil {
		return fmt.Errorf("deleting public IP %s: %w", ipID, err)
	}
	return nil
}

func (a *iaas) ListVolumes(ctx context.Context, projectID, region string) ([]Volume, error) {
	resp, err := a.client.DefaultAPI.ListVolumes(ctx, projectID, region).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing volumes in %s/%s: %w", projectID, region, err)
	}
	volumes := make([]Volume, 0)
	if resp == nil || resp.Items == nil {
		return volumes, nil
	}
	for _, v := range resp.Items {
		size := int64(0)
		if v.Size != nil {
			size = *v.Size
		}
		volumes = append(volumes, Volume{
			ID:        v.GetId(),
			Name:      v.GetName(),
			ProjectID: projectID,
			Region:    region,
			SizeGB:    size,
			Status:    v.GetStatus(),
			ServerID:  v.GetServerId(),
			Labels:    stringLabels(v.GetLabels()),
			CreatedAt: v.GetCreatedAt(),
		})
	}
	return volumes, nil
}

func (a *iaas) GetVolume(ctx context.Context, projectID, region, volumeID string) (*Volume, error) {
	v, err := a.client.DefaultAPI.GetVolume(ctx, projectID, region, volumeID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting volume %s: %w", volumeID, err)
	}
	size := int64(0)
	if v.Size != nil {
		size = *v.Size
	}
	out := Volume{
		ID:        v.GetId(),
		Name:      v.GetName(),
		ProjectID: projectID,
		Region:    region,
		SizeGB:    size,
		Status:    v.GetStatus(),
		ServerID:  v.GetServerId(),
		Labels:    stringLabels(v.GetLabels()),
		CreatedAt: v.GetCreatedAt(),
	}
	return &out, nil
}

func (a *iaas) updateVolumeLabels(ctx context.Context, projectID, region, volumeID string, mark map[string]interface{}) error {
	_, err := a.client.DefaultAPI.UpdateVolume(ctx, projectID, region, volumeID).
		UpdateVolumePayload(iaasv2.UpdateVolumePayload{Labels: mark}).
		Execute()
	if err != nil {
		return fmt.Errorf("updating labels of volume %s: %w", volumeID, err)
	}
	return nil
}

func (a *iaas) SetVolumeMark(ctx context.Context, projectID, region, volumeID, value string) error {
	return a.updateVolumeLabels(ctx, projectID, region, volumeID, map[string]interface{}{markLabelKey: value})
}

func (a *iaas) ClearVolumeMark(ctx context.Context, projectID, region, volumeID string) error {
	return a.updateVolumeLabels(ctx, projectID, region, volumeID, map[string]interface{}{markLabelKey: nil})
}

func (a *iaas) DeleteVolume(ctx context.Context, projectID, region, volumeID string) error {
	if err := a.client.DefaultAPI.DeleteVolume(ctx, projectID, region, volumeID).Execute(); err != nil {
		return fmt.Errorf("deleting volume %s: %w", volumeID, err)
	}
	return nil
}

func (a *iaas) ListSnapshots(ctx context.Context, projectID, region string) ([]Snapshot, error) {
	resp, err := a.client.DefaultAPI.ListSnapshotsInProject(ctx, projectID, region).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing snapshots in %s/%s: %w", projectID, region, err)
	}
	snaps := make([]Snapshot, 0)
	if resp == nil || resp.Items == nil {
		return snaps, nil
	}
	for _, s := range resp.Items {
		snaps = append(snaps, Snapshot{
			ID:       s.GetId(),
			Name:     s.GetName(),
			VolumeID: s.GetVolumeId(),
		})
	}
	return snaps, nil
}

func (a *iaas) DeleteSnapshot(ctx context.Context, projectID, region, snapshotID string) error {
	if err := a.client.DefaultAPI.DeleteSnapshot(ctx, projectID, region, snapshotID).Execute(); err != nil {
		return fmt.Errorf("deleting snapshot %s: %w", snapshotID, err)
	}
	return nil
}

func (a *iaas) ListServers(ctx context.Context, projectID, region string) ([]Server, error) {
	resp, err := a.client.DefaultAPI.ListServers(ctx, projectID, region).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing servers in %s/%s: %w", projectID, region, err)
	}
	servers := make([]Server, 0)
	if resp == nil || resp.Items == nil {
		return servers, nil
	}
	for _, s := range resp.Items {
		status := ""
		if s.Status != nil {
			status = *s.Status
		}
		servers = append(servers, Server{
			ID:     s.GetId(),
			Name:   s.GetName(),
			Status: status,
		})
	}
	return servers, nil
}

// stringLabels converts the SDK's map[string]interface{} label type to
// map[string]string, dropping non-string values (which cannot occur for
// well-formed labels).
func stringLabels(in map[string]interface{}) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
