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

package scanner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/config"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/stackit"
	"github.com/stackitcloud/professional-service/apps/costguard/internal/whitelist"
)

const (
	testOrgID    = "22222222-2222-2222-2222-222222222222"
	testFolderA  = "33333333-3333-3333-3333-333333333333"
	testFolderB  = "44444444-4444-4444-4444-444444444444"
	testProjA    = "55555555-5555-5555-5555-555555555555"
	testProjB    = "66666666-6666-6666-6666-666666666666"
	testProjC    = "77777777-7777-7777-7777-777777777777"
	testProjD    = "88888888-8888-8888-8888-888888888888"
	testProjW    = "99999999-9999-9999-9999-999999999999"
	testIPNew    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testIPMark   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	testIPAttach = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	testIPWhite  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	testVolNew   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	testVolBusy  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	testVolWhite = "11111111-1111-1111-1111-111111111111"
)

var testNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// fakeResourceManager is a data-driven ResourceManager fake.
type fakeResourceManager struct {
	mu              sync.Mutex
	projects        map[string][]stackit.Project
	folders         map[string][]stackit.Folder
	folderDetails   map[string]*stackit.Folder
	errListProjects error
	errListFolders  error
}

func newFakeResourceManager() *fakeResourceManager {
	return &fakeResourceManager{
		projects:      map[string][]stackit.Project{},
		folders:       map[string][]stackit.Folder{},
		folderDetails: map[string]*stackit.Folder{},
	}
}

func (f *fakeResourceManager) ListProjects(_ context.Context, containerID string) ([]stackit.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListProjects != nil {
		return nil, f.errListProjects
	}
	return append([]stackit.Project{}, f.projects[containerID]...), nil
}

func (f *fakeResourceManager) ListFolders(_ context.Context, containerID string) ([]stackit.Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListFolders != nil {
		return nil, f.errListFolders
	}
	return append([]stackit.Folder{}, f.folders[containerID]...), nil
}

func (f *fakeResourceManager) GetProject(_ context.Context, projectID string) (*stackit.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, list := range f.projects {
		for _, p := range list {
			if p.ID == projectID {
				p := p
				return &p, nil
			}
		}
	}
	return nil, errors.New("project not found")
}

func (f *fakeResourceManager) GetFolder(_ context.Context, folderID string) (*stackit.Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.folderDetails[folderID] != nil {
		out := *f.folderDetails[folderID]
		return &out, nil
	}
	return nil, errors.New("folder not found")
}

// fakeIaaS is a data-driven IaaS fake. Mark operations are recorded in
// setMarks/clearedMarks so tests can assert on the mark lifecycle.
type fakeIaaS struct {
	mu             sync.Mutex
	ips            map[string][]stackit.PublicIP
	volumes        map[string][]stackit.Volume
	servers        map[string][]stackit.Server
	snapshots      map[string][]stackit.Snapshot
	areas          []stackit.NetworkArea
	setMarks       map[string]string
	clearedMarks   map[string]bool
	deletedIPs     []string
	deletedVolumes []string
	markErr        error
	errListIPs     map[string]error
	errListVolumes map[string]error
}

func newFakeIaaS() *fakeIaaS {
	return &fakeIaaS{
		ips:            map[string][]stackit.PublicIP{},
		volumes:        map[string][]stackit.Volume{},
		servers:        map[string][]stackit.Server{},
		snapshots:      map[string][]stackit.Snapshot{},
		setMarks:       map[string]string{},
		clearedMarks:   map[string]bool{},
		errListIPs:     map[string]error{},
		errListVolumes: map[string]error{},
	}
}

func prKey(project, region string) string      { return project + "/" + region }
func ipKey(project, region, id string) string  { return "ip/" + prKey(project, region) + "/" + id }
func volKey(project, region, id string) string { return "vol/" + prKey(project, region) + "/" + id }

func (f *fakeIaaS) ListNetworkAreas(_ context.Context, _ string) ([]stackit.NetworkArea, error) {
	return f.areas, nil
}

func (f *fakeIaaS) GetNetworkArea(_ context.Context, _, areaID string) (*stackit.NetworkArea, error) {
	for _, a := range f.areas {
		if a.ID == areaID {
			a := a
			return &a, nil
		}
	}
	return nil, errors.New("area not found")
}

func (f *fakeIaaS) ListPublicIPs(_ context.Context, projectID, region string) ([]stackit.PublicIP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListIPs[prKey(projectID, region)] != nil {
		return nil, f.errListIPs[prKey(projectID, region)]
	}
	return append([]stackit.PublicIP{}, f.ips[prKey(projectID, region)]...), nil
}

func (f *fakeIaaS) GetPublicIP(_ context.Context, projectID, region, id string) (*stackit.PublicIP, error) {
	for _, ip := range f.ips[prKey(projectID, region)] {
		if ip.ID == id {
			ip := ip
			return &ip, nil
		}
	}
	return nil, errors.New("ip not found")
}

func (f *fakeIaaS) SetPublicIPMark(_ context.Context, projectID, region, id, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.setMarks[ipKey(projectID, region, id)] = value
	return nil
}

func (f *fakeIaaS) ClearPublicIPMark(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearedMarks[ipKey(projectID, region, id)] = true
	return nil
}

func (f *fakeIaaS) DeletePublicIP(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedIPs = append(f.deletedIPs, ipKey(projectID, region, id))
	return nil
}

func (f *fakeIaaS) ListVolumes(_ context.Context, projectID, region string) ([]stackit.Volume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListVolumes[prKey(projectID, region)] != nil {
		return nil, f.errListVolumes[prKey(projectID, region)]
	}
	return append([]stackit.Volume{}, f.volumes[prKey(projectID, region)]...), nil
}

func (f *fakeIaaS) GetVolume(_ context.Context, projectID, region, id string) (*stackit.Volume, error) {
	for _, v := range f.volumes[prKey(projectID, region)] {
		if v.ID == id {
			v := v
			return &v, nil
		}
	}
	return nil, errors.New("volume not found")
}

func (f *fakeIaaS) SetVolumeMark(_ context.Context, projectID, region, id, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.setMarks[volKey(projectID, region, id)] = value
	return nil
}

func (f *fakeIaaS) ClearVolumeMark(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearedMarks[volKey(projectID, region, id)] = true
	return nil
}

func (f *fakeIaaS) DeleteVolume(_ context.Context, projectID, region, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedVolumes = append(f.deletedVolumes, volKey(projectID, region, id))
	return nil
}

func (f *fakeIaaS) ListSnapshots(_ context.Context, projectID, region string) ([]stackit.Snapshot, error) {
	return f.snapshots[prKey(projectID, region)], nil
}

func (f *fakeIaaS) DeleteSnapshot(_ context.Context, projectID, region, id string) error {
	return nil
}

func (f *fakeIaaS) ListServers(_ context.Context, projectID, region string) ([]stackit.Server, error) {
	return f.servers[prKey(projectID, region)], nil
}

// fakeCost serves per-day records; keys use the billing date layout.
type fakeCost struct {
	mu       sync.Mutex
	perDay   map[string][]stackit.CostRecord
	errDays  map[string]error
	dayCalls []string
}

func newFakeCost() *fakeCost {
	return &fakeCost{perDay: map[string][]stackit.CostRecord{}, errDays: map[string]error{}}
}

func (f *fakeCost) ListCostsForCustomer(_ context.Context, _ string, from, _ time.Time) ([]stackit.CostRecord, error) {
	key := from.Format("2006-01-02")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dayCalls = append(f.dayCalls, key)
	if f.errDays[key] != nil {
		return nil, f.errDays[key]
	}
	return f.perDay[key], nil
}

type fakeSKE struct {
	mu       sync.Mutex
	clusters map[string][]string
	err      error
}

func newFakeSKE() *fakeSKE {
	return &fakeSKE{clusters: map[string][]string{}}
}

func (f *fakeSKE) ListClusterNames(_ context.Context, projectID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.clusters[projectID], nil
}

type fakeObjectStorage struct {
	mu      sync.Mutex
	buckets map[string][]string
	err     error
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{buckets: map[string][]string{}}
}

func (f *fakeObjectStorage) ListBucketNames(_ context.Context, projectID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.buckets[projectID], nil
}

// testScanner builds a Scanner with fakes and fixed production-ish
// defaults. Callers override fields on cfg as needed.
func testScanner(cfg config.Config, wl *whitelist.Whitelist, rm *fakeResourceManager, iaas *fakeIaaS, cost *fakeCost, ske *fakeSKE, os *fakeObjectStorage) *Scanner {
	if cfg.Scope == "" {
		cfg.Scope = config.ScopeOrganisation
	}
	if cfg.OrgID == "" {
		cfg.OrgID = testOrgID
	}
	if len(cfg.Regions) == 0 {
		cfg.Regions = []string{"eu01"}
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = 90
	}
	if cfg.SNAMaxAgeDays == 0 {
		cfg.SNAMaxAgeDays = 30
	}
	if cfg.SafeLabelKey == "" {
		cfg.SafeLabelKey = "costguard-safe"
	}
	if cfg.SafeLabelValue == "" {
		cfg.SafeLabelValue = "true"
	}
	if cfg.GracePeriod == 0 {
		cfg.GracePeriod = 8 * time.Hour
	}
	if cfg.VolumeCostEurPerGB == 0 {
		cfg.VolumeCostEurPerGB = 0.0619
	}
	if cfg.PublicIPCostEurPerMonth == 0 {
		cfg.PublicIPCostEurPerMonth = 4.82
	}
	if cfg.CostAnomalyThresholdPct == 0 {
		cfg.CostAnomalyThresholdPct = 20
	}
	return &Scanner{
		Clients: &stackit.Set{
			ResourceManager: rm,
			IaaS:            iaas,
			Cost:            cost,
			SKE:             ske,
			ObjectStorage:   os,
		},
		Whitelist: wl,
		Config:    cfg,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Pacer:     NopPacer{},
		Now:       func() time.Time { return testNow },
		Workers:   4,
	}
}

func emptyWhitelist() *whitelist.Whitelist {
	return whitelist.New(config.Whitelist{}, nil, nil)
}

func safeLabels() map[string]string {
	return map[string]string{"costguard-safe": "true"}
}

// TestScanFull covers the org walk (folder protection, invisibility),
// hygiene, candidate marking, and billing in one run.
func TestScanFull(t *testing.T) {
	rm := newFakeResourceManager()
	rm.folders[testOrgID] = []stackit.Folder{
		{ID: testFolderA, Name: "Team A", ParentContainerID: testOrgID},
		{ID: testFolderB, Name: "Protected", ParentContainerID: testOrgID, Labels: safeLabels()},
	}
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjD, Name: "root-old", ParentContainerID: testOrgID, CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE"},
		{ID: testProjW, Name: "whitelisted", ParentContainerID: testOrgID, CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE"},
		{ID: "00000000-0000-0000-0000-000000000000", Name: "inactive", ParentContainerID: testOrgID, CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "INACTIVE"},
	}
	rm.projects[testFolderA] = []stackit.Project{
		{ID: testProjA, Name: "old-with-content", ParentContainerID: testFolderA, CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE"},
		{ID: testProjB, Name: "young", ParentContainerID: testFolderA, CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}
	rm.projects[testFolderB] = []stackit.Project{
		// Inside a safe-labeled folder: invisible, never a candidate.
		{ID: testProjC, Name: "protected-old", ParentContainerID: testFolderB, CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE"},
	}
	wl := whitelist.New(config.Whitelist{Projects: []string{testProjW}, PublicIPs: []string{testIPWhite}, Volumes: []string{testVolWhite}}, nil, nil)

	iaas := newFakeIaaS()
	existingMark := stackit.FormatMark(testNow.Add(time.Hour))
	iaas.ips[prKey(testProjA, "eu01")] = []stackit.PublicIP{
		{ID: testIPNew, Address: "1.2.3.4", ProjectID: testProjA, Region: "eu01"},
		{ID: testIPMark, Address: "1.2.3.5", ProjectID: testProjA, Region: "eu01", Labels: map[string]string{stackit.MarkLabelKey: existingMark}},
		{ID: testIPAttach, Address: "1.2.3.6", ProjectID: testProjA, Region: "eu01", AttachedNIC: "nic-1"},
		{ID: testIPWhite, Address: "1.2.3.7", ProjectID: testProjA, Region: "eu01"},
	}
	iaas.volumes[prKey(testProjA, "eu01")] = []stackit.Volume{
		{ID: testVolNew, Name: "data", ProjectID: testProjA, Region: "eu01", SizeGB: 100, Status: "AVAILABLE"},
		{ID: testVolBusy, Name: "disk", ProjectID: testProjA, Region: "eu01", SizeGB: 200, Status: "IN_USE", ServerID: "srv-1"},
		{ID: testVolWhite, Name: "keep", ProjectID: testProjA, Region: "eu01", SizeGB: 300, Status: "AVAILABLE"},
	}
	iaas.servers[prKey(testProjA, "eu01")] = []stackit.Server{{ID: "srv-1", Name: "web"}}
	iaas.areas = []stackit.NetworkArea{
		{ID: "sna-old", Name: "old-empty", ProjectCount: 0, CreatedAt: testNow.AddDate(0, 0, -40)},
		{ID: "sna-young", Name: "young-empty", ProjectCount: 0, CreatedAt: testNow.AddDate(0, 0, -10)},
		{ID: "sna-busy", Name: "busy", ProjectCount: 3, CreatedAt: testNow.AddDate(0, 0, -40)},
		{ID: "sna-safe", Name: "safe-empty", ProjectCount: 0, CreatedAt: testNow.AddDate(0, 0, -40), Labels: safeLabels()},
	}

	ske := newFakeSKE()
	ske.clusters[testProjA] = []string{"prod", "staging"}
	os := newFakeObjectStorage()
	os.buckets[testProjA] = []string{"logs"}

	cost := newFakeCost()
	dates := dateRange(costWindowDays, testNow)
	for i, d := range dates {
		key := d.Format("2006-01-02")
		charge := 10.0
		if i >= len(dates)-anomalyWindowDays {
			charge = 30.0 // last 7 days: +200% vs baseline -> anomaly
		}
		cost.perDay[key] = []stackit.CostRecord{
			{ProjectID: testProjA, ProjectName: "old-with-content", ChargeEUR: charge},
			{ProjectID: testProjD, ProjectName: "root-old", ChargeEUR: 5.0},
		}
	}

	s := testScanner(config.Config{}, wl, rm, iaas, cost, ske, os)
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(rep.ScanErrors) != 0 {
		t.Fatalf("ScanErrors = %v, want none", rep.ScanErrors)
	}

	// Stale projects: projA (folder A) and projD (root). projC (labeled
	// folder) and projW (whitelisted) are invisible; projB is too young.
	if len(rep.StaleProjects) != 2 {
		t.Fatalf("StaleProjects = %+v, want 2 entries", rep.StaleProjects)
	}
	got := map[string]report.StaleProject{}
	for _, p := range rep.StaleProjects {
		got[p.ID] = p
	}
	if _, ok := got[testProjA]; !ok {
		t.Errorf("missing stale project %s: %+v", testProjA, rep.StaleProjects)
	}
	if p, ok := got[testProjD]; !ok {
		t.Errorf("missing stale project %s", testProjD)
	} else if p.ParentFolderName != "Organization" {
		t.Errorf("parent folder = %q, want Organization", p.ParentFolderName)
	}
	if p, ok := got[testProjA]; ok {
		if p.AgeDays != 100 {
			t.Errorf("ageDays = %d, want 100", p.AgeDays)
		}
		if len(p.SKEClusters) != 2 || p.SKEClusters[0] != "prod" {
			t.Errorf("SKEClusters = %v", p.SKEClusters)
		}
		if len(p.StorageBuckets) != 1 || p.StorageBuckets[0] != "logs" {
			t.Errorf("StorageBuckets = %v", p.StorageBuckets)
		}
		if p.ServerCount != 1 {
			t.Errorf("ServerCount = %d, want 1", p.ServerCount)
		}
		if p.ParentFolderName != "Team A" {
			t.Errorf("parent folder = %q, want Team A", p.ParentFolderName)
		}
	}

	if len(rep.EmptySNAs) != 1 || rep.EmptySNAs[0].ID != "sna-old" {
		t.Errorf("EmptySNAs = %+v, want only sna-old", rep.EmptySNAs)
	}

	// Public IPs: new mark + kept mark; attached and whitelisted absent.
	if len(rep.IdlePublicIPs) != 2 {
		t.Fatalf("IdlePublicIPs = %+v, want 2", rep.IdlePublicIPs)
	}
	byID := map[string]report.IdlePublicIP{}
	for _, ip := range rep.IdlePublicIPs {
		byID[ip.ID] = ip
	}
	if ip := byID[testIPNew]; ip.MarkDeadline == nil || !ip.MarkDeadline.Equal(testNow.Add(8*time.Hour)) {
		t.Errorf("new IP deadline = %v, want %v", ip.MarkDeadline, testNow.Add(8*time.Hour))
	}
	if ip := byID[testIPMark]; ip.MarkDeadline == nil || !ip.MarkDeadline.Equal(testNow.Add(time.Hour)) {
		t.Errorf("kept IP deadline = %v, want %v", ip.MarkDeadline, testNow.Add(time.Hour))
	}
	if _, ok := byID[testIPAttach]; ok {
		t.Errorf("attached IP must not be a candidate")
	}
	if _, ok := byID[testIPWhite]; ok {
		t.Errorf("whitelisted IP must not be a candidate")
	}
	if got := iaas.setMarks[ipKey(testProjA, "eu01", testIPNew)]; got != stackit.FormatMark(testNow.Add(8*time.Hour)) {
		t.Errorf("set mark = %q, want %q", got, stackit.FormatMark(testNow.Add(8*time.Hour)))
	}
	if iaas.setMarks[ipKey(testProjA, "eu01", testIPMark)] != "" {
		t.Errorf("existing mark must not be re-written")
	}

	// Volumes: only the plain detached one.
	if len(rep.DetachedVolumes) != 1 {
		t.Fatalf("DetachedVolumes = %+v, want 1", rep.DetachedVolumes)
	}
	v := rep.DetachedVolumes[0]
	if v.ID != testVolNew || v.SizeGB != 100 || v.EstimatedMonthlyCostEUR != 6.19 {
		t.Errorf("volume = %+v", v)
	}
	if v.MarkDeadline == nil || !v.MarkDeadline.Equal(testNow.Add(8*time.Hour)) {
		t.Errorf("volume deadline = %v", v.MarkDeadline)
	}

	// Billing: per-day total = projA + projD. Baseline days 10+5=15,
	// recent days 30+5=35.
	if rep.TotalCost30dEUR != 590.0 { // 23x15 + 7x35
		t.Errorf("TotalCost30d = %v, want 590.0", rep.TotalCost30dEUR)
	}
	if rep.AvgCostPerDayEUR != 19.67 { // 590/30 = 19.666 -> 19.67
		t.Errorf("AvgPerDay = %v, want 19.67", rep.AvgCostPerDayEUR)
	}
	if !rep.AnomalyDetected {
		t.Errorf("anomaly not detected, want +133%%")
	}
	if rep.AnomalyPct != 133.33 { // (35-15)/15*100
		t.Errorf("anomaly pct = %v, want 133.33", rep.AnomalyPct)
	}
	if len(rep.TopProjects) != 2 || rep.TopProjects[0].ProjectID != testProjA {
		t.Errorf("TopProjects = %+v", rep.TopProjects)
	}
	if rep.TopProjects[0].Cost30dEUR != 440.0 { // 23x10 + 7x30
		t.Errorf("top project cost = %v, want 440.0", rep.TopProjects[0].Cost30dEUR)
	}
	if rep.TopProjects[1].Cost30dEUR != 150.0 { // 30x5
		t.Errorf("second project cost = %v, want 150.0", rep.TopProjects[1].Cost30dEUR)
	}

	// Savings: 6.19 (volume) + 2 x 4.82 (IPs) = 15.83.
	if rep.EstimatedMonthlySavingsEUR != 15.83 {
		t.Errorf("savings = %v, want 15.83", rep.EstimatedMonthlySavingsEUR)
	}

	// The protected project contributed no IaaS calls and no entries.
	for _, k := range iaas.setMarks {
		_ = k
	}
	if _, ok := iaas.ips[prKey(testProjC, "eu01")]; ok {
		t.Errorf("protected project must not be scanned")
	}
}

// TestScanSelfHeal verifies marks are cleared once a resource is no
// longer a candidate.
func TestScanSelfHeal(t *testing.T) {
	rm := newFakeResourceManager()
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjA, Name: "a", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}
	iaas := newFakeIaaS()
	iaas.ips[prKey(testProjA, "eu01")] = []stackit.PublicIP{
		// Marked but now attached: mark must be cleared.
		{ID: testIPAttach, Address: "1.2.3.6", ProjectID: testProjA, Region: "eu01", AttachedNIC: "nic-1", Labels: map[string]string{stackit.MarkLabelKey: stackit.FormatMark(testNow.Add(time.Hour))}},
	}
	iaas.volumes[prKey(testProjA, "eu01")] = []stackit.Volume{
		// Marked but now in use: mark must be cleared.
		{ID: testVolBusy, Name: "disk", ProjectID: testProjA, Region: "eu01", SizeGB: 10, Status: "IN_USE", ServerID: "srv-1", Labels: map[string]string{stackit.MarkLabelKey: stackit.FormatMark(testNow.Add(time.Hour))}},
		// Marked, safe label gained in the meantime: mark must be cleared.
		{ID: testVolNew, Name: "data", ProjectID: testProjA, Region: "eu01", SizeGB: 10, Status: "AVAILABLE", Labels: map[string]string{stackit.MarkLabelKey: stackit.FormatMark(testNow.Add(time.Hour)), "costguard-safe": "true"}},
	}

	s := testScanner(config.Config{}, emptyWhitelist(), rm, iaas, newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(rep.IdlePublicIPs) != 0 || len(rep.DetachedVolumes) != 0 {
		t.Errorf("candidates = %+v / %+v, want none", rep.IdlePublicIPs, rep.DetachedVolumes)
	}
	if !iaas.clearedMarks[ipKey(testProjA, "eu01", testIPAttach)] {
		t.Errorf("attached IP mark not cleared")
	}
	if !iaas.clearedMarks[volKey(testProjA, "eu01", testVolBusy)] {
		t.Errorf("busy volume mark not cleared")
	}
	if !iaas.clearedMarks[volKey(testProjA, "eu01", testVolNew)] {
		t.Errorf("safe-labeled volume mark not cleared")
	}
}

// TestScanMarkErrorSurfaces verifies a failed mark write lands in
// ScanErrors without aborting the run.
func TestScanMarkErrorSurfaces(t *testing.T) {
	rm := newFakeResourceManager()
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjA, Name: "a", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}
	iaas := newFakeIaaS()
	iaas.ips[prKey(testProjA, "eu01")] = []stackit.PublicIP{{ID: testIPNew, ProjectID: testProjA, Region: "eu01"}}
	iaas.markErr = errors.New("boom")

	s := testScanner(config.Config{}, emptyWhitelist(), rm, iaas, newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan must not return error: %v", err)
	}
	if len(rep.IdlePublicIPs) != 1 {
		t.Fatalf("candidate still expected despite mark failure, got %+v", rep.IdlePublicIPs)
	}
	found := false
	for _, e := range rep.ScanErrors {
		if contains(e, "marking public IP") {
			found = true
		}
	}
	if !found {
		t.Errorf("ScanErrors = %v, want mark error", rep.ScanErrors)
	}
}

// TestScanListErrorNonFatal verifies a failing list call is recorded and
// the rest of the run continues.
func TestScanListErrorNonFatal(t *testing.T) {
	rm := newFakeResourceManager()
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjA, Name: "a", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
		{ID: testProjB, Name: "b", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}
	iaas := newFakeIaaS()
	iaas.errListIPs[prKey(testProjA, "eu01")] = errors.New("boom")
	iaas.ips[prKey(testProjB, "eu01")] = []stackit.PublicIP{{ID: testIPNew, ProjectID: testProjB, Region: "eu01"}}

	s := testScanner(config.Config{}, emptyWhitelist(), rm, iaas, newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan must not return error: %v", err)
	}
	if len(rep.IdlePublicIPs) != 1 || rep.IdlePublicIPs[0].ProjectID != testProjB {
		t.Errorf("surviving project still expected: %+v", rep.IdlePublicIPs)
	}
	found := false
	for _, e := range rep.ScanErrors {
		if contains(e, "public IP scan a/eu01") {
			found = true
		}
	}
	if !found {
		t.Errorf("ScanErrors = %v, want list error", rep.ScanErrors)
	}
}

// TestScanWalkFailure verifies a failed org walk degrades to an empty
// scan with the error surfaced.
func TestScanWalkFailure(t *testing.T) {
	rm := newFakeResourceManager()
	rm.errListProjects = errors.New("rm down")
	s := testScanner(config.Config{}, emptyWhitelist(), rm, newFakeIaaS(), newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan must not return error: %v", err)
	}
	found := false
	for _, e := range rep.ScanErrors {
		if contains(e, "org/folder walk") {
			found = true
		}
	}
	if !found {
		t.Errorf("ScanErrors = %v, want walk error", rep.ScanErrors)
	}
}

// TestScanFolderScope verifies the folder scope walks the given folders
// and honors the scope folder's own safe label.
func TestScanFolderScope(t *testing.T) {
	rm := newFakeResourceManager()
	rm.folderDetails[testFolderA] = &stackit.Folder{ID: testFolderA, Name: "Labeled Root", Labels: safeLabels()}
	rm.projects[testFolderA] = []stackit.Project{
		// Old but inside a safe-labeled scope folder: invisible.
		{ID: testProjA, Name: "old", CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE"},
	}
	rm.projects[testFolderB] = []stackit.Project{
		{ID: testProjB, Name: "young", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}

	s := testScanner(config.Config{Scope: config.ScopeFolder, FolderIDs: []string{testFolderA, testFolderB}}, emptyWhitelist(), rm, newFakeIaaS(), newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(rep.StaleProjects) != 0 {
		t.Errorf("StaleProjects = %+v, want none (all protected/young)", rep.StaleProjects)
	}
	if rep.Scope != config.ScopeFolder {
		t.Errorf("Scope = %q", rep.Scope)
	}
}

// TestStaleProjectsBoundary verifies exactly MaxAgeDays is NOT a
// candidate ("exceeds") while one day more is.
func TestStaleProjectsBoundary(t *testing.T) {
	rm := newFakeResourceManager()
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjA, Name: "exact", CreationTime: testNow.AddDate(0, 0, -90), LifecycleState: "ACTIVE"},
		{ID: testProjB, Name: "over", CreationTime: testNow.AddDate(0, 0, -91), LifecycleState: "ACTIVE"},
		{ID: testProjC, Name: "labeled-exact", CreationTime: testNow.AddDate(0, 0, -100), LifecycleState: "ACTIVE", Labels: safeLabels()},
	}
	s := testScanner(config.Config{}, emptyWhitelist(), rm, newFakeIaaS(), newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(rep.StaleProjects) != 1 || rep.StaleProjects[0].ID != testProjB {
		t.Errorf("StaleProjects = %+v, want only over", rep.StaleProjects)
	}
}

// TestSkipScanLists verifies skip-scan projects are not queried.
func TestSkipScanLists(t *testing.T) {
	rm := newFakeResourceManager()
	rm.projects[testOrgID] = []stackit.Project{
		{ID: testProjA, Name: "skip-ips", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
		{ID: testProjB, Name: "skip-vols", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
		{ID: testProjC, Name: "skip-both", CreationTime: testNow.AddDate(0, 0, -10), LifecycleState: "ACTIVE"},
	}
	iaas := newFakeIaaS()
	// If a skip-listed scan is mistakenly invoked, surface it loudly.
	iaas.errListIPs[prKey(testProjA, "eu01")] = errors.New("IP scan of skip project must not run")
	iaas.errListVolumes[prKey(testProjB, "eu01")] = errors.New("volume scan of skip project must not run")
	iaas.errListVolumes[prKey(testProjC, "eu01")] = errors.New("volume scan of skip project must not run")
	iaas.errListIPs[prKey(testProjC, "eu01")] = errors.New("IP scan of skip project must not run")
	wl := whitelist.New(config.Whitelist{
		SkipPublicIPScanProjects: []string{testProjA},
		SkipVolumeScanProjects:   []string{testProjB, testProjC},
	}, nil, nil)
	iaas.ips[prKey(testProjA, "eu01")] = []stackit.PublicIP{{ID: testIPNew, ProjectID: testProjA, Region: "eu01"}}

	s := testScanner(config.Config{}, wl, rm, iaas, newFakeCost(), newFakeSKE(), newFakeObjectStorage())
	rep, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(rep.IdlePublicIPs) != 0 || len(rep.DetachedVolumes) != 0 {
		t.Errorf("skip-scan projects must produce no candidates: %+v / %+v", rep.IdlePublicIPs, rep.DetachedVolumes)
	}
}

// TestDetectAnomaly unit-tests the window comparison.
func TestDetectAnomaly(t *testing.T) {
	cases := []struct {
		name      string
		daily     []float64
		threshold float64
		wantDet   bool
		wantPct   float64
	}{
		{"flat", make([]float64, 30), 20, false, 0},
		{"zero baseline, spend appears", append(make([]float64, 23), 1, 1, 1, 1, 1, 1, 1), 20, false, 0},
		{"below threshold", append(append(make([]float64, 16), 10, 10, 10, 10, 10, 10, 10), 12, 12, 12, 12, 12, 12, 12), 20, false, 20},
		{"above threshold", append(append(make([]float64, 16), 10, 10, 10, 10, 10, 10, 10), 13, 13, 13, 13, 13, 13, 13), 20, true, 30},
		{"too short", []float64{1, 2, 3}, 20, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			det, pct := detectAnomaly(tc.daily, tc.threshold)
			if det != tc.wantDet {
				t.Errorf("detected = %v, want %v", det, tc.wantDet)
			}
			if det && pct != tc.wantPct {
				t.Errorf("pct = %v, want %v", pct, tc.wantPct)
			}
		})
	}
}

// TestDateRange verifies the window is complete days before today.
func TestDateRange(t *testing.T) {
	dates := dateRange(30, testNow)
	if len(dates) != 30 {
		t.Fatalf("len = %d, want 30", len(dates))
	}
	wantFirst := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC) // today-30
	wantLast := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)  // today-1
	if !dates[0].Equal(wantFirst) {
		t.Errorf("first = %v, want %v", dates[0], wantFirst)
	}
	if !dates[29].Equal(wantLast) {
		t.Errorf("last = %v, want %v", dates[29], wantLast)
	}
}

// TestTopProjectsOrdering verifies ranking and the top-10 cap.
func TestTopProjectsOrdering(t *testing.T) {
	totals := map[string]float64{}
	names := map[string]string{}
	for i := 0; i < 15; i++ {
		id := string(rune('a' + i))
		totals[id] = float64(i + 1)
		names[id] = id
	}
	top := topProjects(totals, names, 10)
	if len(top) != 10 {
		t.Fatalf("len = %d, want 10", len(top))
	}
	if top[0].Cost30dEUR != 15 || top[9].Cost30dEUR != 6 {
		t.Errorf("order wrong: %+v", top)
	}
}

// TestFixedPacer verifies the pacing gap and context cancellation.
func TestFixedPacer(t *testing.T) {
	p := &FixedPacer{Interval: 30 * time.Millisecond}
	ctx := context.Background()
	if err := p.Pace(ctx); err != nil {
		t.Fatalf("Pace: %v", err)
	}
	start := time.Now()
	if err := p.Pace(ctx); err != nil {
		t.Fatalf("Pace: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("second pace returned after %v, want >= ~30ms gap", elapsed)
	}

	// Cancellation is tested on a fresh pacer with a long interval so the
	// select is guaranteed to be reached (wait > 0) before ctx.Done fires.
	pc := &FixedPacer{Interval: time.Second}
	if err := pc.Pace(context.Background()); err != nil {
		t.Fatalf("warmup Pace: %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pc.Pace(cancelled); err == nil {
		t.Errorf("Pace(cancelled) = nil, want context error")
	}
}

// TestNopPacer verifies the test pacer never errors or blocks.
func TestNopPacer(t *testing.T) {
	if err := (NopPacer{}).Pace(context.Background()); err != nil {
		t.Errorf("NopPacer = %v, want nil", err)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
