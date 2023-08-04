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

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
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
	var (
		sPool *libvirt.StoragePool
		err   error
	)

	if _, err := l.conn.LookupNetworkByName(l.network); err != nil {
		// Check if network is defined, but not enabled
		definedNets, err := l.conn.ListDefinedNetworks()
		if err != nil {
			return err
		}
		if slices.Contains(definedNets, l.network) {
			return fmt.Errorf("%s is defined but not enabled. Change to an enabled network, or to create a temporary network, use an undefined network name", l.network)
		}
		log.Printf("Network %s is not defined. Creating a new temporary network", l.network)

		// Try subnets from x.x.122.x-x.x.255.x
		subnetBlock := 122
		for ; subnetBlock < 256; subnetBlock++ {
			testAddr := fmt.Sprintf("192.168.%d.1", subnetBlock)
			testDHCPStart := fmt.Sprintf("192.168.%d.128", subnetBlock)
			testDHCPEnd := fmt.Sprintf("192.168.%d.254", subnetBlock)

			networkCfg := &libvirtxml.Network{
				Name: l.network,
				Forward: &libvirtxml.NetworkForward{
					Mode: "nat",
				},
				IPs: []libvirtxml.NetworkIP{
					{
						Address: testAddr,
						DHCP: &libvirtxml.NetworkDHCP{
							Ranges: []libvirtxml.NetworkDHCPRange{
								{
									Start: testDHCPStart,
									End:   testDHCPEnd,
								},
							},
						},
						Netmask: "255.255.255.0",
					},
				},
			}

			networkXML, err := networkCfg.Marshal()
			if err != nil {
				return fmt.Errorf("Failed to create temp network XML: %s", err)
			}

			_, err = l.conn.NetworkCreateXML(networkXML)
			if err == nil {
				log.Printf("Using 192.168.%d.1 as the network ip for %s", subnetBlock, l.network)
				break
			}
		}
		if subnetBlock == 256 {
			return fmt.Errorf("Unable to allocate a network ip address. Failed to create temporary network.")
		}

	}

	if sPool, err = l.conn.LookupStoragePoolByName(l.storage); err != nil {
		definedStorage, err := l.conn.ListDefinedStoragePools()
		if err != nil {
			return err
		}
		if slices.Contains(definedStorage, l.storage) {
			return fmt.Errorf("%s is defined but not enabled. Change to an enabled storage pool, or to create a temporary storage pool, use an undefined network name", l.storage)
		}
		log.Printf("Storage %s is not defined. Creating a new temporary storage pool", l.storage)

		dirPath, err := os.MkdirTemp("", "temp-images")
		if err != nil {
			return err
		}

		poolCfg := &libvirtxml.StoragePool{
			Type: "dir",
			Name: l.storage,
			Target: &libvirtxml.StoragePoolTarget{
				Path: dirPath,
				Permissions: &libvirtxml.StoragePoolTargetPermissions{
					Mode: "0771",
				},
			},
		}
		poolXML, err := poolCfg.Marshal()
		if err != nil {
			return fmt.Errorf("Failed to create temp pool XML: %s", err)
		}

		sPool, err = l.conn.StoragePoolCreateXML(poolXML, libvirt.STORAGE_POOL_CREATE_WITH_BUILD)

		if err != nil {
			return fmt.Errorf("Unable to create temporary storage pool. Please create one beforehand. %s", err)
		}
		log.Printf("Created temporary Pool %s", l.storage)
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
	log.Trace("DeleteVPC()")
	network, err := l.conn.LookupNetworkByName(l.network)
	if err == nil {

		persistent, err := network.IsPersistent()

		if err != nil {
			log.Errorf("Cannot determine whether network %s is persistent", l.network)
		} else {
			if !persistent {
				if err := network.Destroy(); err != nil {
					log.Errorf("Failed to destroy network %s, %s", l.network, err)
				} else {
					log.Printf("Destroyed temp network %s", l.network)
				}
			}
		}
		if err = network.Free(); err != nil {
			log.Errorf("Failed to free network pointer %s, %s", l.storage, err)
		} else {
			log.Printf("Freed network pointer %s", l.storage)
		}
	}

	storage, err := l.conn.LookupStoragePoolByName(l.storage)
	if err == nil {
		persistent, err := storage.IsPersistent()
		if err != nil {
			log.Errorf("Cannot determine whether storage %s is persistent", l.storage)
		} else {
			if !persistent {
				// Delete Storage function only works on inactive storage volumes
				// Transient storage volumes can only be active afaik
				// Going to Delete each of the volumes instead
				volumes, err := storage.ListAllStorageVolumes(0)

				if err != nil {
					log.Errorf("Failed to delete volumes associated with temporary pool %s", l.storage)
				} else {
					for _, v := range volumes {
						err = v.Delete(libvirt.STORAGE_VOL_DELETE_NORMAL)
						if err != nil {
							log.Errorf("Unable to delete volume: %s", err)
						}
					}
					log.Printf("Destroyed volumes associated with %s", l.storage)
				}

				if err := storage.Destroy(); err != nil {
					log.Errorf("Failed to destroy storage %s, %s", l.storage, err)

				} else {
					log.Printf("Destroyed temp storage %s", l.storage)
				}
			}
		}
		if err = storage.Free(); err != nil {
			log.Errorf("Failed to free storage pointer %s, %s", l.storage, err)
		} else {
			log.Printf("Freed storage pointer %s", l.storage)
		}
	}
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
