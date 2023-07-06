// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/k8sops"
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

	var authenticator core.Authenticator

	if config.ApiKey != "" {
		authenticator = &core.IamAuthenticator{
			ApiKey: config.ApiKey,
			URL:    config.IamServiceURL,
		}
	} else if config.IAMProfileID != "" {
		authenticator = &core.ContainerAuthenticator{
			URL:             config.IamServiceURL,
			IAMProfileID:    config.IAMProfileID,
			CRTokenFilename: config.CRTokenFileName,
		}
	} else {
		return nil, fmt.Errorf("either an IAM API Key or Profile ID needs to be set")
	}

	nodeName, ok := os.LookupEnv("NODE_NAME")
	var nodeLabels map[string]string
	if ok {
		var err error
		nodeLabels, err = k8sops.NodeLabels(context.TODO(), nodeName)
		if err != nil {
			logger.Printf("warning, could not find node labels\ndue to: %v\n", err)
		}
	}

	nodeRegion, ok := nodeLabels["topology.kubernetes.io/region"]
	if config.VpcServiceURL == "" && ok {
		// Assume in prod if fetching from labels for now
		// TODO handle other environments
		config.VpcServiceURL = fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", nodeRegion)
	}

	vpcV1, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: authenticator,
		URL:           config.VpcServiceURL,
	})

	if err != nil {
		return nil, err
	}

	// If this label exists assume we are in an IKS cluster
	primarySubnetID, iks := nodeLabels["ibm-cloud.kubernetes.io/subnet-id"]
	if iks {
		if config.ZoneName == "" {
			config.ZoneName = nodeLabels["topology.kubernetes.io/zone"]
		}
		vpcID, rgID, sgID, err := fetchVPCDetails(vpcV1, primarySubnetID)
		if err != nil {
			logger.Printf("warning, unable to automatically populate VPC details\ndue to: %v\n", err)
		} else {
			if config.PrimarySubnetID == "" {
				config.PrimarySubnetID = primarySubnetID
			}
			if config.VpcID == "" {
				config.VpcID = vpcID
			}
			if config.ResourceGroupID == "" {
				config.ResourceGroupID = rgID
			}
			if config.PrimarySecurityGroupID == "" {
				config.PrimarySecurityGroupID = sgID
			}
		}
	}

	logger.Printf("ibmcloud-vpc config: %#v", config.Redact())

	provider := &ibmcloudVPCProvider{
		vpc:           vpcV1,
		serviceConfig: config,
	}

	return provider, nil
}

func fetchVPCDetails(vpcV1 *vpcv1.VpcV1, subnetID string) (vpcID string, resourceGroupID string, securityGroupID string, e error) {
	subnet, response, err := vpcV1.GetSubnet(&vpcv1.GetSubnetOptions{
		ID: &subnetID,
	})
	if err != nil {
		e = fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	sg, response, err := vpcV1.GetVPCDefaultSecurityGroup(&vpcv1.GetVPCDefaultSecurityGroupOptions{
		ID: subnet.VPC.ID,
	})
	if err != nil {
		e = fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	securityGroupID = *sg.ID
	vpcID = *subnet.VPC.ID
	resourceGroupID = *subnet.ResourceGroup.ID
	return
}

func (p *ibmcloudVPCProvider) getInstancePrototype(instanceName, userData string) *vpcv1.InstancePrototype {

	prototype := &vpcv1.InstancePrototype{
		Name:     &instanceName,
		Image:    &vpcv1.ImageIdentity{ID: &p.serviceConfig.ImageID},
		UserData: &userData,
		Profile:  &vpcv1.InstanceProfileIdentity{Name: &p.serviceConfig.ProfileName},
		Zone:     &vpcv1.ZoneIdentity{Name: &p.serviceConfig.ZoneName},
		Keys:     []vpcv1.KeyIdentityIntf{},
		VPC:      &vpcv1.VPCIdentity{ID: &p.serviceConfig.VpcID},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfacePrototype{
			Subnet: &vpcv1.SubnetIdentity{ID: &p.serviceConfig.PrimarySubnetID},
			SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
				&vpcv1.SecurityGroupIdentityByID{ID: &p.serviceConfig.PrimarySecurityGroupID},
			},
		},
	}

	if p.serviceConfig.KeyID != "" {
		prototype.Keys = append(prototype.Keys, &vpcv1.KeyIdentity{ID: &p.serviceConfig.KeyID})
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

func getIPs(instance *vpcv1.Instance, instanceID string, numInterfaces int) ([]netip.Addr, error) {

	interfaces := []*vpcv1.NetworkInterfaceInstanceContextReference{instance.PrimaryNetworkInterface}
	for i, nic := range instance.NetworkInterfaces {
		if *nic.ID != *instance.PrimaryNetworkInterface.ID {
			interfaces = append(interfaces, &instance.NetworkInterfaces[i])
		}
	}

	var ips []netip.Addr

	for i, nic := range interfaces {

		if nic.PrimaryIP == nil {
			return nil, errNotReady
		}
		addr := nic.PrimaryIP.Address
		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip, err := netip.ParseAddr(*addr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod node IP %q: %w", *addr, err)
		}
		ips = append(ips, ip)

		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	if len(ips) < numInterfaces {
		return nil, errNotReady
	}

	return ips, nil
}

func (p *ibmcloudVPCProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec cloud.InstanceTypeSpec) (*cloud.Instance, error) {

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

	var ips []netip.Addr

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
