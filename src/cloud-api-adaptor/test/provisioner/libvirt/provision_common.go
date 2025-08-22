// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"fmt"

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
	conn                 *libvirt.Connect // Libvirt connection
	containerRuntime     string           // Name of the container runtime
	network              string           // Network name
	sshKeyFile           string           // SSH key file used to connect to Libvirt
	storage              string           // Storage pool name
	uri                  string           // Libvirt URI
	wd                   string           // libvirt's directory path on this repository
	volumeName           string           // Podvm volume name
	clusterName          string           // Cluster name
	tunnelType           string           // Tunnel Type
	vxlanPort            string           // VXLAN port number
	secureComms          string           // Activate CAA SECURE_COMMS
	secureCommsNoTrustee string           // Deactivate Trustee mode in SECURE_COMMS
	secureCommsKBSAddr   string           // KBS URL
	initdata             string           // InitData
}

// LibvirtInstallOverlay implements the InstallOverlay interface
type LibvirtInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
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

	sshKeyFile := ""
	if properties["libvirt_ssh_key_file"] != "" {
		sshKeyFile = properties["libvirt_ssh_key_file"]
	}

	storage := "default"
	if properties["libvirt_storage"] != "" {
		storage = properties["libvirt_storage"]
	}

	uri := "qemu+ssh://root@192.168.122.1/system?no_verify=1"
	if properties["libvirt_uri"] != "" {
		uri = properties["libvirt_uri"]
	}

	volName := "podvm-base.qcow2"
	if properties["libvirt_vol_name"] != "" {
		volName = properties["libvirt_vol_name"]
	}

	connURI := "qemu:///system"
	if properties["libvirt_conn_uri"] != "" {
		connURI = properties["libvirt_conn_uri"]
	}
	conn, err := libvirt.NewConnect(connURI)
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

	secureComms := "false"
	if properties["SECURE_COMMS"] != "" {
		secureComms = properties["SECURE_COMMS"]
	}

	secureCommsKbsAddr := ""
	if properties["SECURE_COMMS_KBS_ADDR"] != "" {
		secureCommsKbsAddr = properties["SECURE_COMMS_KBS_ADDR"]
	}

	secureCommsNoTrustee := "false"
	if properties["SECURE_COMMS_NO_TRUSTEE"] != "" {
		secureCommsNoTrustee = properties["SECURE_COMMS_NO_TRUSTEE"]
	}

	initdata := ""
	if properties["INITDATA"] != "" {
		initdata = properties["INITDATA"]
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		conn:                 conn,
		containerRuntime:     properties["container_runtime"],
		network:              network,
		sshKeyFile:           sshKeyFile,
		storage:              storage,
		uri:                  uri,
		wd:                   wd,
		volumeName:           volName,
		clusterName:          clusterName,
		tunnelType:           tunnelType,
		vxlanPort:            vxlanPort,
		secureComms:          secureComms,
		secureCommsKBSAddr:   secureCommsKbsAddr,
		secureCommsNoTrustee: secureCommsNoTrustee,
		initdata:             initdata,
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
		return fmt.Errorf("network '%s' not found. It should be created beforehand", l.network)
	}

	if sPool, err = l.conn.LookupStoragePoolByName(l.storage); err != nil {
		return fmt.Errorf("storage pool '%s' not found. It should be created beforehand", l.storage)
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
		"CONTAINER_RUNTIME":       l.containerRuntime,
		"network":                 l.network,
		"podvm_volume":            l.volumeName,
		"ssh_key_file":            l.sshKeyFile,
		"storage":                 l.storage,
		"uri":                     l.uri,
		"tunnel_type":             l.tunnelType,
		"vxlan_port":              l.vxlanPort,
		"SECURE_COMMS":            l.secureComms,
		"SECURE_COMMS_KBS_ADDR":   l.secureCommsKBSAddr,
		"SECURE_COMMS_NO_TRUSTEE": l.secureCommsNoTrustee,
		"INITDATA":                l.initdata,
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
		return nil, fmt.Errorf("storage pool '%s' not found. It should be created beforehand", l.storage)
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
		"network":                 {"default", "LIBVIRT_NET"},
		"storage":                 {"default", "LIBVIRT_POOL"},
		"pause_image":             {"", "PAUSE_IMAGE"},
		"podvm_volume":            {"", "LIBVIRT_VOL_NAME"},
		"uri":                     {"qemu+ssh://root@192.168.122.1/system?no_verify=1", "LIBVIRT_URI"},
		"tunnel_type":             {"", "TUNNEL_TYPE"},
		"vxlan_port":              {"", "VXLAN_PORT"},
		"INITDATA":                {"", "INITDATA"},
		"SECURE_COMMS":            {"", "SECURE_COMMS"},
		"SECURE_COMMS_NO_TRUSTEE": {"", "SECURE_COMMS_NO_TRUSTEE"},
		"SECURE_COMMS_KBS_ADDR":   {"", "SECURE_COMMS_KBS_ADDR"},
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
