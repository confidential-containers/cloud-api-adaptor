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

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
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
	GetInstanceProfileWithContext(context.Context, *vpcv1.GetInstanceProfileOptions) (*vpcv1.InstanceProfile, *core.DetailedResponse, error)
	GetImageWithContext(ctx context.Context, getImageOptions *vpcv1.GetImageOptions) (*vpcv1.Image, *core.DetailedResponse, error)
}

type ibmcloudVPCProvider struct {
	vpc           vpcV1
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {

	var authenticator core.Authenticator

	if config.APIKey != "" {
		authenticator = &core.IamAuthenticator{
			ApiKey: config.APIKey,
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
		nodeLabels, err = util.NodeLabels(context.TODO(), nodeName)
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
	primarySubnetID, iks := nodeLabels["ibm-provider.kubernetes.io/subnet-id"]
	if !iks {
		primarySubnetID, iks = nodeLabels["ibm-cloud.kubernetes.io/subnet-id"]
	}
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

	provider := &ibmcloudVPCProvider{
		vpc:           vpcV1,
		serviceConfig: config,
	}

	if err = provider.updateInstanceProfileSpecList(); err != nil {
		return nil, err
	}

	if err = provider.updateImageList(context.TODO()); err != nil {
		return nil, err
	}

	logger.Printf("ibmcloud-vpc config: %#v", config.Redact())

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

func (p *ibmcloudVPCProvider) getInstancePrototype(instanceName, userData, instanceProfile, imageID string) *vpcv1.InstancePrototype {

	prototype := &vpcv1.InstancePrototype{
		Name:     &instanceName,
		Image:    &vpcv1.ImageIdentity{ID: &imageID},
		UserData: &userData,
		Profile:  &vpcv1.InstanceProfileIdentity{Name: &instanceProfile},
		Zone:     &vpcv1.ZoneIdentity{Name: &p.serviceConfig.ZoneName},
		Keys:     []vpcv1.KeyIdentityIntf{},
		VPC:      &vpcv1.VPCIdentity{ID: &p.serviceConfig.VpcID},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfacePrototype{
			Subnet: &vpcv1.SubnetIdentity{ID: &p.serviceConfig.PrimarySubnetID},
			SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
				&vpcv1.SecurityGroupIdentityByID{ID: &p.serviceConfig.PrimarySecurityGroupID},
			},
		},
		MetadataService: &vpcv1.InstanceMetadataServicePrototype{
			Enabled:  core.BoolPtr(true),
			Protocol: core.StringPtr(vpcv1.InstanceMetadataServicePatchProtocolHTTPConst),
		},
		ConfidentialComputeMode: core.StringPtr(vpcv1.InstanceConfidentialComputeModeTdxConst),
		EnableSecureBoot:        core.BoolPtr(true),
	}

	if p.serviceConfig.DisableCVM {
		prototype.ConfidentialComputeMode = core.StringPtr(vpcv1.InstanceConfidentialComputeModeDisabledConst)
		prototype.EnableSecureBoot = core.BoolPtr(false)
	}

	if p.serviceConfig.KeyID != "" {
		prototype.Keys = append(prototype.Keys, &vpcv1.KeyIdentity{ID: &p.serviceConfig.KeyID})
	}

	if p.serviceConfig.ResourceGroupID != "" {
		prototype.ResourceGroup = &vpcv1.ResourceGroupIdentity{ID: &p.serviceConfig.ResourceGroupID}
	}

	if p.serviceConfig.SecondarySubnetID != "" {

		var allowIPSpoofing = true

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

func (p *ibmcloudVPCProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	instanceProfile, err := p.selectInstanceProfile(ctx, spec)
	if err != nil {
		return nil, err
	}

	imageID := spec.Image
	if imageID != "" {
		logger.Printf("Choosing %s from annotation as the IBM Cloud Image for the PodVM image", imageID)
	} else {
		imageID, err = p.selectImage(ctx, spec, instanceProfile)
		if err != nil {
			return nil, err
		}
	}

	prototype := p.getInstancePrototype(instanceName, userData, instanceProfile, imageID)

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

	instance := &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

// Select an instance profile based on the memory and vcpu requirements
func (p *ibmcloudVPCProvider) selectInstanceProfile(ctx context.Context, spec provider.InstanceTypeSpec) (string, error) {

	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceProfileSpecList, p.serviceConfig.InstanceProfiles, p.serviceConfig.ProfileName)
}

// Populate instanceProfileSpecList for all the instanceProfiles
func (p *ibmcloudVPCProvider) updateInstanceProfileSpecList() error {

	// Get the instance types from the service config
	instanceProfiles := p.serviceConfig.InstanceProfiles

	// If instanceProfiles is empty then populate it with the default instance type
	if len(instanceProfiles) == 0 {
		instanceProfiles = append(instanceProfiles, p.serviceConfig.ProfileName)
	}

	// Create a list of instanceProfileSpec
	var instanceProfileSpecList []provider.InstanceTypeSpec

	// Iterate over the instance types and populate the instanceProfileSpecList
	for _, profileType := range instanceProfiles {
		vcpus, memory, arch, err := p.getProfileNameInformation(profileType)
		if err != nil {
			return err
		}
		instanceProfileSpecList = append(instanceProfileSpecList, provider.InstanceTypeSpec{InstanceType: profileType, VCPUs: vcpus, Memory: memory, Arch: arch})
	}

	// Sort the instanceProfileSpecList and update the serviceConfig
	p.serviceConfig.InstanceProfileSpecList = provider.SortInstanceTypesOnResources(instanceProfileSpecList)
	logger.Printf("instanceProfileSpecList (%v)", p.serviceConfig.InstanceProfileSpecList)
	return nil
}

// Add a method to retrieve cpu, memory, and arch from the profile name
func (p *ibmcloudVPCProvider) getProfileNameInformation(profileName string) (vcpu int64, memory int64, arch string, err error) {

	// Get the profile information from the instance type using IBMCloud API
	result, details, err := p.vpc.GetInstanceProfileWithContext(context.Background(),
		&vpcv1.GetInstanceProfileOptions{
			Name: &profileName,
		},
	)

	if err != nil {
		return 0, 0, "", fmt.Errorf("instance profile name %s not found, due to %w\nFurther Details:\n%v", profileName, err, details)
	}

	vcpu = int64(*result.VcpuCount.(*vpcv1.InstanceProfileVcpu).Value)
	// Value returned is in GiB, convert to MiB
	memory = int64(*result.Memory.(*vpcv1.InstanceProfileMemory).Value) * 1024
	arch = string(*result.VcpuArchitecture.Value)
	return vcpu, memory, arch, nil
}

// Select Image from list, invalid image IDs should have already been removed
func (p *ibmcloudVPCProvider) selectImage(ctx context.Context, spec provider.InstanceTypeSpec, selectedInstanceProfile string) (string, error) {

	specArch := spec.Arch
	if specArch == "" {
		for _, instanceProfileSpec := range p.serviceConfig.InstanceProfileSpecList {
			if instanceProfileSpec.InstanceType == selectedInstanceProfile {
				specArch = instanceProfileSpec.Arch
				break
			}
		}
	}

	for _, image := range p.serviceConfig.Images {
		if specArch != "" && image.Arch != specArch {
			continue
		}
		logger.Printf("selected image with ID <%s> out of %d images", image.ID, len(p.serviceConfig.Images))
		return image.ID, nil
	}
	return "", fmt.Errorf("unable to find matching image to use")
}

// Remove Images that are not valid (e.g. not found in this region)
func (p *ibmcloudVPCProvider) updateImageList(ctx context.Context) error {
	i := 0
	for _, image := range p.serviceConfig.Images {
		arch, os, err := p.getImageDetails(ctx, image.ID)
		if err != nil {
			logger.Printf("skipping image (%s), due to %v", image.ID, err)
			continue
		}
		image.Arch = arch
		image.OS = os
		p.serviceConfig.Images[i] = image
		i++
	}
	if i == 0 {
		return fmt.Errorf("no images valid images found")
	}
	p.serviceConfig.Images = p.serviceConfig.Images[:i]
	return nil
}

func (p *ibmcloudVPCProvider) getImageDetails(ctx context.Context, imageID string) (arch, os string, err error) {
	result, _, err := p.vpc.GetImageWithContext(ctx, &vpcv1.GetImageOptions{
		ID: &imageID,
	})
	if err != nil {
		return "", "", err
	}
	return *result.OperatingSystem.Architecture, *result.OperatingSystem.Name, nil
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

func (p *ibmcloudVPCProvider) ConfigVerifier() error {
	images := p.serviceConfig.Images.String()
	if len(images) == 0 {
		return fmt.Errorf("image-id is empty")
	}
	return nil
}
