// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/libvirt"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/distribution/reference"
)

// ByomProvisioner extends LibvirtProvisioner for BYOM-specific functionality
type ByomProvisioner struct {
	*libvirt.LibvirtProvisioner
	provisionerCreatedVMs []string // Track VMs created by this provisioner instance
}

// ByomInstallOverlay implements the InstallOverlay interface
type ByomInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

type ByomProperties struct {
	SSHSecretPrivKeyPath string
	SSHSecretPubKeyPath  string
	SSHUsername          string
	VMPoolIPs            string
	ClusterName          string
	ContainerRuntime     string
	LibvirtNetwork       string
	LibvirtStorage       string
	LibvirtURI           string
	LibvirtConnURI       string
	LibvirtVolName       string
	LibvirtSSHKeyFile    string
	CaaImage             string
	CaaImageTag          string
}

var ByomProps = &ByomProperties{}

func initByomProperties(properties map[string]string) error {
	ByomProps = &ByomProperties{
		SSHSecretPrivKeyPath: properties["SSH_SECRET_PRIV_KEY_PATH"],
		SSHSecretPubKeyPath:  properties["SSH_SECRET_PUB_KEY_PATH"],
		SSHUsername:          properties["SSH_USERNAME"],
		VMPoolIPs:            properties["VM_POOL_IPS"],
		ClusterName:          properties["CLUSTER_NAME"],
		ContainerRuntime:     properties["CONTAINER_RUNTIME"],
		LibvirtNetwork:       properties["LIBVIRT_NETWORK"],
		LibvirtStorage:       properties["LIBVIRT_STORAGE"],
		LibvirtURI:           properties["LIBVIRT_URI"],
		LibvirtConnURI:       properties["LIBVIRT_CONN_URI"],
		LibvirtVolName:       properties["LIBVIRT_VOL_NAME"],
		LibvirtSSHKeyFile:    properties["LIBVIRT_SSH_KEY_FILE"],
		CaaImage:             properties["CAA_IMAGE"],
		CaaImageTag:          properties["CAA_IMAGE_TAG"],
	}

	// Set defaults
	if ByomProps.SSHUsername == "" {
		ByomProps.SSHUsername = "peerpod"
	}
	if ByomProps.ClusterName == "" {
		ByomProps.ClusterName = "peer-pods"
	}
	if ByomProps.ContainerRuntime == "" {
		ByomProps.ContainerRuntime = "containerd"
	}
	if ByomProps.LibvirtNetwork == "" {
		ByomProps.LibvirtNetwork = "default"
	}
	if ByomProps.LibvirtStorage == "" {
		ByomProps.LibvirtStorage = "default"
	}
	if ByomProps.LibvirtURI == "" {
		ByomProps.LibvirtURI = "qemu+ssh://root@192.168.122.1/system?no_verify=1"
	}
	if ByomProps.LibvirtConnURI == "" {
		ByomProps.LibvirtConnURI = "qemu:///system"
	}
	if ByomProps.LibvirtVolName == "" {
		ByomProps.LibvirtVolName = "podvm-base.qcow2"
	}

	return nil
}

func NewByomProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	if err := initByomProperties(properties); err != nil {
		return nil, err
	}

	// Create libvirt properties from BYOM properties
	libvirtProps := map[string]string{
		"libvirt_network":      ByomProps.LibvirtNetwork,
		"libvirt_storage":      ByomProps.LibvirtStorage,
		"libvirt_uri":          ByomProps.LibvirtURI,
		"libvirt_conn_uri":     ByomProps.LibvirtConnURI,
		"libvirt_vol_name":     ByomProps.LibvirtVolName,
		"libvirt_ssh_key_file": ByomProps.LibvirtSSHKeyFile,
		"cluster_name":         ByomProps.ClusterName,
		"container_runtime":    ByomProps.ContainerRuntime,
	}

	libvirtProvisioner, err := libvirt.NewLibvirtProvisioner(libvirtProps)
	if err != nil {
		return nil, err
	}

	return &ByomProvisioner{
		LibvirtProvisioner:    libvirtProvisioner.(*libvirt.LibvirtProvisioner),
		provisionerCreatedVMs: make([]string, 0),
	}, nil
}

func (b *ByomProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return b.LibvirtProvisioner.CreateCluster(ctx, cfg)
}

// Calling this method means we are not using existing pre-created VM IPs. Instead asking the provisioner
// to create a new VM from the uploaded image and use its IP.
func (b *ByomProvisioner) CreatePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	// Create VM from libvirt storage volume (uploaded via UploadPodvm)
	vmName := fmt.Sprintf("byom-vm-%d", time.Now().Unix())
	if err := b.createVMFromStorage(vmName); err != nil {
		return err
	}

	// Track this VM as created by the provisioner
	b.provisionerCreatedVMs = append(b.provisionerCreatedVMs, vmName)

	ip, err := b.getVMIPAddress(vmName)
	if err != nil {
		return err
	}

	ByomProps.VMPoolIPs = ip

	log.Infof("Created VM instance %s with IP: %s (total pool: %s)", vmName, ip, ByomProps.VMPoolIPs)
	return nil
}

func (b *ByomProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return b.LibvirtProvisioner.DeleteCluster(ctx, cfg)
}

func (b *ByomProvisioner) DeletePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	// Only delete VMs that were created by this provisioner instance
	if len(b.provisionerCreatedVMs) == 0 {
		log.Info("No VMs created by this provisioner to clean up")
		return nil
	}

	log.Infof("Cleaning up %d VMs created by this provisioner", len(b.provisionerCreatedVMs))

	for _, vmName := range b.provisionerCreatedVMs {
		log.Infof("Destroying provisioner-created VM: %s", vmName)
		if err := b.destroyVM(vmName); err != nil {
			log.Warnf("Failed to destroy VM %s: %v", vmName, err)
		}
	}

	// Clear the tracking and VM pool IPs
	b.provisionerCreatedVMs = make([]string, 0)
	ByomProps.VMPoolIPs = ""

	return nil
}

func (b *ByomProvisioner) GetProvisionValues() map[string]interface{} {
	// TODO: implement properly
	return nil
}

func (b *ByomProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"VM_POOL_IPS":              ByomProps.VMPoolIPs,
		"SSH_SECRET_PRIV_KEY_PATH": ByomProps.SSHSecretPrivKeyPath,
		"SSH_SECRET_PUB_KEY_PATH":  ByomProps.SSHSecretPubKeyPath,
		"SSH_USERNAME":             ByomProps.SSHUsername,
		"CLUSTER_NAME":             ByomProps.ClusterName,
		"CONTAINER_RUNTIME":        ByomProps.ContainerRuntime,
		"LIBVIRT_NETWORK":          ByomProps.LibvirtNetwork,
		"LIBVIRT_STORAGE":          ByomProps.LibvirtStorage,
		"LIBVIRT_URI":              ByomProps.LibvirtURI,
		"LIBVIRT_CONN_URI":         ByomProps.LibvirtConnURI,
		"LIBVIRT_VOL_NAME":         ByomProps.LibvirtVolName,
		"LIBVIRT_SSH_KEY_FILE":     ByomProps.LibvirtSSHKeyFile,
		"CAA_IMAGE":                ByomProps.CaaImage,
		"CAA_IMAGE_TAG":            ByomProps.CaaImageTag,
	}
}

func NewByomInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		log.Errorf("Error creating the byom provider install overlay: %v", err)
		return nil, err
	}

	return &ByomInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (bio *ByomInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return bio.Overlay.Apply(ctx, cfg)
}

func (bio *ByomInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return bio.Overlay.Delete(ctx, cfg)
}

func (bio *ByomInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Configure CAA image if provided
	if caaImage, exists := properties["CAA_IMAGE"]; exists && caaImage != "" {
		spec, err := reference.Parse(caaImage)
		if err != nil {
			return fmt.Errorf("parsing CAA image: %w", err)
		}

		log.Infof("Updating CAA image with %q", spec.String())
		if err = bio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newName", spec.String()); err != nil {
			return err
		}
	}

	if caaImageTag, exists := properties["CAA_IMAGE_TAG"]; exists && caaImageTag != "" {
		log.Infof("Updating CAA image tag with %q", caaImageTag)
		if err := bio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", caaImageTag); err != nil {
			return err
		}
	}

	// Configure VM pool IPs and SSH settings in ConfigMap
	configMapKeys := []string{"VM_POOL_IPS", "SSH_USERNAME"}

	for _, key := range configMapKeys {
		if value, exists := properties[key]; exists && value != "" {
			if err := bio.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", key, value); err != nil {
				return err
			}
		}
	}

	// Configure SSH key files in secrets. This is the key pair used when building the pod VM image
	// and used by CAA to SSH into the Pod VMs.
	if sshPrivKey, exists := properties["SSH_SECRET_PRIV_KEY_PATH"]; exists && sshPrivKey != "" {
		if err := bio.Overlay.SetKustomizeSecretGeneratorFile("ssh-key-secret", sshPrivKey); err != nil {
			return err
		}
	}

	if sshPubKey, exists := properties["SSH_SECRET_PUB_KEY_PATH"]; exists && sshPubKey != "" {
		if err := bio.Overlay.SetKustomizeSecretGeneratorFile("ssh-key-secret", sshPubKey); err != nil {
			return err
		}
	}

	if err := bio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}

func (b *ByomProvisioner) createVMFromStorage(vmName string) error {
	log.Infof("Creating VM %s from libvirt storage", vmName)

	// Use virt-install to create VM with UEFI boot
	cmd := exec.Command("sudo", "virt-install",
		"--name", vmName,
		"--memory", "1024",
		"--vcpus", "1",
		"--disk", fmt.Sprintf("vol=%s/%s,bus=virtio", ByomProps.LibvirtStorage, ByomProps.LibvirtVolName),
		"--network", fmt.Sprintf("network=%s,model=virtio", ByomProps.LibvirtNetwork),
		"--boot", "uefi",
		"--osinfo", "detect=on,require=off",
		"--noautoconsole",
		"--import")

	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to create VM with virt-install: %w", err)
	}

	log.Infof("VM %s created and started successfully", vmName)
	return nil
}

func (b *ByomProvisioner) getVMIPAddress(vmName string) (string, error) {
	log.Infof("Getting IP address for VM %s", vmName)

	// Wait up to 2 minutes for VM to get an IP
	timeout := time.Now().Add(2 * time.Minute)
	for time.Now().Before(timeout) {
		cmd := exec.Command("sudo", "virsh", "--quiet", "domifaddr", vmName)
		output, err := cmd.Output()
		if err != nil {
			log.Debugf("Error getting VM IP (retrying): %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				// Extract IP from CIDR format (e.g., 192.168.122.100/24)
				ipCIDR := fields[3]
				if strings.Contains(ipCIDR, "/") {
					ip := strings.Split(ipCIDR, "/")[0]
					log.Infof("Found IP %s for VM %s", ip, vmName)
					return ip, nil
				}
			}
		}

		log.Debugf("No IP found yet for VM %s, retrying...", vmName)
		time.Sleep(5 * time.Second)
	}

	return "", fmt.Errorf("timeout waiting for VM %s to get IP address", vmName)
}

func (b *ByomProvisioner) destroyVM(vmName string) error {
	log.Infof("Destroying VM %s", vmName)

	// Stop VM (force if necessary)
	if err := exec.Command("sudo", "virsh", "destroy", vmName).Run(); err != nil {
		return fmt.Errorf("Failed to destroy VM %s: %v", vmName, err)
	}

	// Undefine VM with storage and NVRAM cleanup for UEFI VMs
	if err := exec.Command("sudo", "virsh", "undefine", vmName, "--remove-all-storage", "--nvram").Run(); err != nil {
		return fmt.Errorf("Failed to undefine VM %s: %v", vmName, err)
	}

	log.Infof("VM %s destroyed successfully", vmName)
	return nil
}
