//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/libvirt] ", log.LstdFlags|log.Lmsgprefix)

const maxInstanceNameLen = 63

type libvirtProvider struct {
	libvirtClient *libvirtClient
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {

	logger.Printf("libvirt config: %#v", config)

	libvirtClient, err := NewLibvirtClient(*config)
	if err != nil {
		logger.Printf("Unable to create libvirt connection: %v", err)
		return nil, err
	}

	provider := &libvirtProvider{
		libvirtClient: libvirtClient,
		serviceConfig: config,
	}

	return provider, nil
}

func getIPs(instance *vmConfig) ([]netip.Addr, error) {
	return instance.ips, nil
}

func (p *libvirtProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	var instanceMemory uint
	var instanceVCPUs uint
	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	if spec.Memory != 0 {
		instanceMemory = uint(spec.Memory)
	} else {
		instanceMemory = uint(p.serviceConfig.Memory)
	}

	if spec.VCPUs != 0 {
		instanceVCPUs = uint(spec.VCPUs)
	} else {
		instanceVCPUs = uint(p.serviceConfig.CPU)
	}

	// TODO: Specify the maximum instance name length in Libvirt
	vm := &vmConfig{name: instanceName, cpu: instanceVCPUs, mem: instanceMemory, userData: userData, firmware: p.serviceConfig.Firmware, cpuset: p.serviceConfig.CPUSet}

	if p.serviceConfig.DisableCVM {
		vm.launchSecurityType = NoLaunchSecurity
	} else if p.serviceConfig.LaunchSecurity != "" {
		switch p.serviceConfig.LaunchSecurity {
		case "s390-pv":
			vm.launchSecurityType = S390PV
		default:
			return nil, fmt.Errorf("[%s] is not a known launch security setting", p.serviceConfig.LaunchSecurity)
		}
	} else {
		vm.launchSecurityType, err = GetLaunchSecurityType(p.serviceConfig.URI)
		if err != nil {
			logger.Printf("unable to determine launch security type [%v]", err)
			return nil, err
		}
	}
	logger.Printf("LaunchSecurityType: %s", vm.launchSecurityType.String())

	if spec.Image != "" {
		logger.Printf("Choosing %s as libvirt volume for the PodVM image", spec.Image)
		p.libvirtClient.volName = spec.Image
	} else if spec.Image == "" && p.serviceConfig.VolName != p.libvirtClient.volName {
		logger.Printf("Choosing the default %s as libvirt volume for the PodVM image", p.serviceConfig.VolName)
		p.libvirtClient.volName = p.serviceConfig.VolName
	}

	result, err := CreateDomain(ctx, p.libvirtClient, vm)
	if err != nil {
		logger.Printf("failed to create an instance : %v", err)
		return nil, err
	}

	instanceID := result.instance.instanceId

	logger.Printf("created an instance %s for sandbox %s", result.instance.name, sandboxID)

	//Get Libvirt VM IP
	ips, err := getIPs(result.instance)
	if err != nil {
		logger.Printf("failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	instance := &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *libvirtProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	err := DeleteDomain(ctx, p.libvirtClient, instanceID)
	if err != nil {
		logger.Printf("failed to delete instance : %v", err)
		return err
	}
	logger.Printf("deleted an instance %s", instanceID)
	return nil

}

func (p *libvirtProvider) Teardown() error {
	return nil
}

func (p *libvirtProvider) ConfigVerifier() error {
	VolName := p.serviceConfig.VolName
	if len(VolName) == 0 {
		return fmt.Errorf("VolName is empty")
	}
	return nil
}
