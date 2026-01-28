// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

// Initialize logger for OpenStack provider
var logger = log.New(log.Writer(), "[adaptor/cloud/openstack] ", log.LstdFlags|log.Lmsgprefix)

// Maximum length for instance names
const maxInstanceNameLen = 63

// openstackProvider implements the Provider interface for OpenStack
type openstackProvider struct {
	providerClient *gophercloud.ProviderClient
	computeClient  *gophercloud.ServiceClient
	networkClient  *gophercloud.ServiceClient
	serviceConfig  *Config
	floatingIPPool map[string]string
}

// NewProvider creates a new OpenStack provider.
func NewProvider(config *Config) (provider.Provider, error) {

	// Use config.Redact() to customize log output and hide sensitive information
	logger.Printf("openstack config: %+v", config.Redact())

	providerClient, err := NewProviderClient(*config)
	if err != nil {
		logger.Printf("unable to create openstack provider client: %v", err)
		return nil, err
	}

	computeClient, err := NewComputeClient(providerClient, gophercloud.EndpointOpts{
		Region: config.Region,
	})
	if err != nil {
		logger.Printf("unable to create openstack compute client: %v", err)
		return nil, err
	}

	networkClient, err := NewNetworkClient(providerClient, gophercloud.EndpointOpts{
		Region: config.Region,
	})
	if err != nil {
		err = fmt.Errorf("unable to create openstack network client: %v", err)
		return nil, err
	}

	return &openstackProvider{
		providerClient: providerClient,
		computeClient:  computeClient,
		networkClient:  networkClient,
		serviceConfig:  config,
		floatingIPPool: make(map[string]string),
	}, nil
}

func (p *openstackProvider) CreateInstance(ctx context.Context, podname, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {
	instanceName := util.GenerateInstanceName(podname, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// gophercloud-provided struct
	// networks must be specified by their unique IDs
	createOpts := servers.CreateOpts{
		Name:           instanceName,
		ImageRef:       p.serviceConfig.ImageID,
		FlavorRef:      p.serviceConfig.FlavorID,
		SecurityGroups: p.serviceConfig.SecurityGroups,
		UserData:       []byte(cloudConfigData),
	}
	// Make network UUID list.
	networkList := MakeNetworkList(p.serviceConfig.NetworkIDs)

	if len(networkList) != 0 {
		createOpts.Networks = networkList
	} else {
		createOpts.Networks = "auto"
	}

	// Specify any scheduler hints if needed
	schedulerHintOpts := servers.SchedulerHintOpts{}

	server, err := servers.Create(ctx, p.computeClient, createOpts, schedulerHintOpts).Extract()
	if err != nil {
		return nil, err
	}

	ips, err := GetFixedIPs(ctx, p.computeClient, server.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract IPs from server addresses: %w", err)
	}

	if len(ips) != 0 {
		portID := GetPortID(p.computeClient, server.ID, ips[0].String())

		// Assign a floating IP if configured
		if p.serviceConfig.FloatingIpNetworkID != "" && portID != "" {
			fip, fid, err := AssignFloatingIP(ctx, p.networkClient, portID, p.serviceConfig.FloatingIpNetworkID)
			if err != nil {
				return nil, fmt.Errorf("failed to assign floating IP: %w", err)
			}
			p.floatingIPPool[server.ID] = fid

			// prepend floating IP to the IP list
			ips = append([]netip.Addr{fip}, ips...)
		}
	}

	instance := &provider.Instance{
		ID:   server.ID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *openstackProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	logger.Printf("Deleting instance: %s", instanceID)

	// if a floating IP was assigned, release it
	floatingIPID, existsFloatingIP := p.floatingIPPool[instanceID]
	if existsFloatingIP {
		DeleteFloatingIP(ctx, p.networkClient, floatingIPID)
		delete(p.floatingIPPool, instanceID)
	} else {
		logger.Printf("No floating IP assigned to instance: %s", instanceID)
	}

	// delete the instance
	err := servers.Delete(ctx, p.computeClient, instanceID).ExtractErr()
	if err != nil {
		// if the instance is already deleted
		if gophercloud.ResponseCodeIs(err, 404) {
			logger.Printf("Instance %s already deleted", instanceID)
			return nil
		}
		return err
	}
	logger.Printf("Successfully sent delete request for instance: %s", instanceID)
	return nil
}

func (p *openstackProvider) Teardown() error {
	return nil
}

func (p *openstackProvider) ConfigVerifier() error {
	if len(p.serviceConfig.ImageID) == 0 {
		return fmt.Errorf("imageID is empty")
	}
	return nil
}
