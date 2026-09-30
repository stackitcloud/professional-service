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

// Package fake is an in-memory STACKIT for tests. It implements every
// interface of internal/stackit and behaves like the real APIs where the
// rules depend on it: deleting a server releases its volumes, NICs and
// public IPs, and missing things answer 404.
package fake

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
)

// Status returns an SDK error with the HTTP status code.
func Status(code int) error {
	return &oapierror.GenericOpenAPIError{StatusCode: code, ErrorMessage: fmt.Sprintf("HTTP %d", code)}
}

// Store is the fake STACKIT.
type Store struct {
	mu sync.Mutex

	// Containers by ID (folders and projects); Folder marks folders.
	Containers map[string]stackit.Container
	Folder     map[string]bool
	Resources  []*stackit.Resource
	Areas      []stackit.NetworkArea
	Costs      map[string]float64
	// Daily holds the Cost API's daily charges per project; LastModified
	// is its Last-Modified time.
	Daily        map[string]stackit.ProjectDays
	LastModified time.Time
	// SKE, BucketNames and LB are keyed by "project/region".
	SKE         map[string][]string
	BucketNames map[string][]string
	LB          map[string]map[string]string

	// Errs injects errors. Keys: "list:<kind>:<project>/<region>",
	// "folders:<parent>", "projects:<parent>", "get:<id>", "delete:<id>",
	// "label:<id>", "costs", "daily", "areas", "ske:<p>/<r>", "buckets:<p>/<r>",
	// "lb:<p>/<r>".
	Errs map[string]error
	// ErrsOnce are like Errs but happen only on the first call (a hiccup).
	ErrsOnce map[string]error
	// ListDescendants makes project and folder listings return everything
	// below the container, not only its direct children.
	ListDescendants bool
	// AfterDelete, when set, runs after every successful delete (e.g. to
	// cancel the run like a SIGTERM would).
	AfterDelete func(ref stackit.Resource)
	// DeletingReads is how many Gets a deleted server still answers (with
	// status DELETING, attachments kept) before it is gone.
	DeletingReads int

	// Calls records every call, e.g. "delete volume v1" or
	// "label volume v1 delete=<nil>".
	Calls   []string
	pending map[string]int
}

// New returns an empty store.
func New() *Store {
	return &Store{
		Containers:  map[string]stackit.Container{},
		Folder:      map[string]bool{},
		Costs:       map[string]float64{},
		SKE:         map[string][]string{},
		BucketNames: map[string][]string{},
		LB:          map[string]map[string]string{},
		Errs:        map[string]error{},
		ErrsOnce:    map[string]error{},
		pending:     map[string]int{},
	}
}

// Set returns a client set backed by the store.
func (s *Store) Set() *stackit.Set {
	return &stackit.Set{ResourceManager: s, IaaS: s, Cost: s, Services: s}
}

// AddFolder adds a folder under parent (an org or folder ID).
func (s *Store) AddFolder(id, name, parent string, labels map[string]string) {
	s.Containers[id] = stackit.Container{ID: id, Name: name, ParentID: parent, Labels: labels, CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	s.Folder[id] = true
}

// AddProject adds an active project under parent.
func (s *Store) AddProject(id, name, parent string, created time.Time, labels map[string]string) {
	s.Containers[id] = stackit.Container{ID: id, Name: name, ParentID: parent, Labels: labels, CreatedAt: created, LifecycleState: "ACTIVE"}
}

// Add adds resources.
func (s *Store) Add(rs ...stackit.Resource) {
	for i := range rs {
		r := rs[i]
		s.Resources = append(s.Resources, &r)
	}
}

// Find returns the resource with the ID, or nil.
func (s *Store) Find(id string) *stackit.Resource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.find(id)
}

func (s *Store) find(id string) *stackit.Resource {
	for _, r := range s.Resources {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// CallsWith returns the recorded calls that start with prefix.
func (s *Store) CallsWith(prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, c := range s.Calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) record(format string, args ...any) {
	s.Calls = append(s.Calls, fmt.Sprintf(format, args...))
}

func (s *Store) err(key string) error {
	if err, ok := s.ErrsOnce[key]; ok {
		delete(s.ErrsOnce, key)
		return err
	}
	return s.Errs[key]
}

// ---- ResourceManager ----

func (s *Store) children(parent string, folders bool) []stackit.Container {
	var out []stackit.Container
	for id, c := range s.Containers {
		if s.below(c, parent) && s.Folder[id] == folders {
			c.Ancestors = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// below reports whether c is a direct child of parent or, with
// ListDescendants, anywhere below it.
func (s *Store) below(c stackit.Container, parent string) bool {
	if c.ParentID == parent {
		return true
	}
	if !s.ListDescendants {
		return false
	}
	for p := c.ParentID; s.Folder[p]; p = s.Containers[p].ParentID {
		if s.Containers[p].ParentID == parent {
			return true
		}
	}
	return false
}

// ListProjects implements stackit.ResourceManager.
func (s *Store) ListProjects(_ context.Context, parent string) ([]stackit.Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("list projects %s", parent)
	if err := s.err("projects:" + parent); err != nil {
		return nil, err
	}
	return s.children(parent, false), nil
}

// ListFolders implements stackit.ResourceManager.
func (s *Store) ListFolders(_ context.Context, parent string) ([]stackit.Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("list folders %s", parent)
	if err := s.err("folders:" + parent); err != nil {
		return nil, err
	}
	return s.children(parent, true), nil
}

func (s *Store) getContainer(id string, folder bool) (*stackit.Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("get container %s", id)
	if err := s.err("get:" + id); err != nil {
		return nil, err
	}
	c, ok := s.Containers[id]
	if !ok || s.Folder[id] != folder {
		return nil, Status(404)
	}
	c.Ancestors = nil
	for p := c.ParentID; s.Folder[p]; p = s.Containers[p].ParentID {
		c.Ancestors = append(c.Ancestors, stackit.Ancestor{ID: p, Name: s.Containers[p].Name})
	}
	return &c, nil
}

// GetProject implements stackit.ResourceManager.
func (s *Store) GetProject(_ context.Context, id string) (*stackit.Container, error) {
	return s.getContainer(id, false)
}

// GetFolder implements stackit.ResourceManager.
func (s *Store) GetFolder(_ context.Context, id string) (*stackit.Container, error) {
	return s.getContainer(id, true)
}

// ---- IaaS ----

// List implements stackit.IaaS.
func (s *Store) List(_ context.Context, kind stackit.Kind, project, region string) ([]stackit.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.err(fmt.Sprintf("list:%s:%s/%s", kind, project, region)); err != nil {
		return nil, err
	}
	var out []stackit.Resource
	for _, r := range s.Resources {
		if r.Kind == kind && r.ProjectID == project && r.Region == region {
			out = append(out, clone(r))
		}
	}
	return out, nil
}

// Get implements stackit.IaaS.
func (s *Store) Get(_ context.Context, ref stackit.Resource) (*stackit.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("get %s %s", ref.Kind, ref.ID)
	if err := s.err("get:" + ref.ID); err != nil {
		return nil, err
	}
	if n, ok := s.pending[ref.ID]; ok {
		if n > 0 {
			s.pending[ref.ID] = n - 1
		} else {
			delete(s.pending, ref.ID)
			s.release(ref.ID)
			s.remove(ref.ID)
		}
	}
	r := s.find(ref.ID)
	if r == nil || r.Kind != ref.Kind {
		return nil, Status(404)
	}
	c := clone(r)
	return &c, nil
}

// Delete implements stackit.IaaS. Deleting a server releases what was
// attached to it.
func (s *Store) Delete(_ context.Context, ref stackit.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("delete %s %s", ref.Kind, ref.ID)
	if err := s.err("delete:" + ref.ID); err != nil {
		return err
	}
	r := s.find(ref.ID)
	if r == nil || r.Kind != ref.Kind {
		return Status(404)
	}
	if r.Kind == stackit.KindServer {
		if s.DeletingReads > 0 {
			// Still there, DELETING, for the next reads; attachments are
			// released when it is gone.
			r.Status = stackit.ServerStatusDeleting
			s.pending[r.ID] = s.DeletingReads
			return nil
		}
		s.release(r.ID)
	}
	s.remove(r.ID)
	if s.AfterDelete != nil {
		s.AfterDelete(ref)
	}
	return nil
}

// release detaches a server's volumes and deletes its NICs, which frees
// the public IPs on them.
func (s *Store) release(server string) {
	var nics []string
	for _, o := range s.Resources {
		if o.Kind == stackit.KindVolume && o.ServerID == server {
			o.ServerID, o.Status = "", stackit.VolumeStatusAvailable
		}
		if o.Kind == stackit.KindNIC && o.ServerID == server {
			nics = append(nics, o.ID)
			for _, ip := range s.Resources {
				if ip.Kind == stackit.KindPublicIP && ip.NICID == o.ID {
					ip.NICID = ""
				}
			}
		}
	}
	for _, id := range nics {
		s.remove(id)
	}
}

func (s *Store) remove(id string) {
	for i, r := range s.Resources {
		if r.ID == id {
			s.Resources = append(s.Resources[:i], s.Resources[i+1:]...)
			return
		}
	}
}

// SetLabel implements stackit.IaaS.
func (s *Store) SetLabel(_ context.Context, ref stackit.Resource, key string, value *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := "<nil>"
	if value != nil {
		v = *value
	}
	s.record("label %s %s %s=%s", ref.Kind, ref.ID, key, v)
	if err := s.err("label:" + ref.ID); err != nil {
		return err
	}
	if ref.Kind != stackit.KindVolume && ref.Kind != stackit.KindPublicIP {
		return fmt.Errorf("setting labels on %s is not supported", ref.Kind)
	}
	r := s.find(ref.ID)
	if r == nil {
		return Status(404)
	}
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	if value == nil {
		delete(r.Labels, key)
	} else {
		r.Labels[key] = *value
	}
	return nil
}

// ListNetworkAreas implements stackit.IaaS.
func (s *Store) ListNetworkAreas(context.Context, string) ([]stackit.NetworkArea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.err("areas"); err != nil {
		return nil, err
	}
	return append([]stackit.NetworkArea(nil), s.Areas...), nil
}

// ---- Cost and services ----

// ProjectCosts implements stackit.Cost.
func (s *Store) ProjectCosts(_ context.Context, _ string, from, to time.Time) (map[string]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("costs %s %s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err := s.err("costs"); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for k, v := range s.Costs {
		out[k] = v
	}
	return out, nil
}

// DailyCosts implements stackit.Cost. Like the Cost API it returns only
// the days within the range.
func (s *Store) DailyCosts(_ context.Context, _ string, from, to time.Time) (*stackit.DailyCosts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	first, last := from.Format("2006-01-02"), to.Format("2006-01-02")
	s.record("daily costs %s %s", first, last)
	if err := s.err("daily"); err != nil {
		return nil, err
	}
	out := &stackit.DailyCosts{Projects: map[string]stackit.ProjectDays{}, LastModified: s.LastModified}
	for id, p := range s.Daily {
		days := stackit.ProjectDays{Name: p.Name, EUR: map[string]float64{}}
		for day, v := range p.EUR {
			if day >= first && day <= last {
				days.EUR[day] = v
			}
		}
		if len(days.EUR) > 0 {
			out.Projects[id] = days
		}
	}
	return out, nil
}

// SKEClusters implements stackit.Services.
func (s *Store) SKEClusters(_ context.Context, project, region string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := project + "/" + region
	s.record("ske %s", key)
	if err := s.err("ske:" + key); err != nil {
		return nil, err
	}
	return s.SKE[key], nil
}

// Buckets implements stackit.Services.
func (s *Store) Buckets(_ context.Context, project, region string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := project + "/" + region
	s.record("buckets %s", key)
	if err := s.err("buckets:" + key); err != nil {
		return nil, err
	}
	return s.BucketNames[key], nil
}

// LoadBalancerAddresses implements stackit.Services.
func (s *Store) LoadBalancerAddresses(_ context.Context, project, region string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := project + "/" + region
	s.record("lb %s", key)
	if err := s.err("lb:" + key); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range s.LB[key] {
		out[k] = v
	}
	return out, nil
}

func clone(r *stackit.Resource) stackit.Resource {
	c := *r
	if r.Labels != nil {
		c.Labels = make(map[string]string, len(r.Labels))
		for k, v := range r.Labels {
			c.Labels[k] = v
		}
	}
	return c
}
