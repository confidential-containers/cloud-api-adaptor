// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

const (
	maxRetries    = 10
	queryInterval = 2
)

var logger = log.New(log.Writer(), "[adaptor/cloud/ibmcloud] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")

const maxInstanceNameLen = 63

type vpcV1 interface {
	CreateInstanceWithContext(context.Context, *vpcv1.CreateInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error)
	GetInstanceWithContext(context.Context, *vpcv1.GetInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error)
	DeleteInstanceWithContext(context.Context, *vpcv1.DeleteInstanceOptions) (*core.DetailedResponse, error)
}

type ibmcloudVPCProvider struct {
	vpc           vpcV1
	serviceConfig *Config
}

func NewProvider(config *Config) (cloud.Provider, error) {

	logger.Printf("ibmcloud-vpc config: %#v", config.Redact())

	vpcV1, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: config.ApiKey,
			URL:    config.IamServiceURL,
		},
		URL: config.VpcServiceURL,
	})

	if err != nil {
		return nil, err
	}

	provider := &ibmcloudVPCProvider{
		vpc:           vpcV1,
		serviceConfig: config,
	}

	return provider, nil
}

func (p *ibmcloudVPCProvider) getInstancePrototype(instanceName, userData string) *vpcv1.InstancePrototype {

	prototype := &vpcv1.InstancePrototype{
		Name:     &instanceName,
		Image:    &vpcv1.ImageIdentity{ID: &p.serviceConfig.ImageID},
		UserData: &userData,
		Profile:  &vpcv1.InstanceProfileIdentity{Name: &p.serviceConfig.ProfileName},
		Zone:     &vpcv1.ZoneIdentity{Name: &p.serviceConfig.ZoneName},
		Keys: []vpcv1.KeyIdentityIntf{
			&vpcv1.KeyIdentity{ID: &p.serviceConfig.KeyID},
		},
		VPC: &vpcv1.VPCIdentity{ID: &p.serviceConfig.VpcID},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfacePrototype{
			Subnet: &vpcv1.SubnetIdentity{ID: &p.serviceConfig.PrimarySubnetID},
			SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
				&vpcv1.SecurityGroupIdentityByID{ID: &p.serviceConfig.PrimarySecurityGroupID},
			},
		},
	}

	if p.serviceConfig.ResourceGroupID != "" {
		prototype.ResourceGroup = &vpcv1.ResourceGroupIdentity{ID: &p.serviceConfig.ResourceGroupID}
	}

	if p.serviceConfig.SecondarySubnetID != "" {

		var allowIPSpoofing bool = true

		prototype.NetworkInterfaces = []vpcv1.NetworkInterfacePrototype{
			{
				AllowIPSpoofing: &allowIPSpoofing,
				Subnet:          &vpcv1.SubnetIdentity{ID: &p.serviceConfig.SecondarySubnetID},
				SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
					&vpcv1.SecurityGroupIdentityByID{ID: &p.serviceConfig.SecondarySecurityGroupID},
				},
			},
		}
	}

	return prototype
}

func getIPs(instance *vpcv1.Instance, instanceID string, numInterfaces int) ([]net.IP, error) {

	interfaces := []*vpcv1.NetworkInterfaceInstanceContextReference{instance.PrimaryNetworkInterface}
	for i, nic := range instance.NetworkInterfaces {
		if *nic.ID != *instance.PrimaryNetworkInterface.ID {
			interfaces = append(interfaces, &instance.NetworkInterfaces[i])
		}
	}

	var ips []net.IP

	for i, nic := range interfaces {

		if nic.PrimaryIP == nil {
			return nil, errNotReady
		}
		addr := nic.PrimaryIP.Address
		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip := net.ParseIP(*addr)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse pod node IP %q", *addr)
		}
		ips = append(ips, ip)

		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	if len(ips) < numInterfaces {
		return nil, errNotReady
	}

	return ips, nil
}

func (p *ibmcloudVPCProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*cloud.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	prototype := p.getInstancePrototype(instanceName, userData)

	logger.Printf("CreateInstance: name: %q", instanceName)

	vpcInstance, resp, err := p.vpc.CreateInstanceWithContext(ctx, &vpcv1.CreateInstanceOptions{InstancePrototype: prototype})
	if err != nil {
		logger.Printf("failed to create an instance : %v and the response is %s", err, resp)
		return nil, err
	}

	instanceID := *vpcInstance.ID
	numInterfaces := len(prototype.NetworkInterfaces)

	var ips []net.IP

	for retries := 0; retries < maxRetries; retries++ {

		ips, err = getIPs(vpcInstance, instanceID, numInterfaces)

		if err == nil {
			break
		}
		if err != errNotReady {
			return nil, err
		}

		time.Sleep(time.Duration(queryInterval) * time.Second)

		result, resp, err := p.vpc.GetInstanceWithContext(ctx, &vpcv1.GetInstanceOptions{ID: &instanceID})
		if err != nil {
			logger.Printf("failed to get an instance : %v and the response is %s", err, resp)
			return nil, err
		}
		vpcInstance = result
	}

	instance := &cloud.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *ibmcloudVPCProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	options := &vpcv1.DeleteInstanceOptions{}
	options.SetID(instanceID)
	resp, err := p.vpc.DeleteInstanceWithContext(ctx, options)
	if err != nil {
		logger.Printf("failed to delete an instance: %v and the response is %v", err, resp)
		return err
	}

	logger.Printf("deleted an instance %s", instanceID)
	return nil
}

func (p *ibmcloudVPCProvider) Teardown() error {
	return nil
}
