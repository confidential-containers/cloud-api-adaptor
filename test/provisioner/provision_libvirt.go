//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func init() {
	newProvisionerFunctions["libvirt"] = NewLibvirtProvisioner
	newInstallOverlayFunctions["libvirt"] = NewLibvirtInstallOverlay
}

// LibvirtProvisioner implements the CloudProvisioner interface for Libvirt.
type LibvirtProvisioner struct {
	conn         *libvirt.Connect // Libvirt connection
	network      string           // Network name
	ssh_key_file string           // SSH key file used to connect to Libvirt
	storage      string           // Storage pool name
	uri          string           // Libvirt URI
	wd           string           // libvirt's directory path on this repository
	volumeName   string           // Podvm volume name
	clusterName  string           // Cluster name
}

// LibvirtInstallOverlay implements the InstallOverlay interface
type LibvirtInstallOverlay struct {
	overlay *KustomizeOverlay
}

func NewLibvirtProvisioner(properties map[string]string) (CloudProvisioner, error) {
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

	// TODO: accept a different URI.
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, err
	}

	clusterName := "peer-pods"
	if properties["cluster_name"] != "" {
		clusterName = properties["cluster_name"]
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		conn:         conn,
		network:      network,
		ssh_key_file: ssh_key_file,
		storage:      storage,
		uri:          uri,
		wd:           wd,
		volumeName:   vol_name,
		clusterName:  clusterName,
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

	if err := AddNodeRoleWorkerLabel(ctx, clusterName, cfg); err != nil {

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

	// Create the podvm storage volume if it does not exist.
	if _, err = sPool.LookupStorageVolByName(l.volumeName); err != nil {
		volCfg := libvirtxml.StorageVolume{
			Name: l.volumeName,
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
		"network":      l.network,
		"podvm_volume": l.volumeName,
		"ssh_key_file": l.ssh_key_file,
		"storage":      l.storage,
		"uri":          l.uri,
	}
}

func (l *LibvirtProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	// TODO: convert to use the libvirt.org/go/libvirt API.
	//sPool, err := l.GetStoragePool()
	//if err != nil {
	//	return err
	//}

	//sVol, err := sPool.LookupStorageVolByName(l.volumeName)
	//if err != nil {
	//	return err
	//}

	//err = sVol.Upload(stream *Stream, 0, length uint64, libvirt.STORAGE_VOL_UPLOAD_SPARSE_STREAM)
	//if err != nil {
	//	return err
	//}

	//n, _ := sVol.GetName()

	//fmt.Printf("%s\n", n)
	cmd := exec.Command("/bin/bash", "-c",
		fmt.Sprintf("virsh -c qemu:///system vol-upload --vol %s %s --pool default --sparse", l.volumeName, imagePath))
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

func (l *LibvirtProvisioner) GetStoragePool() (*libvirt.StoragePool, error) {
	sp, err := l.conn.LookupStoragePoolByName(l.storage)
	if err != nil {
		return nil, fmt.Errorf("Storage pool '%s' not found. It should be created beforehand", l.storage)
	}

	return sp, nil
}

func NewLibvirtInstallOverlay(installDir string) (InstallOverlay, error) {
	overlay, err := NewKustomizeOverlay(filepath.Join(installDir, "overlays/libvirt"))
	if err != nil {
		return nil, err
	}

	return &LibvirtInstallOverlay{
		overlay: overlay,
	}, nil
}

func (lio *LibvirtInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Apply(ctx, cfg)
}

func (lio *LibvirtInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Delete(ctx, cfg)
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
		"vxlan_port":   {"", "VXLAN_PORT"},
	}

	for k, v := range mapProps {
		if properties[k] != v[0] {
			if err = lio.overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v[1], properties[k]); err != nil {
				return err
			}
		}
	}

	if properties["ssh_key_file"] != "" {
		if err = lio.overlay.SetKustomizeSecretGeneratorFile("ssh-key-secret",
			properties["ssh_key_file"]); err != nil {
			return err
		}
	}

	if err = lio.overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
