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

// Kind is one of the "IaaS basics" resource types costguard handles.
type Kind string

// The IaaS kinds.
const (
	KindServer        Kind = "server"
	KindPublicIP      Kind = "publicip"
	KindNIC           Kind = "nic"
	KindSnapshot      Kind = "snapshot"
	KindVolume        Kind = "volume"
	KindImage         Kind = "image"
	KindSecurityGroup Kind = "securitygroup"
)

var kindNames = map[Kind]string{
	KindServer: "server", KindPublicIP: "public IP", KindNIC: "network interface", KindSnapshot: "snapshot",
	KindVolume: "volume", KindImage: "image", KindSecurityGroup: "security group",
}

// Name is the kind's name for people, e.g. "public IP".
func (k Kind) Name() string {
	if n, ok := kindNames[k]; ok {
		return n
	}
	return string(k)
}

// Kinds lists every kind in deletion order: servers first (their NICs and
// volumes are released with them), security groups last (NICs use them).
var Kinds = []Kind{KindServer, KindPublicIP, KindNIC, KindSnapshot, KindVolume, KindImage, KindSecurityGroup}

// Server and volume states costguard looks at.
const (
	VolumeStatusAvailable = "AVAILABLE"
	ServerStatusDeleting  = "DELETING"
)

// Resource is one IaaS resource. Fields that do not apply to a kind are
// empty.
type Resource struct {
	Kind      Kind
	ID        string
	Name      string
	ProjectID string
	Region    string
	Labels    map[string]string
	Status    string
	CreatedAt time.Time

	// ServerID is the server a volume is attached to, or a NIC's device.
	ServerID string
	// NetworkID is a NIC's network (needed to get or delete it).
	NetworkID string
	// NICID is the NIC a public IP is attached to.
	NICID string
	// Address is a public IP's address.
	Address string
	// VolumeID is a snapshot's source volume.
	VolumeID string
	// SizeGB is a volume's size.
	SizeGB int64
}

// NetworkArea is an organization-level network area (SNA).
type NetworkArea struct {
	ID           string
	Name         string
	ProjectCount int64
	Labels       map[string]string
	CreatedAt    time.Time
}

// IaaS is the IaaS surface costguard needs.
type IaaS interface {
	// List returns all resources of a kind in one project and region.
	List(ctx context.Context, kind Kind, projectID, region string) ([]Resource, error)
	// Get re-reads a resource (Kind, ID, ProjectID, Region and, for NICs,
	// NetworkID must be set).
	Get(ctx context.Context, ref Resource) (*Resource, error)
	// Delete deletes a resource.
	Delete(ctx context.Context, ref Resource) error
	// SetLabel sets (value non-nil) or removes (value nil) one label. Only
	// volumes and public IPs are supported: they are the only kinds
	// costguard labels.
	SetLabel(ctx context.Context, ref Resource, key string, value *string) error
	// ListNetworkAreas lists the organization's network areas.
	ListNetworkAreas(ctx context.Context, organizationID string) ([]NetworkArea, error)
}

type iaas struct {
	api iaasv2.DefaultAPI
}

func newIaaS(client *iaasv2.APIClient) IaaS {
	return &iaas{api: client.DefaultAPI}
}

func (a *iaas) List(ctx context.Context, kind Kind, projectID, region string) ([]Resource, error) {
	out, err := a.list(ctx, kind, projectID, region)
	if err != nil {
		return nil, fmt.Errorf("listing %ss in %s/%s: %w", kind, projectID, region, err)
	}
	return out, nil
}

func (a *iaas) list(ctx context.Context, kind Kind, p, r string) ([]Resource, error) {
	var out []Resource
	switch kind {
	case KindServer:
		resp, err := a.api.ListServers(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromServer(&resp.Items[i], p, r))
		}
	case KindVolume:
		resp, err := a.api.ListVolumes(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromVolume(&resp.Items[i], p, r))
		}
	case KindPublicIP:
		resp, err := a.api.ListPublicIPs(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromPublicIP(&resp.Items[i], p, r))
		}
	case KindSnapshot:
		resp, err := a.api.ListSnapshotsInProject(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromSnapshot(&resp.Items[i], p, r))
		}
	case KindImage:
		resp, err := a.api.ListImages(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			img := &resp.Items[i]
			// Only the project's own images; public or shared images are
			// never costguard's business.
			if owner := img.GetOwner(); owner != "" && owner != p {
				continue
			}
			out = append(out, fromImage(img, p, r))
		}
	case KindNIC:
		resp, err := a.api.ListProjectNICs(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromNIC(&resp.Items[i], p, r))
		}
	case KindSecurityGroup:
		resp, err := a.api.ListSecurityGroups(ctx, p, r).Execute()
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			out = append(out, fromSecurityGroup(&resp.Items[i], p, r))
		}
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
	return out, nil
}

func (a *iaas) Get(ctx context.Context, ref Resource) (*Resource, error) {
	p, r := ref.ProjectID, ref.Region
	var res Resource
	switch ref.Kind {
	case KindServer:
		v, err := a.api.GetServer(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromServer(v, p, r)
	case KindVolume:
		v, err := a.api.GetVolume(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromVolume(v, p, r)
	case KindPublicIP:
		v, err := a.api.GetPublicIP(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromPublicIP(v, p, r)
	case KindSnapshot:
		v, err := a.api.GetSnapshot(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromSnapshot(v, p, r)
	case KindImage:
		v, err := a.api.GetImage(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromImage(v, p, r)
	case KindNIC:
		v, err := a.api.GetNic(ctx, p, r, ref.NetworkID, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromNIC(v, p, r)
	case KindSecurityGroup:
		v, err := a.api.GetSecurityGroup(ctx, p, r, ref.ID).Execute()
		if err != nil {
			return nil, getErr(ref, err)
		}
		res = fromSecurityGroup(v, p, r)
	default:
		return nil, fmt.Errorf("unsupported kind %q", ref.Kind)
	}
	return &res, nil
}

func getErr(ref Resource, err error) error {
	return fmt.Errorf("getting %s %s: %w", ref.Kind, ref.ID, err)
}

func (a *iaas) Delete(ctx context.Context, ref Resource) error {
	p, r := ref.ProjectID, ref.Region
	var err error
	switch ref.Kind {
	case KindServer:
		err = a.api.DeleteServer(ctx, p, r, ref.ID).Execute()
	case KindVolume:
		err = a.api.DeleteVolume(ctx, p, r, ref.ID).Execute()
	case KindPublicIP:
		err = a.api.DeletePublicIP(ctx, p, r, ref.ID).Execute()
	case KindSnapshot:
		err = a.api.DeleteSnapshot(ctx, p, r, ref.ID).Execute()
	case KindImage:
		err = a.api.DeleteImage(ctx, p, r, ref.ID).Execute()
	case KindNIC:
		err = a.api.DeleteNic(ctx, p, r, ref.NetworkID, ref.ID).Execute()
	case KindSecurityGroup:
		err = a.api.DeleteSecurityGroup(ctx, p, r, ref.ID).Execute()
	default:
		return fmt.Errorf("unsupported kind %q", ref.Kind)
	}
	if err != nil {
		return fmt.Errorf("deleting %s %s: %w", ref.Kind, ref.ID, err)
	}
	return nil
}

func (a *iaas) SetLabel(ctx context.Context, ref Resource, key string, value *string) error {
	// A null value removes the key; other labels are left alone (the API
	// merges the patch).
	patch := map[string]interface{}{key: nil}
	if value != nil {
		patch[key] = *value
	}
	var err error
	switch ref.Kind {
	case KindVolume:
		_, err = a.api.UpdateVolume(ctx, ref.ProjectID, ref.Region, ref.ID).
			UpdateVolumePayload(iaasv2.UpdateVolumePayload{Labels: patch}).Execute()
	case KindPublicIP:
		_, err = a.api.UpdatePublicIP(ctx, ref.ProjectID, ref.Region, ref.ID).
			UpdatePublicIPPayload(iaasv2.UpdatePublicIPPayload{Labels: patch}).Execute()
	default:
		return fmt.Errorf("setting labels on %s is not supported", ref.Kind)
	}
	if err != nil {
		return fmt.Errorf("updating labels of %s %s: %w", ref.Kind, ref.ID, err)
	}
	return nil
}

func (a *iaas) ListNetworkAreas(ctx context.Context, organizationID string) ([]NetworkArea, error) {
	resp, err := a.api.ListNetworkAreas(ctx, organizationID).Execute()
	if err != nil {
		return nil, fmt.Errorf("listing network areas of organization %s: %w", organizationID, err)
	}
	out := make([]NetworkArea, 0, len(resp.Items))
	for _, area := range resp.Items {
		out = append(out, NetworkArea{
			ID:           area.GetId(),
			Name:         area.GetName(),
			ProjectCount: area.GetProjectCount(),
			Labels:       stringLabels(area.GetLabels()),
			CreatedAt:    area.GetCreatedAt(),
		})
	}
	return out, nil
}

// ---- SDK model mapping ----

func fromServer(v *iaasv2.Server, p, r string) Resource {
	return Resource{Kind: KindServer, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), Status: v.GetStatus(), CreatedAt: v.GetCreatedAt()}
}

func fromVolume(v *iaasv2.Volume, p, r string) Resource {
	return Resource{Kind: KindVolume, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), Status: v.GetStatus(), CreatedAt: v.GetCreatedAt(),
		ServerID: v.GetServerId(), SizeGB: v.GetSize()}
}

func fromPublicIP(v *iaasv2.PublicIp, p, r string) Resource {
	nic := ""
	if !iaasv2.IsNil(v.NetworkInterface) {
		nic = v.GetNetworkInterface()
	}
	return Resource{Kind: KindPublicIP, ID: v.GetId(), Name: v.GetIp(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), NICID: nic, Address: v.GetIp()}
}

func fromSnapshot(v *iaasv2.Snapshot, p, r string) Resource {
	return Resource{Kind: KindSnapshot, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), Status: v.GetStatus(), CreatedAt: v.GetCreatedAt(),
		VolumeID: v.GetVolumeId(), SizeGB: v.GetSize()}
}

func fromImage(v *iaasv2.Image, p, r string) Resource {
	return Resource{Kind: KindImage, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), Status: v.GetStatus(), CreatedAt: v.GetCreatedAt()}
}

func fromNIC(v *iaasv2.NIC, p, r string) Resource {
	return Resource{Kind: KindNIC, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), Status: v.GetStatus(),
		ServerID: v.GetDevice(), NetworkID: v.GetNetworkId()}
}

func fromSecurityGroup(v *iaasv2.SecurityGroup, p, r string) Resource {
	return Resource{Kind: KindSecurityGroup, ID: v.GetId(), Name: v.GetName(), ProjectID: p, Region: r,
		Labels: stringLabels(v.GetLabels()), CreatedAt: v.GetCreatedAt()}
}
