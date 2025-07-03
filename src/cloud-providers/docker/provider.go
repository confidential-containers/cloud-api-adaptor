// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	putil "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/docker/docker/client"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/docker] ", log.LstdFlags|log.Lmsgprefix)

type dockerProvider struct {
	Client           *client.Client
	DataDir          string
	PodVMDockerImage string
	NetworkName      string
}

const maxInstanceNameLen = 63

func NewProvider(config *Config) (*dockerProvider, error) {

	logger.Printf("docker config: %#v", config)

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	// Create the data directory if it doesn't exist
	err = os.MkdirAll(config.DataDir, 0755)
	if err != nil {
		return nil, err
	}

	return &dockerProvider{
		Client:           cli,
		DataDir:          config.DataDir,
		PodVMDockerImage: config.PodVMDockerImage,
		NetworkName:      config.NetworkName,
	}, nil
}

func (p *dockerProvider) CreateInstance(ctx context.Context, podName, sandboxID string,
	cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	instanceName := putil.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	logger.Printf("CreateInstance: name: %q", instanceName)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}
	// Write userdata to a file named after the instance name in the data directory
	// File name: $data-dir/instanceName-userdata
	// File content: userdata
	instanceUserdataFile, err := provider.WriteUserData(instanceName, userData, p.DataDir)
	if err != nil {
		return nil, err
	}

	// Create volume binding for the container

	// mount userdata to /media/cidata/user-data
	// This file will be read by process-user-data and apf.json will be written to
	// /run/peerpods/apf.json at runtime
	volumeBinding := []string{
		// note: we are not importing that path from the CAA package to avoid circular dependencies
		fmt.Sprintf("%s:%s", instanceUserdataFile, "/media/cidata/user-data"),
	}

	// Add host bind mount for /run/kata-containers and /run/image to avoid
	// overlay on overlay issue
	// (host)kata-containers dir -> (container) /run/kata-containers
	volumeBinding = append(volumeBinding, fmt.Sprintf("%s:%s",
		filepath.Join(p.DataDir, "kata-containers"), "/run/kata-containers"))

	// Add host bind mounts required by iptables
	volumeBinding = append(volumeBinding, fmt.Sprintf("%s:%s", "/lib/modules", "/lib/modules"))
	volumeBinding = append(volumeBinding, fmt.Sprintf("%s:%s", "/run/xtables.lock", "/run/xtables.lock"))

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the docker image for the PodVM image", spec.Image)
		p.PodVMDockerImage = spec.Image
	}

	// (host)image dir -> (container) /image
	// There is a podvm systemd service in pod which bind mounts /run/image to /image
	volumeBinding = append(volumeBinding, fmt.Sprintf("%s:%s",
		filepath.Join(p.DataDir, "image"), "/image"))

	instanceID, ip, err := createContainer(ctx, p.Client, instanceName, volumeBinding,
		p.PodVMDockerImage, p.NetworkName)
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: instanceID: %q, ip: %q", instanceID, ip)

	// Convert ip to []netip.Addr
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, err
	}

	return &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  []netip.Addr{ipAddr}, // Convert ipAddr to a slice of netip.Addr
	}, nil

}

func (p *dockerProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	logger.Printf("DeleteInstance: instanceID: %q", instanceID)

	// Delete the container
	err := deleteContainer(ctx, p.Client, instanceID)
	if err != nil {
		return err
	}

	return nil
}

func (p *dockerProvider) Teardown() error {
	return nil
}

func (p *dockerProvider) ConfigVerifier() error {
	return nil
}
