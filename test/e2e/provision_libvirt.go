//go:build libvirt
package e2e

import (
	"context"
	"fmt"
	"libvirt.org/go/libvirt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// LibvirtProvisioner implements the CloudProvision interface for Libvirt.
type LibvirtProvisioner struct {
	conn    *libvirt.Connect // Libvirt connection
	network string           // Network name
	storage string           // Storage pool name
	wd      string           // libvirt's directory path on this repository
}

func NewLibvirtProvisioner(network string, storage string) (*LibvirtProvisioner, error) {
	wd, err := filepath.Abs(path.Join("..", "..", "libvirt"))
	if err != nil {
		return nil, err
	}

	// TODO: accept a different URI.
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, err
	}

	// TODO: Check network and storage are not nil?
	return &LibvirtProvisioner{
		conn:    conn,
		network: network,
		storage: storage,
		wd:      wd,
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

	if _, err := l.conn.LookupNetworkByName(l.network); err != nil {
		return fmt.Errorf("Network '%s' not found. It should be created beforehand", l.network)
	}

	if _, err := l.conn.LookupStoragePoolByName(l.storage); err != nil {
		return fmt.Errorf("Storage pool '%s' not found. It should be created beforehand", l.storage)
	}

	return nil
}

func GetCloudProvisioner() (CloudProvision, error) {
	return NewLibvirtProvisioner("default", "default")
}
