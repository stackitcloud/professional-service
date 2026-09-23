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

package deleter

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	oapierror "github.com/stackitcloud/stackit-sdk-go/core/oapierror"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

const (
	tOrg   = "22222222-2222-2222-2222-222222222222"
	tProj  = "55555555-5555-5555-5555-555555555555"
	tIP    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tIP2   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	tVol   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	tVol2  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	tSnap  = "11111111-1111-1111-1111-111111111111"
	tSnap2 = "22222222-2222-2222-2222-222222222222"
)

var tNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

func dueMark() map[string]string {
	return map[string]string{stackit.MarkLabelKey: stackit.FormatMark(tNow.Add(-time.Hour))}
}

func futureMark() map[string]string {
	return map[string]string{stackit.MarkLabelKey: stackit.FormatMark(tNow.Add(7 * time.Hour))}
}

// fakeIaaS serves current resource state and records deletions. Delete
// error scripts allow testing the retry wrapper.
type fakeIaaS struct {
	mu        sync.Mutex
	ips       map[string]stackit.PublicIP
	volumes   map[string]stackit.Volume
	snapshots []stackit.Snapshot

	getIPErr    map[string]error
	getVolErr   map[string]error
	deleteIP    []error // popped per call; empty tail = last error sticks
	deleteVol   []error
	deleteSnap  []error
	listSnapErr error

	deletedIPs     []string
	deletedVols    []string
	deletedSnaps   []string
	deleteIPCalls  int
	deleteVolCalls int
}

func newFakeIaaS() *fakeIaaS {
	return &fakeIaaS{
		ips:       map[string]stackit.PublicIP{},
		volumes:   map[string]stackit.Volume{},
		getIPErr:  map[string]error{},
		getVolErr: map[string]error{},
	}
}

func (f *fakeIaaS) nextErr(scr *[]error) error {
	if len(*scr) == 0 {
		return nil
	}
	err := (*scr)[0]
	*scr = (*scr)[1:]
	return err
}

func (f *fakeIaaS) ListNetworkAreas(context.Context, string) ([]stackit.NetworkArea, error) {
	return nil, nil
}
func (f *fakeIaaS) GetNetworkArea(context.Context, string, string) (*stackit.NetworkArea, error) {
	return nil, nil
}
func (f *fakeIaaS) ListPublicIPs(context.Context, string, string) ([]stackit.PublicIP, error) {
	return nil, nil
}

func (f *fakeIaaS) GetPublicIP(_ context.Context, _, _, id string) (*stackit.PublicIP, error) {
	if f.getIPErr[id] != nil {
		return nil, f.getIPErr[id]
	}
	ip, ok := f.ips[id]
	if !ok {
		return nil, &oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "not found"}
	}
	return &ip, nil
}

func (f *fakeIaaS) SetPublicIPMark(context.Context, string, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ClearPublicIPMark(context.Context, string, string, string) error {
	return nil
}

func (f *fakeIaaS) DeletePublicIP(_ context.Context, _, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteIPCalls++
	if err := f.nextErr(&f.deleteIP); err != nil {
		return err
	}
	f.deletedIPs = append(f.deletedIPs, id)
	return nil
}

func (f *fakeIaaS) ListVolumes(context.Context, string, string) ([]stackit.Volume, error) {
	return nil, nil
}

func (f *fakeIaaS) GetVolume(_ context.Context, _, _, id string) (*stackit.Volume, error) {
	if f.getVolErr[id] != nil {
		return nil, f.getVolErr[id]
	}
	v, ok := f.volumes[id]
	if !ok {
		return nil, &oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "not found"}
	}
	return &v, nil
}

func (f *fakeIaaS) SetVolumeMark(context.Context, string, string, string, string) error {
	return nil
}
func (f *fakeIaaS) ClearVolumeMark(context.Context, string, string, string) error {
	return nil
}

func (f *fakeIaaS) DeleteVolume(_ context.Context, _, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteVolCalls++
	if err := f.nextErr(&f.deleteVol); err != nil {
		return err
	}
	f.deletedVols = append(f.deletedVols, id)
	return nil
}

func (f *fakeIaaS) ListSnapshots(context.Context, string, string) ([]stackit.Snapshot, error) {
	if f.listSnapErr != nil {
		return nil, f.listSnapErr
	}
	return f.snapshots, nil
}

func (f *fakeIaaS) DeleteSnapshot(_ context.Context, _, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.nextErr(&f.deleteSnap); err != nil {
		return err
	}
	f.deletedSnaps = append(f.deletedSnaps, id)
	return nil
}
func (f *fakeIaaS) ListServers(context.Context, string, string) ([]stackit.Server, error) {
	return nil, nil
}

func testDeleter(t *testing.T, iaas *fakeIaaS, wl *whitelist.Whitelist) *Deleter {
	t.Helper()
	cfg := config.Config{
		SafeLabelKey:   "costguard-safe",
		SafeLabelValue: "true",
		GracePeriod:    8 * time.Hour,
	}
	return &Deleter{
		IaaS:             iaas,
		Whitelist:        wl,
		Config:           cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		BetweenDeletions: 0,
		RetryBackoff:     time.Millisecond,
		MaxAttempts:      3,
		Now:              func() time.Time { return tNow },
	}
}

func emptyWL() *whitelist.Whitelist {
	return whitelist.New(config.Whitelist{}, nil, nil)
}

func dueIP(id, addr string) report.IdlePublicIP {
	return report.IdlePublicIP{ID: id, Address: addr, ProjectID: tProj, ProjectName: "proj", Region: "eu01", MarkDeadline: ptr(tNow.Add(-time.Hour))}
}

func dueVol(id, name string) report.DetachedVolume {
	return report.DetachedVolume{ID: id, Name: name, ProjectID: tProj, ProjectName: "proj", Region: "eu01", SizeGB: 10, MarkDeadline: ptr(tNow.Add(-time.Hour))}
}

func ptr[T any](v T) *T { return &v }

func TestDeleteAllHappyPathWithSnapshots(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", SizeGB: 10, Status: "AVAILABLE", Labels: dueMark()}
	// One snapshot of the target volume and one of another volume: only
	// the target's snapshot may be deleted.
	iaas.snapshots = []stackit.Snapshot{
		{ID: tSnap, VolumeID: tVol},
		{ID: tSnap2, VolumeID: "other-vol"},
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{
		IdlePublicIPs:   []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")},
		DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")},
	}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, skipped := summary.Counts()
	if deleted != 2 || failed != 0 || skipped != 0 {
		t.Fatalf("counts = (%d,%d,%d), want (2,0,0); results: %+v", deleted, failed, skipped, summary.Results)
	}
	// Order: public IP first, then volume.
	if summary.Results[0].Item.Kind != "publicip" || summary.Results[1].Item.Kind != "volume" {
		t.Errorf("order = %v, %v; want publicip then volume", summary.Results[0].Item.Kind, summary.Results[1].Item.Kind)
	}
	if len(iaas.deletedIPs) != 1 || iaas.deletedIPs[0] != tIP {
		t.Errorf("deletedIPs = %v", iaas.deletedIPs)
	}
	if len(iaas.deletedVols) != 1 || iaas.deletedVols[0] != tVol {
		t.Errorf("deletedVols = %v", iaas.deletedVols)
	}
	if len(iaas.deletedSnaps) != 1 || iaas.deletedSnaps[0] != tSnap {
		t.Errorf("deletedSnaps = %v, want only the volume's own snapshot", iaas.deletedSnaps)
	}
}

func TestDeleteSkipsNotDue(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: futureMark()}
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: futureMark()}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{
		IdlePublicIPs:   []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")},
		DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")},
	}

	summary := d.DeleteAll(context.Background(), rep)
	_, failed, skipped := summary.Counts()
	if skipped != 2 || failed != 0 {
		t.Errorf("counts: failed=%d skipped=%d, want 0/2; results %+v", failed, skipped, summary.Results)
	}
	if len(iaas.deletedIPs) != 0 || len(iaas.deletedVols) != 0 {
		t.Error("nothing may be deleted when the deadline has not passed")
	}
	if summary.Results[0].Status != report.StatusSkipped {
		t.Errorf("status = %s, want skipped", summary.Results[0].Status)
	}
}

func TestDeleteSkipsWhitelistedAfterWarning(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: dueMark()}
	wl := whitelist.New(config.Whitelist{PublicIPs: []string{tIP}, Volumes: []string{tVol}}, nil, nil)
	d := testDeleter(t, iaas, wl)
	rep := &report.Report{
		IdlePublicIPs:   []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")},
		DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")},
	}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, skipped := summary.Counts()
	if deleted != 0 || failed != 0 || skipped != 2 {
		t.Errorf("counts = (%d,%d,%d), want (0,0,2)", deleted, failed, skipped)
	}
	for _, r := range summary.Results {
		if r.Reason != "whitelisted" {
			t.Errorf("reason = %q, want whitelisted", r.Reason)
		}
	}
}

func TestDeleteSkipsWhenNoLongerCandidate(t *testing.T) {
	iaas := newFakeIaaS()
	// IP got attached since the scan.
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", AttachedNIC: "nic-1", Labels: dueMark()}
	// Volume gained the safe label since the scan.
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: map[string]string{
		stackit.MarkLabelKey: stackit.FormatMark(tNow.Add(-time.Hour)),
		"costguard-safe":     "true",
	}}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{
		IdlePublicIPs:   []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")},
		DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")},
	}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, skipped := summary.Counts()
	if deleted != 0 || failed != 0 || skipped != 2 {
		t.Errorf("counts = (%d,%d,%d), want (0,0,2)", deleted, failed, skipped)
	}
	if summary.Results[0].Reason != "attached to a network interface" {
		t.Errorf("IP reason = %q", summary.Results[0].Reason)
	}
	if summary.Results[1].Reason != "safe label present" {
		t.Errorf("volume reason = %q", summary.Results[1].Reason)
	}
}

func TestDeleteFailClosedWithoutMark(t *testing.T) {
	iaas := newFakeIaaS()
	// Detached and idle but the mark is gone — fail-closed: skip.
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE"}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, skipped := summary.Counts()
	if deleted != 0 || failed != 0 || skipped != 1 {
		t.Errorf("counts = (%d,%d,%d), want (0,0,1)", deleted, failed, skipped)
	}
	if summary.Results[0].Reason != "mark label missing or malformed" {
		t.Errorf("reason = %q", summary.Results[0].Reason)
	}
}

func TestDeleteAlreadyGoneIsSuccess(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.getIPErr[tIP] = &oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "not found"}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{IdlePublicIPs: []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 1 || failed != 0 {
		t.Errorf("counts = (%d,%d), want (1,0)", deleted, failed)
	}
	if summary.Results[0].Status != report.StatusDeleted {
		t.Errorf("status = %s, want deleted (already gone)", summary.Results[0].Status)
	}
}

func TestDeleteRetriesTransientThenSucceeds(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	// 429, then 500, then success.
	iaas.deleteIP = []error{
		&oapierror.GenericOpenAPIError{StatusCode: 429, ErrorMessage: "rate limited"},
		&oapierror.GenericOpenAPIError{StatusCode: 500, ErrorMessage: "boom"},
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{IdlePublicIPs: []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 1 || failed != 0 {
		t.Fatalf("counts = (%d,%d), want (1,0): %+v", deleted, failed, summary.Results)
	}
	if iaas.deleteIPCalls != 3 {
		t.Errorf("deleteIPCalls = %d, want 3 (2 retries)", iaas.deleteIPCalls)
	}
}

func TestDeleteRetriesExhausted(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.deleteIP = []error{
		&oapierror.GenericOpenAPIError{StatusCode: 503, ErrorMessage: "unavailable"},
		&oapierror.GenericOpenAPIError{StatusCode: 503, ErrorMessage: "unavailable"},
		&oapierror.GenericOpenAPIError{StatusCode: 503, ErrorMessage: "unavailable"},
		&oapierror.GenericOpenAPIError{StatusCode: 503, ErrorMessage: "unavailable"}, // never reached
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{IdlePublicIPs: []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 0 || failed != 1 {
		t.Fatalf("counts = (%d,%d), want (0,1)", deleted, failed)
	}
	if iaas.deleteIPCalls != 3 {
		t.Errorf("deleteIPCalls = %d, want exactly 3 attempts", iaas.deleteIPCalls)
	}
}

func TestDeleteNonTransientFailsFast(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.deleteIP = []error{
		&oapierror.GenericOpenAPIError{StatusCode: 400, ErrorMessage: "bad request"},
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{IdlePublicIPs: []report.IdlePublicIP{dueIP(tIP, "1.2.3.4")}}

	summary := d.DeleteAll(context.Background(), rep)
	_, failed, _ := summary.Counts()
	if failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
	if iaas.deleteIPCalls != 1 {
		t.Errorf("deleteIPCalls = %d, want 1 (no retry on 400)", iaas.deleteIPCalls)
	}
}

func TestDelete404OnDeleteIsSuccess(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: dueMark()}
	iaas.deleteVol = []error{&oapierror.GenericOpenAPIError{StatusCode: 404, ErrorMessage: "gone"}}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 1 || failed != 0 {
		t.Errorf("counts = (%d,%d), want (1,0): 404-on-delete is success", deleted, failed)
	}
}

func TestSnapshotFailureKeepsVolume(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: dueMark()}
	iaas.snapshots = []stackit.Snapshot{{ID: tSnap, VolumeID: tVol}}
	iaas.deleteSnap = []error{
		&oapierror.GenericOpenAPIError{StatusCode: 400, ErrorMessage: "snapshot locked"},
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")}}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 0 || failed != 1 {
		t.Fatalf("counts = (%d,%d), want (0,1)", deleted, failed)
	}
	if iaas.deleteVolCalls != 0 {
		t.Errorf("DeleteVolume called %d times, must not run when snapshot deletion fails", iaas.deleteVolCalls)
	}
}

func TestListSnapshotFailureFailsVolume(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: dueMark()}
	iaas.listSnapErr = &oapierror.GenericOpenAPIError{StatusCode: 500, ErrorMessage: "boom"}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")}}

	summary := d.DeleteAll(context.Background(), rep)
	_, failed, _ := summary.Counts()
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	if iaas.deleteVolCalls != 0 {
		t.Error("DeleteVolume must not run when snapshot listing fails")
	}
}

func TestPerItemIsolation(t *testing.T) {
	iaas := newFakeIaaS()
	iaas.ips[tIP] = stackit.PublicIP{ID: tIP, Address: "1.2.3.4", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.ips[tIP2] = stackit.PublicIP{ID: tIP2, Address: "1.2.3.5", ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	iaas.deleteIP = []error{
		&oapierror.GenericOpenAPIError{StatusCode: 400, ErrorMessage: "first fails"},
	} // only the first call fails; the second succeeds
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{
		IdlePublicIPs: []report.IdlePublicIP{dueIP(tIP, "1.2.3.4"), dueIP(tIP2, "1.2.3.5")},
	}

	summary := d.DeleteAll(context.Background(), rep)
	deleted, failed, _ := summary.Counts()
	if deleted != 1 || failed != 1 {
		t.Fatalf("counts = (%d,%d), want (1,1): one failure must not abort the rest", deleted, failed)
	}
	if len(iaas.deletedIPs) != 1 || iaas.deletedIPs[0] != tIP2 {
		t.Errorf("deletedIPs = %v, want the surviving IP", iaas.deletedIPs)
	}
}

func TestContextCancellationStopsRun(t *testing.T) {
	iaas := newFakeIaaS()
	for i, id := range []string{tIP, tIP2} {
		iaas.ips[id] = stackit.PublicIP{ID: id, Address: "1.2.3." + string(rune('4'+i)), ProjectID: tProj, Region: "eu01", Labels: dueMark()}
	}
	d := testDeleter(t, iaas, emptyWL())
	rep := &report.Report{
		IdlePublicIPs:   []report.IdlePublicIP{dueIP(tIP, "1.2.3.4"), dueIP(tIP2, "1.2.3.5")},
		DetachedVolumes: []report.DetachedVolume{dueVol(tVol, "data")},
	}
	iaas.volumes[tVol] = stackit.Volume{ID: tVol, Name: "data", ProjectID: tProj, Region: "eu01", Status: "AVAILABLE", Labels: dueMark()}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately: nothing should be deleted.
	cancel()
	summary := d.DeleteAll(ctx, rep)
	if len(summary.Results) != 0 {
		t.Errorf("results = %+v, want none after immediate cancel", summary.Results)
	}
	if len(iaas.deletedIPs) != 0 || len(iaas.deletedVols) != 0 {
		t.Error("nothing may be deleted after context cancellation")
	}
}

func TestDelayHonorsContext(t *testing.T) {
	d := &Deleter{
		BetweenDeletions: time.Hour,
		Now:              func() time.Time { return tNow },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	d.delay(ctx)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("delay blocked %v despite cancelled context", elapsed)
	}
}

func TestIsTransientAndNotFound(t *testing.T) {
	cases := []struct {
		code  int
		trans bool
		nf    bool
	}{
		{404, false, true},
		{400, false, false},
		{409, true, false},
		{429, true, false},
		{500, true, false},
		{503, true, false},
		{200, false, false},
	}
	for _, c := range cases {
		err := &oapierror.GenericOpenAPIError{StatusCode: c.code, ErrorMessage: "x"}
		if got := isTransient(err); got != c.trans {
			t.Errorf("isTransient(%d) = %v, want %v", c.code, got, c.trans)
		}
		if got := isNotFound(err); got != c.nf {
			t.Errorf("isNotFound(%d) = %v, want %v", c.code, got, c.nf)
		}
	}
	if isTransient(context.DeadlineExceeded) || isNotFound(context.Canceled) {
		t.Error("non-API errors must not be transient/not-found")
	}
}
