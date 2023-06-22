// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

const maxInstanceNameLen = 63

var logger = log.New(log.Writer(), "[adaptor/cloud/ibmcloud-powervs] ", log.LstdFlags|log.Lmsgprefix)

type ibmcloudPowerVSProvider struct {
	powervsService
	serviceConfig *Config
}

func NewProvider(config *Config) (cloud.Provider, error) {

	logger.Printf("ibmcloud-powervs config: %#v", config.Redact())

	powervs, err := newPowervsClient(config.ApiKey, config.ServiceInstanceID, config.Zone)
	if err != nil {
		return nil, err
	}

	return &ibmcloudPowerVSProvider{
		powervsService: *powervs,
		serviceConfig:  config,
	}, nil
}

func (p *ibmcloudPowerVSProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	body := &models.PVMInstanceCreate{
		ServerName:  &instanceName,
		ImageID:     &p.serviceConfig.ImageID,
		KeyPairName: p.serviceConfig.SSHKey,
		Networks: []*models.PVMInstanceAddNetwork{
			{
				NetworkID: &p.serviceConfig.NetworkID,
			}},
		Memory:     core.Float64Ptr(p.serviceConfig.Memory),
		Processors: core.Float64Ptr(p.serviceConfig.Processors),
		ProcType:   core.StringPtr(p.serviceConfig.ProcessorType),
		SysType:    p.serviceConfig.SystemType,
		UserData:   base64.StdEncoding.EncodeToString([]byte(userData)),
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	pvsInstances, err := p.powervsService.instanceClient(ctx).Create(body)
	if err != nil {
		logger.Printf("failed to create an instance : %v", err)
		return nil, err
	}

	if len(*pvsInstances) <= 0 {
		return nil, fmt.Errorf("there are no instances created")
	}

	ins := (*pvsInstances)[0]
	instanceID := *ins.PvmInstanceID

	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	logger.Printf("Waiting for instance to reach state: ACTIVE")
	err = retry.Do(
		func() error {
			in, err := p.powervsService.instanceClient(ctx).Get(*ins.PvmInstanceID)
			if err != nil {
				return fmt.Errorf("failed to get the instance: %v", err)
			}

			if *in.Status == "ERROR" {
				return fmt.Errorf("instance is in error state")
			}

			if *in.Status == "ACTIVE" {
				logger.Printf("instance is in desired state: %s", *in.Status)
				return nil
			}

			return fmt.Errorf("Instance failed to reach ACTIVE state")
		},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(5*time.Second),
	)

	if err != nil {
		logger.Print(err)
		return nil, err
	}

	ips, err := p.getVMIPs(ctx, ins)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPs for the instance : %v", err)
	}

	return &cloud.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}, nil
}

func (p *ibmcloudPowerVSProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	err := p.powervsService.instanceClient(ctx).Delete(instanceID)
	if err != nil {
		logger.Printf("failed to delete an instance: %v", err)
		return err
	}

	logger.Printf("deleted an instance %s", instanceID)
	return nil
}

func (p *ibmcloudPowerVSProvider) Teardown() error {
	return nil
}

func (p *ibmcloudPowerVSProvider) getVMIPs(ctx context.Context, instance *models.PVMInstance) ([]net.IP, error) {
	var ips []net.IP
	ins, err := p.powervsService.instanceClient(ctx).Get(*instance.PvmInstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get the instance: %v", err)
	}

	for i, network := range ins.Networks {
		if ins.Networks[i].Type == "fixed" {
			ip := net.ParseIP(network.IPAddress)
			if ip == nil {
				return nil, fmt.Errorf("failed to parse pod node IP: %q", network.IPAddress)
			}

			ips = append(ips, ip)
			logger.Printf("podNodeIP[%d]=%s", i, ip.String())
		}
	}

	if len(ips) > 0 {
		return ips, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 750*time.Second)
	defer cancel()

	// If IP is not assigned to the instance, fetch it from DHCP server
	logger.Printf("Trying to fetch IP from DHCP server..")
	err = retry.Do(func() error {
		ip, err := p.getFromDHCPServer(ctx, ins)
		if err != nil {
			logger.Print(err)
			return err
		}
		if ip == nil {
			return fmt.Errorf("failed to get IP from DHCP server: %v", err)
		}

		addr := net.ParseIP(*ip)
		if addr == nil {
			return fmt.Errorf("failed to parse pod node IP: %q", *ip)
		}

		ips = append(ips, addr)
		logger.Printf("podNodeIP=%s", addr.String())
		return nil
	},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(10*time.Second),
	)

	if err != nil {
		logger.Print(err)
		return nil, err
	}

	return ips, nil
}

func (p *ibmcloudPowerVSProvider) getFromDHCPServer(ctx context.Context, instance *models.PVMInstance) (*string, error) {
	networkID := p.serviceConfig.NetworkID

	var pvsNetwork *models.PVMInstanceNetwork
	for _, net := range instance.Networks {
		if net.NetworkID == networkID {
			pvsNetwork = net
		}
	}
	if pvsNetwork == nil {
		return nil, fmt.Errorf("failed to get network attached to instance")
	}

	dhcpServers, err := p.powervsService.dhcpClient(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get the DHCP servers: %v", err)
	}

	var dhcpServerDetails *models.DHCPServerDetail
	for _, server := range dhcpServers {
		if *server.Network.ID == networkID {
			dhcpServerDetails, err = p.powervsService.dhcpClient(ctx).Get(*server.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get DHCP server details: %v", err)
			}
			break
		}
	}

	if dhcpServerDetails == nil {
		return nil, fmt.Errorf("DHCP server associated with network is nil")
	}

	var ip *string
	for _, lease := range dhcpServerDetails.Leases {
		if *lease.InstanceMacAddress == pvsNetwork.MacAddress {
			ip = lease.InstanceIP
			break
		}
	}

	return ip, nil
}
