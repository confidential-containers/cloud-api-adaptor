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

const authJSONTemplate string = `{
	"auths": {
		"quay.io": {
			"auth": "%s"
		}
	}
}`

// LibvirtProvisioner implements the CloudProvisioner interface for Libvirt.
type LibvirtProvisioner struct {
	conn          *libvirt.Connect // Libvirt connection
	network       string           // Network name
	ssh_key_file  string           // SSH key file used to connect to Libvirt
	storage       string           // Storage pool name
	uri           string           // Libvirt URI
	wd            string           // libvirt's directory path on this repository
	volumeName    string           // Podvm volume name
	clusterName   string           // Cluster name
	kbs_image     string           // KBS Service OCI Image URL
	kbs_image_tag string           // KBS Service OCI Image Tag
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

	kbs_image := "ghcr.io/confidential-containers/key-broker-service"
	if properties["KBS_IMAGE"] != "" {
		kbs_image = properties["KBS_IMAGE"]
	}

	kbs_image_tag := "latest"
	if properties["KBS_IMAGE_TAG"] != "" {
		kbs_image_tag = properties["KBS_IMAGE_TAG"]
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		conn:          conn,
		network:       network,
		ssh_key_file:  ssh_key_file,
		storage:       storage,
		uri:           uri,
		wd:            wd,
		volumeName:    vol_name,
		clusterName:   clusterName,
		kbs_image:     kbs_image,
		kbs_image_tag: kbs_image_tag,
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
		"network":       l.network,
		"podvm_volume":  l.volumeName,
		"ssh_key_file":  l.ssh_key_file,
		"storage":       l.storage,
		"uri":           l.uri,
		"KBS_IMAGE":     l.kbs_image,
		"KBS_IMAGE_TAG": l.kbs_image_tag,
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

	sVol, err := sPool.LookupStorageVolByName(l.volumeName)
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
		"network":       {"default", "LIBVIRT_NET"},
		"storage":       {"default", "LIBVIRT_POOL"},
		"pause_image":   {"", "PAUSE_IMAGE"},
		"podvm_volume":  {"", "LIBVIRT_VOL_NAME"},
		"uri":           {"qemu+ssh://root@192.168.122.1/system?no_verify=1", "LIBVIRT_URI"},
		"vxlan_port":    {"", "VXLAN_PORT"},
		"AA_KBC_PARAMS": {"", "AA_KBC_PARAMS"},
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

	if cred := os.Getenv("REGISTRY_CREDENTIAL_ENCODED"); cred != "" {
		authJSON := fmt.Sprintf(authJSONTemplate, cred)
		if err := os.WriteFile(filepath.Join(lio.Overlay.ConfigDir, "auth.json"), []byte(authJSON), 0644); err != nil {
			return err
		}
		if err = lio.Overlay.SetKustomizeSecretGeneratorFile("auth-json-secret", "auth.json"); err != nil {
			return err
		}
	}

	if err = lio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
