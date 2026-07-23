/*
Copyright Confidential Containers Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	v1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/peerpod-ctrl/api/v1alpha1"
)

type mockProvider struct {
	deletedIDs []string
	deleteErr  error
}

func (m *mockProvider) CreateInstance(_ context.Context, _, _ string, _ cloudinit.CloudConfigGenerator, _ provider.InstanceTypeSpec) (*provider.Instance, error) {
	return nil, nil
}

func (m *mockProvider) DeleteInstance(_ context.Context, instanceID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deletedIDs = append(m.deletedIDs, instanceID)
	return nil
}

func (m *mockProvider) Teardown() error       { return nil }
func (m *mockProvider) ConfigVerifier() error { return nil }

type mockLister struct {
	instances []*provider.Instance
	err       error
}

func (m *mockLister) ListInstances(_ context.Context, _ provider.ListInstancesInput) ([]*provider.Instance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	return s
}

func newPeerPod(name, cloudProvider, instanceID string) *v1alpha1.PeerPod {
	return &v1alpha1.PeerPod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1alpha1.PeerPodSpec{
			CloudProvider: cloudProvider,
			InstanceID:    instanceID,
		},
	}
}

func TestCollect_InstanceWithMatchingPeerPod_NotDeleted(t *testing.T) {
	scheme := newScheme()
	pp := newPeerPod("pod-1", "aws", "i-aaa")
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pp).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-aaa", Name: "vm-1"}},
	}

	gc := &GarbageCollector{KubeClient: k8sClient, Namespace: "test-ns"}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 0 {
		t.Errorf("expected no deletions, got %v", mp.deletedIDs)
	}
}

func TestCollect_OrphanWithinGracePeriod_NotDeleted(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-orphan", Name: "orphan-vm"}},
	}

	gc := &GarbageCollector{KubeClient: k8sClient, Namespace: "test-ns"}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 0 {
		t.Errorf("expected no deletions during grace period, got %v", mp.deletedIDs)
	}
	if _, ok := gc.firstSeen["i-orphan"]; !ok {
		t.Error("expected orphan to be tracked in firstSeen")
	}
}

func TestCollect_OrphanPastGracePeriod_Deleted(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-orphan", Name: "orphan-vm"}},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen:  map[string]time.Time{"i-orphan": time.Now().Add(-15 * time.Minute)},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 1 || mp.deletedIDs[0] != "i-orphan" {
		t.Errorf("expected [i-orphan] to be deleted, got %v", mp.deletedIDs)
	}
	if _, ok := gc.firstSeen["i-orphan"]; ok {
		t.Error("expected orphan to be removed from firstSeen after deletion")
	}
}

func TestCollect_DisappearedInstance_RemovedFromFirstSeen(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen:  map[string]time.Time{"i-gone": time.Now().Add(-20 * time.Minute)},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if gc.firstSeen != nil {
		t.Errorf("expected firstSeen to be nil when no cloud instances exist, got %v", gc.firstSeen)
	}
}

func TestCollect_StaleFirstSeenEntry_CleanedUp(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-current", Name: "current-vm"}},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen: map[string]time.Time{
			"i-gone":    time.Now().Add(-20 * time.Minute),
			"i-current": time.Now(),
		},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if _, ok := gc.firstSeen["i-gone"]; ok {
		t.Error("expected stale firstSeen entry for i-gone to be cleaned up")
	}
	if _, ok := gc.firstSeen["i-current"]; !ok {
		t.Error("expected firstSeen entry for i-current to be retained")
	}
}

func TestCollect_ListInstancesError_SkipsCycle(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{err: fmt.Errorf("API error")}

	gc := &GarbageCollector{KubeClient: k8sClient, Namespace: "test-ns"}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 0 {
		t.Errorf("expected no deletions on ListInstances error, got %v", mp.deletedIDs)
	}
}

func TestCollect_DeleteInstanceError_ContinuesProcessing(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	failOnce := &mockProvider{deleteErr: fmt.Errorf("delete failed")}
	lister := &mockLister{
		instances: []*provider.Instance{
			{ID: "i-fail", Name: "fail-vm"},
			{ID: "i-ok", Name: "ok-vm"},
		},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen: map[string]time.Time{
			"i-fail": time.Now().Add(-15 * time.Minute),
			"i-ok":   time.Now().Add(-15 * time.Minute),
		},
	}
	gc.collect(context.Background(), failOnce, lister, "aws", "uid-123", 10*time.Minute)

	if _, ok := gc.firstSeen["i-fail"]; !ok {
		t.Error("expected i-fail to remain in firstSeen after delete error")
	}
	if _, ok := gc.firstSeen["i-ok"]; !ok {
		t.Error("expected i-ok to remain in firstSeen after delete error (deleteErr applies to all)")
	}
}

func TestCollect_MatchedInstanceClearsFirstSeen(t *testing.T) {
	scheme := newScheme()
	pp := newPeerPod("pod-1", "aws", "i-matched")
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pp).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-matched", Name: "matched-vm"}},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen:  map[string]time.Time{"i-matched": time.Now().Add(-5 * time.Minute)},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if _, ok := gc.firstSeen["i-matched"]; ok {
		t.Error("expected firstSeen to be cleared for matched instance")
	}
}

func TestCollect_DifferentCloudProvider_NotMatched(t *testing.T) {
	scheme := newScheme()
	pp := newPeerPod("pod-1", "azure", "i-azure-vm")
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pp).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-azure-vm", Name: "azure-vm"}},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen:  map[string]time.Time{"i-azure-vm": time.Now().Add(-15 * time.Minute)},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 1 || mp.deletedIDs[0] != "i-azure-vm" {
		t.Errorf("expected instance with different cloud provider PeerPod to be treated as orphan, got %v", mp.deletedIDs)
	}
}

func TestCollect_MultipleOrphans_MixedGracePeriod(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{
			{ID: "i-old", Name: "old-orphan"},
			{ID: "i-new", Name: "new-orphan"},
		},
	}

	gc := &GarbageCollector{
		KubeClient: k8sClient,
		Namespace:  "test-ns",
		firstSeen: map[string]time.Time{
			"i-old": time.Now().Add(-15 * time.Minute),
		},
	}
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 10*time.Minute)

	if len(mp.deletedIDs) != 1 || mp.deletedIDs[0] != "i-old" {
		t.Errorf("expected only i-old to be deleted, got %v", mp.deletedIDs)
	}
	if _, ok := gc.firstSeen["i-new"]; !ok {
		t.Error("expected i-new to be tracked in firstSeen")
	}
}

func TestCollect_ZeroGracePeriod_DeletesImmediately(t *testing.T) {
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mp := &mockProvider{}
	lister := &mockLister{
		instances: []*provider.Instance{{ID: "i-instant", Name: "instant-orphan"}},
	}

	gc := &GarbageCollector{KubeClient: k8sClient, Namespace: "test-ns"}

	// First call: firstSeen is recorded with time.Now(), so even 0 grace period
	// will see time.Since(firstSeen) as ~0ns which is >= 0. The instance should
	// be deleted because the check is `< gracePeriod` and gracePeriod is 0.
	gc.collect(context.Background(), mp, lister, "aws", "uid-123", 0)

	if len(mp.deletedIDs) != 1 || mp.deletedIDs[0] != "i-instant" {
		t.Errorf("expected immediate deletion with zero grace period, got %v", mp.deletedIDs)
	}
}

func TestParseDurationConfig(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]string
		key        string
		defaultVal time.Duration
		want       time.Duration
	}{
		{
			name:       "missing key uses default",
			data:       map[string]string{},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       30 * time.Minute,
		},
		{
			name:       "empty value uses default",
			data:       map[string]string{"GC_INTERVAL": ""},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       30 * time.Minute,
		},
		{
			name:       "valid duration parsed",
			data:       map[string]string{"GC_INTERVAL": "5m"},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       5 * time.Minute,
		},
		{
			name:       "invalid duration uses default",
			data:       map[string]string{"GC_INTERVAL": "notaduration"},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       30 * time.Minute,
		},
		{
			name:       "negative duration uses default",
			data:       map[string]string{"GC_INTERVAL": "-5m"},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       30 * time.Minute,
		},
		{
			name:       "zero duration uses default",
			data:       map[string]string{"GC_INTERVAL": "0s"},
			key:        "GC_INTERVAL",
			defaultVal: 30 * time.Minute,
			want:       30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDurationConfig(tt.data, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("parseDurationConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
