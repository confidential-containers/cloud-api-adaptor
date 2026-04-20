// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/docker"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// ByomProvisioner uses DockerProvisioner for BYOM-specific functionality
type ByomProvisioner struct {
	*docker.DockerProvisioner
	provisionerCreatedVMs []string // Track VMs created by this provisioner instance
}

// ByomInstallChart implements the InstallChart interface
type ByomInstallChart struct {
	Helm *pv.Helm
}

type ByomProperties struct {
	SSHSecretPrivKeyPath string
	SSHSecretPubKeyPath  string
	SSHUsername          string
	VMPoolIPs            string
	PoolSize             int // Number of containers to create for the pool
	ClusterName          string
	WorkerNodeName       string
	ContainerRuntime     string
	DockerHost           string
	DockerNetworkName    string
	ByomPodvmImage       string
	CaaImage             string
	CaaImageTag          string
	KindConfigFile       string
}

var ByomProps = &ByomProperties{}

func initByomProperties(properties map[string]string) error {
	poolSize, err := strconv.Atoi(properties["POOL_SIZE"])
	if err != nil || poolSize <= 0 {
		return fmt.Errorf("invalid POOL_SIZE value: %s", properties["POOL_SIZE"])
	}

	ByomProps = &ByomProperties{
		SSHSecretPrivKeyPath: properties["SSH_SECRET_PRIV_KEY_PATH"],
		SSHSecretPubKeyPath:  properties["SSH_SECRET_PUB_KEY_PATH"],
		SSHUsername:          properties["SSH_USERNAME"],
		VMPoolIPs:            properties["VM_POOL_IPS"],
		PoolSize:             poolSize,
		ClusterName:          properties["CLUSTER_NAME"],
		WorkerNodeName:       properties["WORKER_NODE_NAME"],
		ContainerRuntime:     properties["CONTAINER_RUNTIME"],
		DockerHost:           properties["DOCKER_HOST"],
		DockerNetworkName:    properties["DOCKER_NETWORK_NAME"],
		ByomPodvmImage:       properties["BYOM_PODVM_IMAGE"],
		CaaImage:             properties["CAA_IMAGE"],
		CaaImageTag:          properties["CAA_IMAGE_TAG"],
		KindConfigFile:       properties["KIND_CONFIG_FILE"],
	}

	// Set defaults
	if ByomProps.SSHUsername == "" {
		ByomProps.SSHUsername = "peerpod"
	}
	if ByomProps.ClusterName == "" {
		ByomProps.ClusterName = "peer-pods"
	}
	if ByomProps.WorkerNodeName == "" {
		ByomProps.WorkerNodeName = fmt.Sprintf("%s-worker", ByomProps.ClusterName)
	}
	if ByomProps.ContainerRuntime == "" {
		ByomProps.ContainerRuntime = "containerd"
	}
	if ByomProps.DockerNetworkName == "" {
		ByomProps.DockerNetworkName = "kind"
	}

	return nil
}

func NewByomProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	if err := initByomProperties(properties); err != nil {
		return nil, err
	}
	dockerProps := map[string]string{
		"DOCKER_HOST":         ByomProps.DockerHost,
		"DOCKER_NETWORK_NAME": ByomProps.DockerNetworkName,
		"BYOM_PODVM_IMAGE":    ByomProps.ByomPodvmImage,
		"CLUSTER_NAME":        ByomProps.ClusterName,
		"CONTAINER_RUNTIME":   ByomProps.ContainerRuntime,
		"CAA_IMAGE":           ByomProps.CaaImage,
		"CAA_IMAGE_TAG":       ByomProps.CaaImageTag,
	}

	dockerProvisioner, err := docker.NewDockerProvisioner(dockerProps)
	if err != nil {
		return nil, err
	}

	return &ByomProvisioner{
		DockerProvisioner: dockerProvisioner.(*docker.DockerProvisioner),
	}, nil
}

func (b *ByomProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	kindConfigPath, err := filepath.Abs(ByomProps.KindConfigFile)
	if err != nil {
		return fmt.Errorf("error getting absolute path of kind config file: %w", err)
	}

	log.Infof("Using BYOM kind config from: %s", kindConfigPath)
	os.Setenv("KIND_CONFIG_FILE", kindConfigPath)

	if err := b.DockerProvisioner.CreateCluster(ctx, cfg); err != nil {
		return err
	}

	// Update containerd configuration to not discard unpacked layers
	log.Info("Configuring containerd on worker node to keep unpacked layers...")

	cmd := exec.Command("docker", "exec", ByomProps.WorkerNodeName, "sed", "-i",
		"s/discard_unpacked_layers = true/discard_unpacked_layers = false/g",
		"/etc/containerd/config.toml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to update containerd config: %v, output: %s", err, string(output))
	} else {
		log.Info("Updated containerd config to keep unpacked layers")

		// Restart containerd to apply the change
		cmd = exec.Command("docker", "exec", ByomProps.WorkerNodeName, "systemctl", "restart", "containerd")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Warnf("Failed to restart containerd: %v, output: %s", err, string(output))
		} else {
			log.Info("Restarted containerd, waiting for it to be ready...")
			time.Sleep(5 * time.Second)

			// Verify if containerd is running
			cmd = exec.Command("docker", "exec", ByomProps.WorkerNodeName, "systemctl", "is-active", "containerd")
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

// CreatePodVMInstance creates new containers from the uploaded image and use their IPs for the pool.
func (b *ByomProvisioner) CreatePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	log.Infof("Creating %d BYOM container instances", ByomProps.PoolSize)

	var poolIPs []string

	for i := 0; i < ByomProps.PoolSize; i++ {
		containerName := fmt.Sprintf("byom-container-%d-%d", time.Now().Unix(), i)

		log.Infof("Creating container %d/%d: %s", i+1, ByomProps.PoolSize, containerName)
		if err := b.createContainerFromImage(containerName); err != nil {
			return fmt.Errorf("failed to create container %d: %w", i, err)
		}

		b.provisionerCreatedVMs = append(b.provisionerCreatedVMs, containerName)

		// Get container IP address
		ip, err := b.getContainerIPAddress(containerName)
		if err != nil {
			return fmt.Errorf("failed to get IP for container %d: %w", i, err)
		}

		poolIPs = append(poolIPs, ip)
		log.Infof("Created container %d/%d: %s with IP: %s", i+1, ByomProps.PoolSize, containerName, ip)
	}

	ByomProps.VMPoolIPs = strings.Join(poolIPs, ",")

	log.Infof("Successfully created %d containers with pool IPs: %s", ByomProps.PoolSize, ByomProps.VMPoolIPs)
	return nil
}

func (b *ByomProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return b.DockerProvisioner.DeleteCluster(ctx, cfg)
}

func (b *ByomProvisioner) DeletePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	if len(b.provisionerCreatedVMs) == 0 {
		log.Info("No containers created by this provisioner to clean up")
		return nil
	}

	log.Infof("Cleaning up %d containers created by this provisioner", len(b.provisionerCreatedVMs))

	for _, containerName := range b.provisionerCreatedVMs {
		log.Infof("Destroying provisioner-created container: %s", containerName)
		if err := b.destroyContainer(containerName); err != nil {
			log.Warnf("Failed to destroy container %s: %v", containerName, err)
		}
	}

	// Clear the tracking and VM pool IPs
	b.provisionerCreatedVMs = make([]string, 0)
	ByomProps.VMPoolIPs = ""

	return nil
}

func (b *ByomProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"VM_POOL_IPS":              ByomProps.VMPoolIPs,
		"SSH_SECRET_PRIV_KEY_PATH": ByomProps.SSHSecretPrivKeyPath,
		"SSH_SECRET_PUB_KEY_PATH":  ByomProps.SSHSecretPubKeyPath,
		"SSH_USERNAME":             ByomProps.SSHUsername,
		"CLUSTER_NAME":             ByomProps.ClusterName,
		"WORKER_NODE_NAME":         ByomProps.WorkerNodeName,
		"CONTAINER_RUNTIME":        ByomProps.ContainerRuntime,
		"DOCKER_HOST":              ByomProps.DockerHost,
		"DOCKER_NETWORK_NAME":      ByomProps.DockerNetworkName,
		"BYOM_PODVM_IMAGE":         ByomProps.ByomPodvmImage,
		"CAA_IMAGE":                ByomProps.CaaImage,
		"CAA_IMAGE_TAG":            ByomProps.CaaImageTag,
		"KIND_CONFIG_FILE":         ByomProps.KindConfigFile,
	}
}

func NewByomInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &ByomInstallChart{
		Helm: helm,
	}, nil
}

func (b *ByomInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	// Create SSH key secret before installing Helm chart
	if err := b.createSSHKeySecret(ctx, cfg); err != nil {
		return fmt.Errorf("failed to create SSH key secret: %w", err)
	}

	if err := b.Helm.Install(ctx, cfg); err != nil {
		return err
	}

	// Wait for the worker node and heck for kata-runtime label
	log.Info("Waiting for worker node to be labeled with kata-runtime...")
	timeout := time.Now().Add(3 * time.Minute)
	for time.Now().Before(timeout) {
		checkLabel := exec.Command("kubectl", "get", "node", ByomProps.WorkerNodeName, "--show-labels")
		checkLabel.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
		out, err := checkLabel.Output()
		if err == nil && strings.Contains(string(out), "katacontainers.io/kata-runtime=true") {
			log.Info("Worker node is labeled with kata-runtime and ready!")
			return nil
		}
		time.Sleep(15 * time.Second)
	}

	return fmt.Errorf("kata-runtime label not found - node may not be ready for deployment")
}

func (b *ByomInstallChart) createSSHKeySecret(ctx context.Context, cfg *envconf.Config) error {
	if ByomProps.SSHSecretPrivKeyPath == "" || ByomProps.SSHSecretPubKeyPath == "" {
		return fmt.Errorf("SSH_SECRET_PRIV_KEY_PATH and SSH_SECRET_PUB_KEY_PATH must be set")
	}

	// Create namespace first if it doesn't exist
	if err := pv.CreateAndWaitForNamespace(ctx, cfg.Client(), b.Helm.Namespace); err != nil {
		return fmt.Errorf("failed to create Namespace: %w", err)
	}

	secretName := "ssh-key-secret"
	log.Infof("Creating SSH key secret from %s and %s", ByomProps.SSHSecretPrivKeyPath, ByomProps.SSHSecretPubKeyPath)

	// Create the secret using kubectl
	args := []string{
		"create", "secret", "generic", secretName,
		"--from-file=id_rsa=" + ByomProps.SSHSecretPrivKeyPath,
		"--from-file=id_rsa.pub=" + ByomProps.SSHSecretPubKeyPath,
		"-n", b.Helm.Namespace,
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to create ssh-key-secret: %w, output: %s", err, string(stdoutStderr))
	}
	log.Infof("Created ssh-key-secret from SSH key files")
	return nil
}

func (b *ByomInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return b.Helm.Uninstall(ctx, cfg)
}

func (b *ByomInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Handle CAA image - already split into CAA_IMAGE and CAA_IMAGE_TAG
	if properties["CAA_IMAGE"] != "" {
		b.Helm.OverrideValues["image.name"] = properties["CAA_IMAGE"]
	}
	if properties["CAA_IMAGE_TAG"] != "" {
		b.Helm.OverrideValues["image.tag"] = properties["CAA_IMAGE_TAG"]
	}

	// Mapping the internal properties to Helm chart values.
	mapProps := map[string]string{
		"VM_POOL_IPS":              "VM_POOL_IPS",
		"SSH_USERNAME":             "SSH_USERNAME",
		"SSH_SECRET_PRIV_KEY_PATH": "SSH_SECRET_PRIV_KEY_PATH",
		"SSH_SECRET_PUB_KEY_PATH":  "SSH_SECRET_PUB_KEY_PATH",
		"DOCKER_HOST":              "DOCKER_HOST",
		"DOCKER_NETWORK_NAME":      "DOCKER_NETWORK_NAME",
		"BYOM_PODVM_IMAGE":         "BYOM_PODVM_IMAGE",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			b.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	return nil
}

func (b *ByomProvisioner) createContainerFromImage(containerName string) error {
	log.Infof("Creating container %s from Docker image", containerName)

	cmd := exec.Command("docker", "run",
		"-d",
		"--name", containerName,
		"--network", ByomProps.DockerNetworkName,
		"--restart=always",
		"--privileged",
		ByomProps.ByomPodvmImage)

	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to create container with docker run: %w, output: %s", err, string(stdoutStderr))
	}

	log.Infof("Container %s created and started successfully", containerName)

	// Copy SSH public key to container
	if ByomProps.SSHSecretPubKeyPath != "" {
		if err := b.copySSHKeyToContainer(containerName); err != nil {
			return fmt.Errorf("failed to copy SSH key to container: %w", err)
		}
	}

	return nil
}

func (b *ByomProvisioner) copySSHKeyToContainer(containerName string) error {
	log.Infof("Copying SSH public key from %s to container %s", ByomProps.SSHSecretPubKeyPath, containerName)

	// Copy the public key file to container
	cmd := exec.Command("docker", "cp",
		ByomProps.SSHSecretPubKeyPath,
		fmt.Sprintf("%s:/home/peerpod/.ssh/authorized_keys", containerName))

	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to copy SSH key: %w, output: %s", err, string(stdoutStderr))
	}

	// Fix permissions on the authorized_keys file
	cmd = exec.Command("docker", "exec", containerName,
		"chmod", "600", "/home/peerpod/.ssh/authorized_keys")
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to set permissions on authorized_keys: %w, output: %s", err, string(stdoutStderr))
	}

	// Fix ownership of the authorized_keys file
	cmd = exec.Command("docker", "exec", containerName,
		"chown", "peerpod:peerpod", "/home/peerpod/.ssh/authorized_keys")
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to set ownership on authorized_keys: %w, output: %s", err, string(stdoutStderr))
	}

	log.Infof("SSH public key copied and configured successfully")
	return nil
}

func (b *ByomProvisioner) getContainerIPAddress(containerName string) (string, error) {
	log.Infof("Getting IP address for container %s", containerName)

	timeout := time.Now().Add(30 * time.Second)
	for time.Now().Before(timeout) {
		cmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName)
		output, err := cmd.Output()
		if err != nil {
			log.Debugf("Error getting container IP (retrying): %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		ip := strings.TrimSpace(string(output))
		if ip != "" {
			log.Infof("Found IP %s for container %s", ip, containerName)
			return ip, nil
		}

		log.Debugf("No IP found yet for container %s, retrying...", containerName)
		time.Sleep(5 * time.Second)
	}

	return "", fmt.Errorf("timeout waiting for container %s to get IP address", containerName)
}

func (b *ByomProvisioner) destroyContainer(containerName string) error {
	log.Infof("Destroying container %s", containerName)

	if err := exec.Command("docker", "stop", containerName).Run(); err != nil {
		log.Warnf("Failed to stop container %s: %v", containerName, err)
	}

	if err := exec.Command("docker", "rm", "-f", containerName).Run(); err != nil {
		return fmt.Errorf("Failed to remove container %s: %v", containerName, err)
	}

	log.Infof("Container %s destroyed successfully", containerName)
	return nil
}
