//go:build libvirt

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"log"
	"net"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/libvirt] ", log.LstdFlags|log.Lmsgprefix)

const maxInstanceNameLen = 63

type libvirtProvider struct {
	libvirtClient *libvirtClient
	serviceConfig *Config
}

func NewProvider(config *Config) (cloud.Provider, error) {

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

func getIPs(instance *vmConfig) ([]net.IP, error) {
	return instance.ips, nil
}

func (p *libvirtProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// TODO: Specify the maximum instance name length in Libvirt
	vm := &vmConfig{name: instanceName, userData: userData}
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

	instance := &cloud.Instance{
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
