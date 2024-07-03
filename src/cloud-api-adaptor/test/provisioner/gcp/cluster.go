// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
	//"path/filepath"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	//clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
)

// GKECluster implements the basic GKE Cluster client operations.
type GKECluster struct {
	clusterName    string
	clusterVersion string
	credentials    string
	machineType    string
	nodeCount      int64
	projectID      string
	zone           string
	cluster        *container.Cluster
}

// NewGKECluster creates a new GKECluster with the given properties
func NewGKECluster(properties map[string]string) (*GKECluster, error) {
	defaults := map[string]string{
		"cluster_name":    "peer-pods",
		"cluster_version": "1.27.11-gke.1062004",
		"machine_type":    "n1-standard-1",
		"node_count":      "2",
		"zone":            "us-central1-a",
	}

	for key, value := range properties {
		defaults[key] = value
	}

	requiredFields := []string{"project_id", "credentials"}
	for _, field := range requiredFields {
		if _, ok := defaults[field]; !ok {
			return nil, fmt.Errorf("%s is required", field)
		}
	}

	nodeCount, err := strconv.ParseInt(defaults["node_count"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid node_count: %v", err)
	}

	return &GKECluster{
		clusterName:    defaults["cluster_name"],
		clusterVersion: defaults["cluster_version"],
		credentials:    defaults["credentials"],
		machineType:    defaults["machine_type"],
		nodeCount:      nodeCount,
		projectID:      defaults["project_id"],
		zone:           defaults["zone"],
		cluster:        nil,
	}, nil
}

// CreateCluster creates the GKE cluster
func (g *GKECluster) CreateCluster(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	srv, err := container.NewService(
		ctx, option.WithCredentialsFile(g.credentials),
	)
	if err != nil {
		return fmt.Errorf("GKE: container.NewService: %v", err)
	}

	cluster := &container.Cluster{
		Name:             g.clusterName,
		InitialNodeCount: g.nodeCount,
		NodeConfig: &container.NodeConfig{
			MachineType: g.machineType,
			ImageType:   "UBUNTU_CONTAINERD", // Default CO OS has a ro fs.
		},
	}

	req := &container.CreateClusterRequest{
		Cluster: cluster,
	}

	op, err := srv.Projects.Zones.Clusters.Create(
		g.projectID, g.zone, req,
	).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Projects.Zones.Clusters.Create: %v", err)
	}

	log.Infof("GKE: Cluster creation operation: %v\n", op.Name)

	g.cluster, err = g.WaitForClusterActive(ctx, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("GKE: Error waiting for cluster to become active: %v", err)
	}

	err = g.ApplyNodeLabels(ctx)
	if err != nil {
		return fmt.Errorf("GKE: Error applying node labels: %v", err)
	}
	return nil
}

// DeleteCluster deletes the GKE cluster
func (g *GKECluster) DeleteCluster(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	srv, err := container.NewService(
		ctx, option.WithCredentialsFile(g.credentials),
	)
	if err != nil {
		return fmt.Errorf("GKE: container.NewService: %v", err)
	}

	op, err := srv.Projects.Zones.Clusters.Delete(
		g.projectID, g.zone, g.clusterName,
	).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Projects.Zones.Clusters.Delete: %v", err)
	}

	log.Infof("GKE: Cluster deletion operation: %v\n", op.Name)

	// Wait for the cluster to be deleted
	activationTimeout := 30 * time.Minute
	err = g.WaitForClusterDeleted(ctx, activationTimeout)
	if err != nil {
		return fmt.Errorf("GKE: error waiting for cluster to be deleted: %v", err)
	}
	return nil
}

// WaitForClusterActive waits until the GKE cluster is active
func (g *GKECluster) WaitForClusterActive(
	ctx context.Context, activationTimeout time.Duration,
) (*container.Cluster, error) {
	srv, err := container.NewService(
		ctx, option.WithCredentialsFile(g.credentials),
	)
	if err != nil {
		return nil, fmt.Errorf("GKE: container.NewService: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, activationTimeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("GKE: Reached timeout waiting for cluster.")
		case <-ticker.C:
			cluster, err := srv.Projects.Zones.Clusters.Get(g.projectID, g.zone, g.clusterName).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("GKE: Projects.Zones.Clusters.Get: %v", err)
			}

			if cluster.Status == "RUNNING" {
				log.Info("GKE: Cluster is now active")
				return cluster, nil
			}

			log.Info("GKE: Waiting for cluster to become active...")
		}
	}
}

// WaitForClusterDeleted waits until the GKE cluster is deleted
func (g *GKECluster) WaitForClusterDeleted(
	ctx context.Context, activationTimeout time.Duration,
) error {
	srv, err := container.NewService(
		ctx, option.WithCredentialsFile(g.credentials),
	)
	if err != nil {
		return fmt.Errorf("GKE: container.NewService: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, activationTimeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("GKE: timeout waiting for cluster deletion")
		case <-ticker.C:
			_, err := srv.Projects.Zones.Clusters.Get(g.projectID, g.zone, g.clusterName).Context(ctx).Do()
			if err != nil {
				if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 {
					log.Info("GKE: Cluster deleted successfully")
					return nil
				}
				return fmt.Errorf("GKE: Projects.Zones.Clusters.Get: %v", err)
			}

			log.Info("GKE: Waiting for cluster to be deleted...")
		}
	}
}

func (g *GKECluster) ApplyNodeLabels(ctx context.Context) error {
	kubeconfigPath, err := g.GetKubeconfigFile(ctx)
	if err != nil {
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	for _, node := range nodes.Items {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			n, err := clientset.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get node: %v", err)
			}

			n.Labels["node.kubernetes.io/worker"] = ""
			n.Labels["node-role.kubernetes.io/worker"] = ""
			_, err = clientset.CoreV1().Nodes().Update(ctx, n, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			return fmt.Errorf("Failed to label node %s: %v\n", node.Name, err)
		}
		log.Infof("Successfully labeled node %s\n", node.Name)
	}
	return nil
}

// GetKubeconfigFile retrieves the path to the kubeconfig file
func (g *GKECluster) GetKubeconfigFile(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gcloud", "container", "clusters", "get-credentials", g.clusterName, "--zone", g.zone, "--project", g.projectID)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("Failed to get cluster credentials: %v\nOutput: %s", err, output)
	}

	if g.cluster == nil {
		return "", fmt.Errorf("Cluster not found. Call CreateCluster() first.")
	}

	kubeconfigPath := kconf.ResolveKubeConfigFile()
	_, err = os.Stat(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("Failed to resolve KubeConfigfile: %v", err)
	}
	return kubeconfigPath, nil
}
