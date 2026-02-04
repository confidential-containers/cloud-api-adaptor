// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"fmt"
	"strings"

	"os"
	"os/exec"
	"path"
	"path/filepath"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const AlternateVolumeName = "another-podvm-base.qcow2"
const DefaultMemory = 8192
const DefaultCPU = 2

// LibvirtProvisioner implements the CloudProvisioner interface for Libvirt.
type LibvirtProvisioner struct {
	caa_image        string           // The CAA image to install
	conn             *libvirt.Connect // Libvirt connection
	containerRuntime string           // Name of the container runtime
	network          string           // Network name
	ssh_key_file     string           // SSH key file used to connect to Libvirt
	storage          string           // Storage pool name
	uri              string           // Libvirt URI
	wd               string           // libvirt's directory path on this repository
	volumeName       string           // Podvm volume name
	clusterName      string           // Cluster name
	tunnelType       string           // Tunnel Type
	vxlanPort        string           // VXLAN port number
	initdata         string           // InitData
}

// LibvirtInstallOverlay implements the InstallOverlay interface
type LibvirtInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

type LibvirtInstallChart struct {
	Helm *pv.Helm
}

func NewLibvirtProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	wd, err := filepath.Abs(path.Join("..", "..", "libvirt"))
	if err != nil {
		return nil, err
	}
	network := "default"
	if properties["libvirt_network"] != "" {
		network = properties["libvirt_network"]
	}

	ssh_key_file := ""
	if properties["libvirt_ssh_key_file"] != "" {
		ssh_key_file = properties["libvirt_ssh_key_file"]
	}

	storage := "default"
	if properties["libvirt_storage"] != "" {
		storage = properties["libvirt_storage"]
	}

	uri := "qemu+ssh://root@192.168.122.1/system?no_verify=1"
	if properties["libvirt_uri"] != "" {
		uri = properties["libvirt_uri"]
	}

	vol_name := "podvm-base.qcow2"
	if properties["libvirt_vol_name"] != "" {
		vol_name = properties["libvirt_vol_name"]
	}

	conn_uri := "qemu:///system"
	if properties["libvirt_conn_uri"] != "" {
		conn_uri = properties["libvirt_conn_uri"]
	}
	conn, err := libvirt.NewConnect(conn_uri)
	if err != nil {
		return nil, err
	}

	clusterName := "peer-pods"
	if properties["cluster_name"] != "" {
		clusterName = properties["cluster_name"]
	}

	tunnelType := ""
	if properties["tunnel_type"] != "" {
		tunnelType = properties["tunnel_type"]
	}

	vxlanPort := ""
	if properties["vxlan_port"] != "" {
		vxlanPort = properties["vxlan_port"]
	}

	initdata := ""
	if properties["INITDATA"] != "" {
		initdata = properties["INITDATA"]
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		caa_image:        properties["CAA_IMAGE"],
		conn:             conn,
		containerRuntime: properties["container_runtime"],
		network:          network,
		ssh_key_file:     ssh_key_file,
		storage:          storage,
		uri:              uri,
		wd:               wd,
		volumeName:       vol_name,
		clusterName:      clusterName,
		tunnelType:       tunnelType,
		vxlanPort:        vxlanPort,
		initdata:         initdata,
	}, nil
}

func (l *LibvirtProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {

	cmd := exec.Command("/bin/bash", "-c", "./kcli_cluster.sh create")
	cmd.Dir = l.wd
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CLUSTER_NAME="+l.clusterName)
	cmd.Env = append(cmd.Env, "LIBVIRT_NETWORK="+l.network)
	cmd.Env = append(cmd.Env, "LIBVIRT_POOL="+l.storage)
	err := cmd.Run()
	if err != nil {
		return err
	}

	clusterName := l.clusterName
	home, _ := os.UserHomeDir()
	kubeconfig := path.Join(home, ".kcli/clusters", clusterName, "auth/kubeconfig")
	cfg.WithKubeconfigFile(kubeconfig)

	if err := pv.AddNodeRoleWorkerLabel(ctx, clusterName, cfg); err != nil {

		return fmt.Errorf("labeling nodes: %w", err)
	}

	return nil
}

func (l *LibvirtProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	// TODO: create a temporary Network and storage pool to use on
	// the tests.
	var (
		sPool *libvirt.StoragePool
		err   error
	)

	if _, err := l.conn.LookupNetworkByName(l.network); err != nil {
		return fmt.Errorf("Network '%s' not found. It should be created beforehand", l.network)
	}

	if sPool, err = l.conn.LookupStoragePoolByName(l.storage); err != nil {
		return fmt.Errorf("Storage pool '%s' not found. It should be created beforehand", l.storage)
	}

	// Create two volumes to test the multiple podvm image scenario.
	lVolumes := [2]string{l.volumeName, AlternateVolumeName}

	// Create the podvm storage volumes if it does not exist.
	for _, volume := range lVolumes {
		if _, err = sPool.LookupStorageVolByName(volume); err != nil {
			volCfg := libvirtxml.StorageVolume{
				Name: volume,
				Capacity: &libvirtxml.StorageVolumeSize{
					Unit:  "GiB",
					Value: 20,
				},
				Allocation: &libvirtxml.StorageVolumeSize{
					Unit:  "GiB",
					Value: 2,
				},
				Target: &libvirtxml.StorageVolumeTarget{
					Format: &libvirtxml.StorageVolumeTargetFormat{
						Type: "qcow2",
					},
				},
			}
			xml, err := volCfg.Marshal()
			if err != nil {
				return err
			}
			if _, err = sPool.StorageVolCreateXML(xml, libvirt.STORAGE_VOL_CREATE_PREALLOC_METADATA); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *LibvirtProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	cmd := exec.Command("/bin/bash", "-c", "./kcli_cluster.sh delete")
	cmd.Dir = l.wd
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (l *LibvirtProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	// TODO: delete the resources created on CreateVPC() that currently only checks
	// the Libvirt's storage and network exist.
	return nil
}

func (l *LibvirtProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"CAA_IMAGE":         l.caa_image,
		"CONTAINER_RUNTIME": l.containerRuntime,
		"network":           l.network,
		"podvm_volume":      l.volumeName,
		"ssh_key_file":      l.ssh_key_file,
		"storage":           l.storage,
		"uri":               l.uri,
		"tunnel_type":       l.tunnelType,
		"vxlan_port":        l.vxlanPort,
		"INITDATA":          l.initdata,
	}
}

func (l *LibvirtProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Trace("UploadPodvm()")

	sPool, err := l.GetStoragePool()
	if err != nil {
		return err
	}

	fileStat, err := os.Stat(imagePath)
	if err != nil {
		return err
	}
	length := fileStat.Size()

	lVolumes := [2]string{l.volumeName, AlternateVolumeName}

	for _, volume := range lVolumes {
		sVol, err := sPool.LookupStorageVolByName(volume)
		if err != nil {
			return err
		}

		stream, err := l.conn.NewStream(0)
		if err != nil {
			return err
		}

		if err := sVol.Upload(stream, 0, uint64(length), libvirt.STORAGE_VOL_UPLOAD_SPARSE_STREAM); err != nil {
			return err
		}

		fileByteSlice, err := os.ReadFile(imagePath)
		if err != nil {
			return err
		}

		sent := 0
		source := func(stream *libvirt.Stream, nbytes int) ([]byte, error) {
			tosend := nbytes
			if tosend > (len(fileByteSlice) - sent) {
				tosend = len(fileByteSlice) - sent
			}

			if tosend == 0 {
				return []byte{}, nil
			}

			data := fileByteSlice[sent : sent+tosend]
			sent += tosend

			return data, nil
		}

		if err := stream.SendAll(source); err != nil {
			return err
		}

		if err := stream.Finish(); err != nil {
			return err
		}

		if err := stream.Free(); err != nil {
			return err
		}
	}

	return nil
}

func (l *LibvirtProvisioner) GetStoragePool() (*libvirt.StoragePool, error) {
	sp, err := l.conn.LookupStoragePoolByName(l.storage)
	if err != nil {
		return nil, fmt.Errorf("Storage pool '%s' not found. It should be created beforehand", l.storage)
	}

	return sp, nil
}

func NewLibvirtInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		return nil, err
	}

	return &LibvirtInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (lio *LibvirtInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Apply(ctx, cfg)
}

func (lio *LibvirtInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Delete(ctx, cfg)
}

// Update install/overlays/libvirt/kustomization.yaml
func (lio *LibvirtInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error

	// Mapping the internal properties to ConfigMapGenerator properties and their default values.
	mapProps := map[string][2]string{
		"network":      {"default", "LIBVIRT_NET"},
		"storage":      {"default", "LIBVIRT_POOL"},
		"pause_image":  {"", "PAUSE_IMAGE"},
		"podvm_volume": {"", "LIBVIRT_VOL_NAME"},
		"uri":          {"qemu+ssh://root@192.168.122.1/system?no_verify=1", "LIBVIRT_URI"},
		"tunnel_type":  {"", "TUNNEL_TYPE"},
		"vxlan_port":   {"", "VXLAN_PORT"},
		"INITDATA":     {"", "INITDATA"},
	}

	for k, v := range mapProps {
		if properties[k] != v[0] {
			if err = lio.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v[1], properties[k]); err != nil {
				return err
			}
		}
	}

	if properties["ssh_key_file"] != "" {
		if err = lio.Overlay.SetKustomizeSecretGeneratorFile("ssh-key-secret",
			properties["ssh_key_file"]); err != nil {
			return err
		}
	}

	if err = lio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}

func NewLibvirtInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &LibvirtInstallChart{
		Helm: helm,
	}, nil
}

func (l *LibvirtInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return l.Helm.Install(ctx, cfg)
}

func (l *LibvirtInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return l.Helm.Uninstall(ctx, cfg)
}

func (l *LibvirtInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	if properties["CAA_IMAGE"] != "" {
		img := strings.Split(properties["CAA_IMAGE"], ":")
		imageNameProp := "image.name"
		log.Printf("Configuring helm: override value (%s=%s)", imageNameProp, img[0])
		l.Helm.OverrideValues[imageNameProp] = img[0]
		if len(img) == 2 {
			imageTagProp := "image.tag"
			l.Helm.OverrideValues[imageTagProp] = img[1]
		}
	}

	if properties["CONTAINER_RUNTIME"] == "crio" {
		prop := "kata-deploy.snapshotter.setup"
		l.Helm.OverrideValues[prop] = ""
	}

	// Mapping the internal properties to Helm chart values.
	mapProps := map[string]string{
		"network":      "LIBVIRT_NET",
		"pause_image":  "PAUSE_IMAGE",
		"podvm_volume": "LIBVIRT_VOL_NAME",
		"storage":      "LIBVIRT_POOL",
		"uri":          "LIBVIRT_URI",
		"tunnel_type":  "TUNNEL_TYPE",
		"vxlan_port":   "VXLAN_PORT",
		"INITDATA":     "INITDATA",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			l.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	if properties["ssh_key_file"] != "" {
		// Create the secret from the file created in config_libvirt.sh
		secretFile := properties["ssh_key_file"]
		secretName := "ssh-key-secret"
		if err := l.createSSHKeySecret(ctx, cfg, secretFile, secretName); err != nil {
			return err
		}

		// Set the secret name on the helm override
		l.Helm.OverrideValues["secrets.mode"] = "reference"
		l.Helm.OverrideValues["secrets.existingSshKeySecretName"] = secretName
	}
	return nil
}

// createSSHKeySecret creates the ssh-key-secret for libvirt.
// NOTE: Helm deals with secret properties, but this particular one is outside
// its scope for now. We need to create it manually while our Helm template
// doesn't have a mechanism to properly inject secrets (respecting the backend types).
func (l *LibvirtInstallChart) createSSHKeySecret(ctx context.Context, cfg *envconf.Config, sshKeyFile, secretName string) error {
	if sshKeyFile == "" {
		return nil
	}

	// Create namespace first if it doesn't exist
	if err := pv.CreateAndWaitForNamespace(ctx, cfg.Client(), l.Helm.Namespace); err != nil {
		return fmt.Errorf("failed to create Namespace: %w", err)
	}

	// TODO: Once we have removed the kustomize flow - update config_libvirt.sh to write
	// the full path of the ssh-key created and then remove this ~/.ssh offset
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	sshKeyPath := filepath.Join(homeDir, ".ssh", sshKeyFile)

	// TODO rewrite this to use go's k8s framework once stable
	args := []string{
		"create", "secret", "generic", secretName,
		"--from-file=id_rsa=" + sshKeyPath,
		"-n", l.Helm.Namespace,
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return fmt.Errorf("failed to create ssh-key-secret: %w, output: %s", err, string(stdoutStderr))
	}
	log.Infof("Created ssh-key-secret from %s", sshKeyPath)
	return nil
}
