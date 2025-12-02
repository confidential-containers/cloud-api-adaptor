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

	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries    = 10
	queryInterval = 2

	clusterInfoCMName      = "cluster-info"
	clusterInfoCMNamespace = "kube-system"
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

type globalTaggingV1 interface {
	AttachTagWithContext(ctx context.Context, attachTagOptions *globaltaggingv1.AttachTagOptions) (*globaltaggingv1.TagResults, *core.DetailedResponse, error)
}

type clusterV2 interface {
	GetClusterTypeSecurityGroups(clusterID string) (result []securityGroup, response *core.DetailedResponse, err error)
}

type ibmcloudVPCProvider struct {
	vpc           vpcV1
	globalTagging globalTaggingV1
	cluster       clusterV2
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {

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

	if config.ClusterID == "" {
		clusterID, err := getClusterID()
		if err != nil {
			return nil, fmt.Errorf("could not automatically find cluster ID: %w", err)
		}
		config.ClusterID = clusterID
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
		vpcID, rgID, err := fetchVPCDetails(vpcV1, primarySubnetID)
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
		}
	}

	// Return error early
	if config.ZoneName == "" {
		return nil, fmt.Errorf("zone was not provided and could not detect automatically")
	}

	if len(config.DedicatedHostIDs) > 0 {
		selected, err := pickIDInZone(
			config.DedicatedHostIDs,
			config.ZoneName,
			func(id string) (string, error) { return getDedicatedHostZone(vpcV1, id) },
			"Dedicated Host",
		)
		if err != nil {
			return nil, err
		}
		config.selectedDedicatedHostID = selected
	}

	if len(config.DedicatedHostGroupIDs) > 0 {
		selected, err := pickIDInZone(
			config.DedicatedHostGroupIDs,
			config.ZoneName,
			func(id string) (string, error) { return getDedicatedHostGroupZone(vpcV1, id) },
			"Dedicated Host Group",
		)
		if err != nil {
			return nil, err
		}
		config.selectedDedicatedHostGroupID = selected
	}

	gTaggingV1, err := globaltaggingv1.NewGlobalTaggingV1(
		&globaltaggingv1.GlobalTaggingV1Options{
			Authenticator: authenticator,
		})
	if err != nil {
		return nil, err
	}

	clusterV2, err := NewClusterV2Service(&ClusterOptions{Authenticator: authenticator})
	if err != nil {
		return nil, err
	}

	if config.PrimarySecurityGroupID == "" {
		sgID, err := fetchClusterSG(clusterV2, config.ClusterID)
		if err != nil {
			return nil, err
		}
		config.PrimarySecurityGroupID = sgID
	}

	provider := &ibmcloudVPCProvider{
		vpc:           vpcV1,
		globalTagging: gTaggingV1,
		cluster:       clusterV2,
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

func getClusterID() (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get k8s rest config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	cm, err := clientset.CoreV1().ConfigMaps(clusterInfoCMNamespace).Get(context.Background(), clusterInfoCMName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("could not get %s config map in %s namespace: %w", clusterInfoCMName, clusterInfoCMNamespace, err)
	}

	clusterID, ok := cm.Data["cluster_id"]
	if !ok {
		return "", fmt.Errorf("could not find cluster_id key in %s config map in %s namespace", clusterInfoCMName, clusterInfoCMNamespace)
	}

	return clusterID, nil
}

func fetchVPCDetails(vpcV1 *vpcv1.VpcV1, subnetID string) (vpcID string, resourceGroupID string, e error) {
	subnet, response, err := vpcV1.GetSubnet(&vpcv1.GetSubnetOptions{
		ID: &subnetID,
	})
	if err != nil {
		e = fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	vpcID = *subnet.VPC.ID
	resourceGroupID = *subnet.ResourceGroup.ID
	return
}

func fetchClusterSG(clusterv2 clusterV2, clusterID string) (securityGroupID string, e error) {
	securityGroups, response, err := clusterv2.GetClusterTypeSecurityGroups(clusterID)
	if err != nil {
		e = fmt.Errorf("cluster error with:\n %w\nfurther details:\n %v", err, response)
		return
	}

	expectedSgName := fmt.Sprintf("kube-%s", clusterID)

	for _, sg := range securityGroups {
		if sg.Name == expectedSgName {
			securityGroupID = sg.ID
			return
		}
	}
	e = fmt.Errorf("could not find default cluster security group %s", expectedSgName)
	return
}

func getDedicatedHostZone(vpcV1 *vpcv1.VpcV1, dedicatedHostID string) (string, error) {
	dedicatedHostOptions := vpcv1.GetDedicatedHostOptions{
		ID: &dedicatedHostID,
	}
	dedicatedHost, response, err := vpcV1.GetDedicatedHost(&dedicatedHostOptions)
	if err != nil {
		return "", fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
	}

	return *dedicatedHost.Zone.Name, nil
}

func getDedicatedHostGroupZone(vpcV1 *vpcv1.VpcV1, dedicatedHostGroupID string) (string, error) {
	dedicatedHostGroupOptions := vpcv1.GetDedicatedHostGroupOptions{
		ID: &dedicatedHostGroupID,
	}
	dedicatedHostGroup, response, err := vpcV1.GetDedicatedHostGroup(&dedicatedHostGroupOptions)
	if err != nil {
		return "", fmt.Errorf("VPC error with:\n %w\nfurther details:\n %v", err, response)
	}

	return *dedicatedHostGroup.Zone.Name, nil
}

// pickIDInZone finds the first ID whose zone equals zoneName.
// If multiple IDs match the zone, it logs a warning and returns the first.
// If none match, it returns a descriptive error.
func pickIDInZone(ids []string, zoneName string, getZone func(string) (string, error), resourceLabel string) (string, error) {
	var selected string

	for _, id := range ids {
		zone, err := getZone(id)
		if err != nil {
			return "", fmt.Errorf("couldn't get %s %s's zone: %w", id, resourceLabel, err)
		}
		if zone == zoneName {
			if selected != "" && logger != nil {
				logger.Printf("warning, multiple %ss were provided in zone %s; only one will be used",
					resourceLabel, zoneName)
				// Continue to keep the first match as the selected one.
				continue
			}
			selected = id
		}
	}

	if selected == "" {
		return "", fmt.Errorf("no %s in zone %s was provided; please provide a %s in the specified zone for High Availability",
			resourceLabel, zoneName, resourceLabel)
	}
	return selected, nil
}

func (p *ibmcloudVPCProvider) getAttachTagOptions(vpcInstanceCRN *string) (*globaltaggingv1.AttachTagOptions, error) {
	if vpcInstanceCRN == nil {
		return nil, fmt.Errorf("missing vpc instance crn, can't create attach tag options")
	}

	tagNames := append([]string{"coco-pod-vm:" + p.serviceConfig.ClusterID}, p.serviceConfig.Tags...)

	options := &globaltaggingv1.AttachTagOptions{
		Resources: []globaltaggingv1.Resource{{ResourceID: vpcInstanceCRN}},
	}
	options.SetTagType("user")
	options.SetTagNames(tagNames)

	return options, nil
}

func (p *ibmcloudVPCProvider) getInstancePrototype(instanceName, userData, instanceProfile, imageId string) *vpcv1.InstancePrototype {

	prototype := &vpcv1.InstancePrototype{
		Name:     &instanceName,
		Image:    &vpcv1.ImageIdentity{ID: &imageId},
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

	// When both dedicated host id and group id provided the (more specific) dedicated host id will be used as the placement target
	if p.serviceConfig.selectedDedicatedHostGroupID != "" {
		prototype.PlacementTarget = &vpcv1.InstancePlacementTargetPrototypeDedicatedHostGroupIdentityDedicatedHostGroupIdentityByID{ID: &p.serviceConfig.selectedDedicatedHostGroupID}
	}

	if p.serviceConfig.selectedDedicatedHostID != "" {
		prototype.PlacementTarget = &vpcv1.InstancePlacementTargetPrototypeDedicatedHostIdentityDedicatedHostIdentityByID{ID: &p.serviceConfig.selectedDedicatedHostID}
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

func (p *ibmcloudVPCProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (instance *provider.Instance, err error) {

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

	vpcInstance, err := p.createInstanceWithFallback(ctx, prototype)
	if err != nil {
		return nil, err
	}

	instanceID := *vpcInstance.ID
	numInterfaces := len(prototype.NetworkInterfaces)

	// Create partial instance to return on error (allows caller to cleanup)
	instance = &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
	}

	var ips []netip.Addr

	for retries := 0; retries < maxRetries; retries++ {

		ips, err = getIPs(vpcInstance, instanceID, numInterfaces)

		if err == nil {
			break
		}
		if err != errNotReady {
			return instance, err
		}

		time.Sleep(time.Duration(queryInterval) * time.Second)

		result, response, err := p.vpc.GetInstanceWithContext(ctx, &vpcv1.GetInstanceOptions{ID: &instanceID})
		if err != nil {
			logger.Printf("failed to get an instance : %v and the response is %s", err, response)
			return instance, err
		}
		vpcInstance = result
	}

	instance.IPs = ips

	options, err := p.getAttachTagOptions(vpcInstance.CRN)
	if err != nil {
		return instance, fmt.Errorf("failed to get attach tag options: %w", err)
	}

	_, resp, err := p.globalTagging.AttachTagWithContext(ctx, options)
	if err != nil {
		return instance, fmt.Errorf("failed to attach tags: %w and the response is %s", err, resp)
	}
	logger.Printf("successfully attached tags: %v to instance: %v", options.TagNames, *vpcInstance.CRN)

	return instance, nil
}

func (p *ibmcloudVPCProvider) createInstanceWithFallback(ctx context.Context, prototype *vpcv1.InstancePrototype) (*vpcv1.Instance, error) {

	dedicatedHostID := p.serviceConfig.selectedDedicatedHostID
	dedicatedHostGroupID := p.serviceConfig.selectedDedicatedHostGroupID

	inst, resp, err := p.vpc.CreateInstanceWithContext(ctx, &vpcv1.CreateInstanceOptions{
		InstancePrototype: prototype,
	})
	if err == nil {
		return inst, nil
	}

	// Fallback if both IDs exist
	if dedicatedHostID != "" && dedicatedHostGroupID != "" {
		logger.Printf("warning, creation failed on dedicated host %q: %v; retrying on dedicated host group %q", dedicatedHostID, err, dedicatedHostGroupID)

		prototype.PlacementTarget =
			&vpcv1.InstancePlacementTargetPrototypeDedicatedHostGroupIdentityDedicatedHostGroupIdentityByID{
				ID: &dedicatedHostGroupID,
			}

		inst2, resp2, err2 := p.vpc.CreateInstanceWithContext(ctx, &vpcv1.CreateInstanceOptions{
			InstancePrototype: prototype,
		})
		if err2 == nil {
			return inst2, nil
		}

		// Return both errors for context.
		return nil, errors.Join(
			fmt.Errorf("instance creation on dedicated host %q failed: %w and the response is %s", dedicatedHostID, err, resp),
			fmt.Errorf("fallback instance creation on dedicated host group %q failed: %w and the response is %s", dedicatedHostGroupID, err2, resp2),
		)
	}

	return nil, fmt.Errorf("failed to create an instance: %w and the response is %s", err, resp)
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
