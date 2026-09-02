// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs // IBMCloudPowerVSProvisioner implements the CloudProvisioner interface for IBM Cloud PowerVS.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

// IBMCloudPowerVSProvisioner implements CloudProvisioner for IBM Cloud PowerVS.
type IBMCloudPowerVSProvisioner struct {
	kind *KindCluster

	IBMCloudPowerVSAPIKey    string
	PowerVSZone              string
	PowerVSServiceInstanceID string
	PowerVSImageID           string
	PowerVSNetworkID         string
	PowerVSSSHKeyName        string
	PowerVSSystemType        string
	PowerVSMemory            string
	PowerVSProcessorType     string
	PowerVSProcessors        string
	ForwarderPort            string
	ProxyTimeout             string
	UsePublicIP              string
}

// KindClusterProperties holds the properties needed to manage a local kind cluster.
type KindClusterProperties struct {
	ClusterName      string
	ContainerRuntime string
	KindConfigFile   string
	WorkerNodeName   string
}

// KindCluster manages a local kind cluster for e2e testing.
type KindCluster struct {
	properties KindClusterProperties
}

func (p *IBMCloudPowerVSProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Info("IBMCloudPowerVS: provisioning local kind cluster for e2e tests")
	if err := p.kind.CreateCluster(ctx, cfg); err != nil {
		return err
	}
	log.Info("IBMCloudPowerVS: kind cluster ready")
	return nil
}

func (p *IBMCloudPowerVSProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Info("IBMCloudPowerVS: deleting local kind cluster")
	if err := p.kind.DeleteCluster(ctx, cfg); err != nil {
		return err
	}
	log.Info("IBMCloudPowerVS: kind cluster deleted")
	return nil
}

func (p *IBMCloudPowerVSProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMCloudPowerVSProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMCloudPowerVSProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"IBMCLOUD_API_KEY":            p.IBMCloudPowerVSAPIKey,
		"POWERVS_ZONE":                p.PowerVSZone,
		"POWERVS_SERVICE_INSTANCE_ID": p.PowerVSServiceInstanceID,
		"POWERVS_IMAGE_ID":            p.PowerVSImageID,
		"POWERVS_NETWORK_ID":          p.PowerVSNetworkID,
		"POWERVS_SSH_KEY_NAME":        p.PowerVSSSHKeyName,
		"POWERVS_SYSTEM_TYPE":         p.PowerVSSystemType,
		"POWERVS_MEMORY":              p.PowerVSMemory,
		"POWERVS_PROCESSOR_TYPE":      p.PowerVSProcessorType,
		"POWERVS_PROCESSORS":          p.PowerVSProcessors,
		"CLUSTER_NAME":                p.kind.properties.ClusterName,
		"FORWARDER_PORT":              p.ForwarderPort,
		"PROXY_TIMEOUT":               p.ProxyTimeout,
		"USE_PUBLIC_IP":               p.UsePublicIP,
	}
}

func (p *IBMCloudPowerVSProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func NewIBMCloudPowerVSProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	provisioner, err := newIBMCloudPowerVSProvisioner(properties)
	if err != nil {
		return nil, err
	}

	provisioner.kind, err = newKindCluster(properties)
	if err != nil {
		return nil, err
	}

	return provisioner, nil
}

func newKindCluster(properties map[string]string) (*KindCluster, error) {
	clusterName := properties["CLUSTER_NAME"]
	if clusterName == "" {
		clusterName = "peer-pods-e2e"
	}
	kindConfigFile := properties["KIND_CONFIG_FILE"]
	if kindConfigFile == "" {
		return nil, fmt.Errorf("KIND_CONFIG_FILE must be set")
	}
	containerRuntime := properties["CONTAINER_RUNTIME"]
	if containerRuntime == "" {
		containerRuntime = "containerd"
	}
	workerNodeName := properties["WORKER_NODE_NAME"]
	if workerNodeName == "" {
		workerNodeName = fmt.Sprintf("%s-worker", clusterName)
	}

	return &KindCluster{
		properties: KindClusterProperties{
			ClusterName:      clusterName,
			ContainerRuntime: containerRuntime,
			KindConfigFile:   kindConfigFile,
			WorkerNodeName:   workerNodeName,
		},
	}, nil
}

func (k *KindCluster) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	kindConfigPath, err := filepath.Abs(k.properties.KindConfigFile)
	if err != nil {
		return fmt.Errorf("error getting absolute path of kind config file: %w", err)
	}

	log.Infof("Using kind config from: %s", kindConfigPath)
	os.Setenv("KIND_CONFIG_FILE", kindConfigPath)

	if err := k.runScript("create"); err != nil {
		log.Errorf("Error creating kind cluster: %v", err)
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	cfg.WithKubeconfigFile(filepath.Join(home, ".kube/config"))

	if err := pv.AddNodeRoleWorkerLabel(context.Background(), k.properties.ClusterName, cfg); err != nil {
		return fmt.Errorf("failed to label nodes: %w", err)
	}
	// Update containerd configuration to not discard unpacked layers
	log.Info("Configuring containerd on worker node to keep unpacked layers...")

	cmd := exec.Command("docker", "exec", k.properties.WorkerNodeName, "sed", "-i",
		"s/discard_unpacked_layers = true/discard_unpacked_layers = false/g",
		"/etc/containerd/config.toml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to update containerd config: %v, output: %s", err, string(output))
	} else {
		log.Info("Updated containerd config to keep unpacked layers")

		// Restart containerd to apply the change
		cmd = exec.Command("docker", "exec", k.properties.WorkerNodeName, "systemctl", "restart", "containerd")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Warnf("Failed to restart containerd: %v, output: %s", err, string(output))
		} else {
			log.Info("Restarted containerd, waiting for it to be ready...")
			time.Sleep(5 * time.Second)

			// Verify if containerd is running
			cmd = exec.Command("docker", "exec", k.properties.WorkerNodeName, "systemctl", "is-active", "containerd")
			output, err = cmd.CombinedOutput()
			status := strings.TrimSpace(string(output))
			if err != nil || status != "active" {
				log.Warnf("Containerd may not be running properly: status=%s, err=%v", status, err)
			} else {
				log.Info("Containerd is active and running")
			}
		}
	}
	return nil
}

func (k *KindCluster) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return k.runScript("delete")
}

func (k *KindCluster) runScript(action string) error {
	scriptPath, err := pv.KindClusterScriptPath()
	if err != nil {
		return fmt.Errorf("failed to locate kind_cluster.sh: %w", err)
	}
	cmd := exec.Command("/bin/bash", scriptPath, action)
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	// Set CLUSTER_NAME and CONTAINER_RUNTIME. Unset KUBECONFIG so the default path is used.
	cmd.Env = append(cmd.Env,
		"CLUSTER_NAME="+k.properties.ClusterName,
		"KUBECONFIG=",
		"CONTAINER_RUNTIME="+k.properties.ContainerRuntime,
	)
	if err := cmd.Run(); err != nil {
		log.Errorf("Error running kind_cluster.sh %s: %v", action, err)
		return err
	}
	return nil
}
