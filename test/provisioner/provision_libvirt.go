//go:build libvirt

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// LibvirtProvisioner implements the CloudProvision interface for Libvirt.
type LibvirtProvisioner struct {
	conn       *libvirt.Connect // Libvirt connection
	network    string           // Network name
	storage    string           // Storage pool name
	wd         string           // libvirt's directory path on this repository
	volumeName string           // Podvm volume name
}

func NewLibvirtProvisioner(properties map[string]string) (*LibvirtProvisioner, error) {
	wd, err := filepath.Abs(path.Join("..", "..", "libvirt"))
	if err != nil {
		return nil, err
	}

	network := "default"
	if properties["libvirt_network"] != "" {
		network = properties["libvirt_network"]
	}

	storage := "default"
	if properties["libvirt_storage"] != "" {
		storage = properties["libvirt_storage"]
	}

	// TODO: accept a different URI.
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, err
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		conn:       conn,
		network:    network,
		storage:    storage,
		wd:         wd,
		volumeName: "podvm-base.qcow2",
	}, nil
}

func (l *LibvirtProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	cmd := exec.Command("/bin/bash", "-c", "./kcli_cluster.sh create")
	cmd.Dir = l.wd
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	// TODO: cluster name should be customized.
	clusterName := "peer-pods"
	home, _ := os.UserHomeDir()
	kubeconfig := path.Join(home, ".kcli/clusters", clusterName, "auth/kubeconfig")
	cfg.WithKubeconfigFile(kubeconfig)

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
