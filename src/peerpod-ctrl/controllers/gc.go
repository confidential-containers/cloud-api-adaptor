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
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	confidentialcontainersorgv1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/peerpod-ctrl/api/v1alpha1"
)

const defaultGCInterval = 30 * time.Minute
const defaultGCGracePeriod = 10 * time.Minute

var logger = ctrl.Log.WithName("gc")

type GarbageCollector struct {
	KubeClient client.Client
	Namespace  string
	firstSeen  map[string]time.Time
}

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get

func (gc *GarbageCollector) Start(ctx context.Context) error {
	if !gc.isGCEnabled(ctx) {
		logger.Info("GC is disabled via ENABLE_GC=false")
		return nil
	}

	var (
		cloudProvider string
		clusterUID    string
		p             provider.Provider
		lister        provider.InstanceLister
		initialized   bool
	)

	interval, prevGracePeriod := gc.readGCDynamicConfig(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping garbage collector")
			return nil
		case <-ticker.C:
			newInterval, gracePeriod := gc.readGCDynamicConfig(ctx)
			if newInterval != interval {
				logger.Info("GC interval changed", "old", interval, "new", newInterval)
				interval = newInterval
				ticker.Reset(interval)
			}
			if gracePeriod != prevGracePeriod {
				logger.Info("GC grace period changed, resetting firstSeen tracking",
					"old", prevGracePeriod, "new", gracePeriod)
				gc.firstSeen = nil
				prevGracePeriod = gracePeriod
			}
			if !initialized {
				var err error
				cloudProvider, p, lister, clusterUID, err = gc.initialize(ctx)
				if err != nil {
					logger.Error(err, "failed to initialize garbage collector, will retry next cycle")
					continue
				}
				if lister == nil {
					return nil
				}
				initialized = true
				logger.Info("starting garbage collector", "provider", cloudProvider)
			}
			gc.collect(ctx, p, lister, cloudProvider, clusterUID, gracePeriod)
		}
	}
}

func (gc *GarbageCollector) initialize(ctx context.Context) (string, provider.Provider, provider.InstanceLister, string, error) {
	if err := loadCloudConfigs(ctx, gc.KubeClient, gc.Namespace); err != nil {
		return "", nil, nil, "", fmt.Errorf("failed to load cloud configs: %w", err)
	}
	logger.Info("cloud config loaded for GC")

	uid, err := gc.getClusterUID(ctx)
	if err != nil {
		return "", nil, nil, "", fmt.Errorf("failed to get cluster UID: %w", err)
	}

	cloudProvider := os.Getenv("CLOUD_PROVIDER")
	if cloudProvider == "" {
		return "", nil, nil, "", fmt.Errorf("CLOUD_PROVIDER is not set in peer-pods-cm")
	}

	p, err := GetProvider(cloudProvider)
	if err != nil {
		return "", nil, nil, "", fmt.Errorf("failed to create provider %s: %w", cloudProvider, err)
	}

	lister, ok := p.(provider.InstanceLister)
	if !ok {
		logger.Info("provider does not implement InstanceLister, GC disabled", "provider", cloudProvider)
		return cloudProvider, p, nil, uid, nil
	}

	return cloudProvider, p, lister, uid, nil
}

func (gc *GarbageCollector) getClusterUID(ctx context.Context) (string, error) {
	ns := corev1.Namespace{}
	if err := gc.KubeClient.Get(ctx, types.NamespacedName{Name: "kube-system"}, &ns); err != nil {
		return "", fmt.Errorf("failed to get kube-system namespace: %w", err)
	}
	logger.Info("cluster UID resolved", "uid", string(ns.UID))
	return string(ns.UID), nil
}

func parseDurationConfig(data map[string]string, key string, defaultVal time.Duration) time.Duration {
	v, ok := data[key]
	if !ok || v == "" {
		return defaultVal
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		logger.Error(err, "invalid duration config, using default", "key", key, "value", v)
		return defaultVal
	}
	if parsed <= 0 {
		logger.Info("WARNING: duration must be positive, falling back to default", "key", key, "value", v, "default", defaultVal)
		return defaultVal
	}
	return parsed
}

func (gc *GarbageCollector) isGCEnabled(ctx context.Context) bool {
	cm := corev1.ConfigMap{}
	if err := gc.KubeClient.Get(ctx, types.NamespacedName{Name: ppConfigMap, Namespace: gc.Namespace}, &cm); err != nil {
		logger.Error(err, "failed to read peer-pods-cm for ENABLE_GC, defaulting to enabled")
		return true
	}
	if v, ok := cm.Data["ENABLE_GC"]; ok && v == "false" {
		return false
	}
	return true
}

func (gc *GarbageCollector) readGCDynamicConfig(ctx context.Context) (interval, gracePeriod time.Duration) {
	interval = defaultGCInterval
	gracePeriod = defaultGCGracePeriod

	cm := corev1.ConfigMap{}
	if err := gc.KubeClient.Get(ctx, types.NamespacedName{Name: ppConfigMap, Namespace: gc.Namespace}, &cm); err != nil {
		logger.Error(err, "failed to read peer-pods-cm for GC config, using defaults")
		return
	}

	interval = parseDurationConfig(cm.Data, "GC_INTERVAL", defaultGCInterval)
	gracePeriod = parseDurationConfig(cm.Data, "GC_GRACE_PERIOD", defaultGCGracePeriod)
	return
}

func (gc *GarbageCollector) collect(ctx context.Context, p provider.Provider, lister provider.InstanceLister, cloudProvider, clusterUID string, gracePeriod time.Duration) {
	cloudInstances, err := lister.ListInstances(ctx, provider.ListInstancesInput{ClusterUID: clusterUID})
	if err != nil {
		logger.Error(err, "failed to list cloud instances, skipping GC cycle")
		return
	}

	if len(cloudInstances) == 0 {
		gc.firstSeen = nil
		return
	}

	ppList := confidentialcontainersorgv1alpha1.PeerPodList{}
	if err := gc.KubeClient.List(ctx, &ppList); err != nil {
		logger.Error(err, "failed to list PeerPod CRs, skipping GC cycle")
		return
	}

	knownInstances := make(map[string]struct{})
	for i := range ppList.Items {
		pp := &ppList.Items[i]
		if pp.Spec.CloudProvider == cloudProvider && pp.Spec.InstanceID != "" {
			knownInstances[pp.Spec.InstanceID] = struct{}{}
		}
	}

	deleted := 0
	if gc.firstSeen == nil {
		gc.firstSeen = make(map[string]time.Time)
	}

	cloudIDs := make(map[string]struct{}, len(cloudInstances))
	for _, inst := range cloudInstances {
		cloudIDs[inst.ID] = struct{}{}
	}

	for _, inst := range cloudInstances {
		if _, ok := knownInstances[inst.ID]; ok {
			delete(gc.firstSeen, inst.ID)
			continue
		}
		if _, seen := gc.firstSeen[inst.ID]; !seen {
			gc.firstSeen[inst.ID] = time.Now()
		}
		if time.Since(gc.firstSeen[inst.ID]) < gracePeriod {
			logger.Info("skipping recently discovered orphan candidate",
				"instanceID", inst.ID, "firstSeen", time.Since(gc.firstSeen[inst.ID]),
				"createdAt", inst.CreatedAt.UTC().Format(time.RFC3339))
			continue
		}
		logger.Info("deleting orphan instance", "instanceID", inst.ID, "instanceName", inst.Name,
			"createdAt", inst.CreatedAt.UTC().Format(time.RFC3339))
		if err := p.DeleteInstance(ctx, inst.ID); err != nil {
			logger.Error(err, "failed to delete orphan instance", "instanceID", inst.ID)
			continue
		}
		delete(gc.firstSeen, inst.ID)
		deleted++
	}

	for id := range gc.firstSeen {
		if _, exists := cloudIDs[id]; !exists {
			delete(gc.firstSeen, id)
		}
	}

	logger.Info("garbage collection cycle complete", "orphansDeleted", deleted, "totalCloudInstances", len(cloudInstances))
}
