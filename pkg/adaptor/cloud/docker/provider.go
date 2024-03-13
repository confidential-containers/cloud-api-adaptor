package docker

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"github.com/docker/docker/client"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/docker] ", log.LstdFlags|log.Lmsgprefix)

type dockerProvider struct {
	Client  *client.Client
	DataDir string
}

const maxInstanceNameLen = 63

func NewProvider(config *Config) (*dockerProvider, error) {

	logger.Printf("docker config: %#v", config)

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &dockerProvider{
		Client:  cli,
		DataDir: config.DataDir,
	}, nil
}

func (p *dockerProvider) CreateInstance(ctx context.Context, podName, sandboxID string,
	cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	logger.Printf("CreateInstance: name: %q", instanceName)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}
	// Write userdata to a file named after the instance name in the data directory
	// File name: $data-dir/instanceName-userdata.json
	// File content: userdata
	instanceUserdataFile, err := util.WriteUserData(instanceName, userData, p.DataDir)
	if err != nil {
		return nil, err
	}

	// Create volume binding for the container
	// mount userdata to DockerUserDataUrl=/peerpod/daemon.json
	volumeBinding := fmt.Sprintf("%s:%s", instanceUserdataFile, DockerUserDataUrl)

	instanceID, ip, err := createContainer(ctx, p.Client, instanceName, []string{volumeBinding})
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: instanceID: %q, ip: %q", instanceID, ip)

	// Convert ip to []netip.Addr
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, err
	}

	return &cloud.Instance{
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
