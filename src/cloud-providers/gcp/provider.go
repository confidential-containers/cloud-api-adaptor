// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/netip"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	crm "cloud.google.com/go/resourcemanager/apiv3"
	crmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	resourcemanagerpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	proto "google.golang.org/protobuf/proto"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/gcp] ", log.LstdFlags|log.Lmsgprefix)
var computeScope = "https://www.googleapis.com/auth/compute"

const maxInstanceNameLen = 63

type gcpProvider struct {
	serviceConfig   *Config
	instancesClient *compute.InstancesClient
}

func (p *gcpProvider) ConfigVerifier() error {
	return nil
}

func NewProvider(config *Config) (provider.Provider, error) {
	logger.Printf("gcp config: %#v", config.Redact())
	provider := &gcpProvider{
		serviceConfig:   config,
		instancesClient: nil,
	}
	if config.GcpCredentials != "" {
		creds, err := google.CredentialsFromJSON(context.TODO(), []byte(config.GcpCredentials), computeScope)
		if err != nil {
			return nil, fmt.Errorf("configuration error when using creds: %s", err)
		}
		provider.instancesClient, err = compute.NewInstancesRESTClient(context.TODO(), option.WithCredentials(creds))
		if err != nil {
			return nil, fmt.Errorf("NewInstancesRESTClient with credentials error: %s", err)
		}
	} else {
		var err error
		provider.instancesClient, err = compute.NewInstancesRESTClient(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("NewInstancesRESTClient error: %s", err)
		}
	}
	return provider, nil
}

func parseIPString(ipStr string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to parse pod node IP %q: %w", ipStr, err)
	}

	return ip, nil
}

func getNatIPs(nic *computepb.NetworkInterface) ([]netip.Addr, error) {
	var natIPs []netip.Addr

	for _, access := range nic.GetAccessConfigs() {
		ip, err := parseIPString(access.GetNatIP())
		if err != nil {
			return nil, err
		}

		natIPs = append(natIPs, ip)
	}

	return natIPs, nil
}

func getIPs(intfcs []*computepb.NetworkInterface, usePublicIPs bool) ([]netip.Addr, error) {
	var podNodeIPs []netip.Addr

	for _, nic := range intfcs {
		var ips []netip.Addr

		if usePublicIPs {
			var err error

			ips, err = getNatIPs(nic)
			if err != nil {
				return nil, err
			}
		} else {
			ip, err := parseIPString(nic.GetNetworkIP())
			if err != nil {
				return nil, err
			}

			ips = []netip.Addr{ip}
		}

		podNodeIPs = append(podNodeIPs, ips...)
	}

	return podNodeIPs, nil
}

func (p *gcpProvider) ListAllTags(ctx context.Context) (map[string]map[string]*crmpb.TagValue, error) {
	tagKeysClient, err := crm.NewTagKeysClient(ctx)
	if err != nil {
		return nil, err
	}
	defer tagKeysClient.Close()

	tagValuesClient, err := crm.NewTagValuesClient(ctx)
	if err != nil {
		return nil, err
	}
	defer tagValuesClient.Close()

	parent := fmt.Sprintf("projects/%s", p.serviceConfig.ProjectId)
	tags := make(map[string]map[string]*crmpb.TagValue)

	it := tagKeysClient.ListTagKeys(ctx, &crmpb.ListTagKeysRequest{Parent: parent})
	for {
		key, err := it.Next()
		if err != nil {
			break
		}
		tagKeyID := key.Name
		keyName := key.ShortName
		tags[keyName] = make(map[string]*crmpb.TagValue)

		valIt := tagValuesClient.ListTagValues(ctx, &crmpb.ListTagValuesRequest{Parent: tagKeyID})
		for {
			val, err := valIt.Next()
			if err != nil {
				break
			}
			tags[keyName][val.ShortName] = val
		}
	}
	return tags, nil
}

func (p *gcpProvider) getImageSizeGB(ctx context.Context, image string) (int64, error) {
	client, err := compute.NewImagesRESTClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	var projectID string
	var imageName string

	// Parse project ID from full path if present
	// Supported formats:
	// - /projects/PROJECT-ID/global/images/IMAGE-NAME
	// - projects/PROJECT-ID/global/images/IMAGE-NAME
	// - https://www.googleapis.com/compute/v1/projects/PROJECT-ID/global/images/IMAGE-NAME
	if strings.HasPrefix(image, "/projects/") || strings.HasPrefix(image, "projects/") || strings.HasPrefix(image, "https://") {
		parts := strings.Split(image, "/")
		// Look for pattern: .../images/IMAGE-NAME
		for i := len(parts) - 2; i >= 0; i-- {
			if parts[i] == "images" && i >= 2 {
				projectID = parts[i-2]
				imageName = parts[len(parts)-1]
				break
			}
		}
	}

	// Fallback to ConfigMap project and image name
	if projectID == "" {
		projectID = p.serviceConfig.ProjectId
		parts := strings.Split(image, "/")
		imageName = parts[len(parts)-1]
	}

	req := &computepb.GetImageRequest{
		Project: projectID,
		Image:   imageName,
	}

	img, err := client.Get(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("Failed to get image for %s: %w", image, err)
	}

	return img.GetDiskSizeGb(), nil
}

func (p *gcpProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (instance *provider.Instance, err error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)
	logger.Printf("CreateInstance: name: %q", instanceName)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// Check if the tags exist within the project
	// Otherwise, abort the instance creation
	allTags, err := p.ListAllTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("Aborting: Failed to list tags: %w", err)
	}

	allTagValues := make([]*crmpb.TagValue, 0)
	for tagKey, tagValue := range p.serviceConfig.Tags {
		tagId := allTags[tagKey][tagValue]
		if tagId == nil {
			msg := fmt.Sprintf("Aborting: Tag %s=%s not found", tagKey, tagValue)
			logger.Print(msg)
			return nil, fmt.Errorf("%s", msg)
		}
		allTagValues = append(allTagValues, tagId)
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	// It's expected that the image from the annotation will follow one of supported formats:
	// - "projects/<project>/global/images/<imageid>" and "/projects/<project>/global/images/<imageid>",
	// - url: "https://www.googleapis.com/compute/v1/projects/<project>/global/images/<imageid>",
	// - simple "<imageid>" if the image is present on the same project.
	var srcImage *string
	if hasAnyPrefix(p.serviceConfig.ImageName, "projects/", "/projects", "https") {
		srcImage = proto.String(p.serviceConfig.ImageName)
	} else {
		srcImage = proto.String(fmt.Sprintf("projects/%s/global/images/%s", p.serviceConfig.ProjectId, p.serviceConfig.ImageName))
	}

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the GCP image for the PodVM image", spec.Image)
		srcImage = proto.String(spec.Image)
	}

	imageSizeGB, err := p.getImageSizeGB(ctx, *srcImage)
	if err != nil {
		return nil, fmt.Errorf("Failed to get image size: %w", err)
	}

	// If user provided RootVolumeSize, use the larger of the two
	if p.serviceConfig.RootVolumeSize > 0 && int64(p.serviceConfig.RootVolumeSize) > imageSizeGB {
		imageSizeGB = int64(p.serviceConfig.RootVolumeSize)
	}

	// Format subnetwork: support both short names and full paths
	// GCP accepts formats:
	// - "projects/<project>/regions/<region>/subnetworks/<subnetwork>" (full path)
	// - "regions/<region>/subnetworks/<subnetwork>" (partial path)
	// - "<subnetwork>" (short name, will be formatted as full path)
	// Extract region from zone (e.g., "us-central1-a" -> "us-central1")
	var subnetworkValue *string
	if p.serviceConfig.Subnetwork != "" {
		subnetworkName := p.serviceConfig.Subnetwork
		if hasAnyPrefix(subnetworkName, "projects/", "/projects", "regions/", "https") {
			subnetworkValue = proto.String(subnetworkName)
		} else {
			// Extract region from zone (format: "region-zone" e.g., "us-central1-a")
			zoneParts := strings.Split(p.serviceConfig.Zone, "-")
			if len(zoneParts) >= 2 {
				region := strings.Join(zoneParts[:len(zoneParts)-1], "-")
				formattedSubnetwork := fmt.Sprintf("projects/%s/regions/%s/subnetworks/%s", p.serviceConfig.ProjectId, region, subnetworkName)
				subnetworkValue = proto.String(formattedSubnetwork)
			} else {
				// Fallback: assume zone format is invalid, try to use as-is
				subnetworkValue = proto.String(subnetworkName)
			}
		}
	}

	networkInterface := &computepb.NetworkInterface{
		Network: proto.String(p.serviceConfig.Network),
		AccessConfigs: []*computepb.AccessConfig{
			{
				Name:        proto.String("External NAT"),
				NetworkTier: proto.String("STANDARD"),
			},
		},
		StackType: proto.String("IPV4_Only"),
	}
	if subnetworkValue != nil {
		networkInterface.Subnetwork = subnetworkValue
	}

	instanceResource := &computepb.Instance{
		Name: proto.String(instanceName),
		Disks: []*computepb.AttachedDisk{
			{
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					DiskSizeGb:  proto.Int64(imageSizeGB),
					SourceImage: srcImage,
					DiskType:    proto.String(fmt.Sprintf("zones/%s/diskTypes/%s", p.serviceConfig.Zone, p.serviceConfig.DiskType)),
				},
				AutoDelete: proto.Bool(true),
				Boot:       proto.Bool(true),
				Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
			},
		},
		Metadata: &computepb.Metadata{
			Items: []*computepb.Items{
				{
					Key:   proto.String("user-data"),
					Value: proto.String(userDataEnc),
				},
				{
					Key:   proto.String("user-data-encoding"),
					Value: proto.String("base64"),
				},
			},
		},
		MachineType:       proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.serviceConfig.Zone, p.serviceConfig.MachineType)),
		NetworkInterfaces: []*computepb.NetworkInterface{networkInterface},
	}

	// Check if OnHostMaintenance needs to be set to TERMINATE
	// This is required for:
	// 1. Confidential VMs
	// 2. GPU instances (when spec.GPUs > 0)
	requiresTerminatePolicy := false

	if !p.serviceConfig.DisableCVM {
		if p.serviceConfig.ConfidentialType == "" {
			return nil, fmt.Errorf("ConfidentialType must be set when using Confidential VM.")
		}

		instanceResource.ConfidentialInstanceConfig = &computepb.ConfidentialInstanceConfig{
			ConfidentialInstanceType:  proto.String(p.serviceConfig.ConfidentialType),
			EnableConfidentialCompute: proto.Bool(true),
		}
		requiresTerminatePolicy = true
	}

	// Check if GPUs are requested via annotation
	if spec.GPUs > 0 {
		logger.Printf("GPUs requested (%d), setting OnHostMaintenance to TERMINATE", spec.GPUs)
		requiresTerminatePolicy = true
	}

	if requiresTerminatePolicy {
		instanceResource.Scheduling = &computepb.Scheduling{
			OnHostMaintenance: proto.String("TERMINATE"),
		}
	}

	insertReq := &computepb.InsertInstanceRequest{
		Project:          p.serviceConfig.ProjectId,
		Zone:             p.serviceConfig.Zone,
		InstanceResource: instanceResource,
	}

	op, err := p.instancesClient.Insert(ctx, insertReq)
	if err != nil {
		return nil, fmt.Errorf("Instances.Insert error: %s. req: %v", err, insertReq)
	}
	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for Instances.Insert error: %s. req: %v", err, insertReq)
	}
	logger.Printf("created an instance %s for sandbox %s", instanceName, sandboxID)

	// Create partial instance to return on error (allows caller to cleanup)
	instance = &provider.Instance{
		ID:   instanceName,
		Name: instanceName,
	}

	getReq := &computepb.GetInstanceRequest{
		Project:  p.serviceConfig.ProjectId,
		Zone:     p.serviceConfig.Zone,
		Instance: instanceName,
	}

	gcpInstance, err := p.instancesClient.Get(ctx, getReq)
	if err != nil {
		return instance, fmt.Errorf("unable to get instance: %w, req: %v", err, getReq)
	}
	logger.Printf("instance name %s, id %d", gcpInstance.GetName(), gcpInstance.GetId())

	// Binding all the tagValues to the instance that was already created
	// Specific endpoint is needed for tag bindings because global endpoint
	// doesn't work for zonal resources.
	tagBindingsClient, err := crm.NewTagBindingsClient(ctx,
		option.WithEndpoint(fmt.Sprintf("%s-cloudresourcemanager.googleapis.com:443", p.serviceConfig.Zone)),
	)
	if err != nil {
		return instance, fmt.Errorf("failed to create bind client: %w", err)
	}
	defer tagBindingsClient.Close()

	parent := fmt.Sprintf("//compute.googleapis.com/projects/%s/zones/%s/instances/%d", p.serviceConfig.ProjectId, p.serviceConfig.Zone, gcpInstance.GetId())

	for _, tagValue := range allTagValues {
		logger.Printf("Creating tag binding for %s on %s", tagValue.Name, parent)

		tagBinding := &resourcemanagerpb.TagBinding{
			Parent:   parent,
			TagValue: tagValue.Name,
		}

		req := &resourcemanagerpb.CreateTagBindingRequest{
			TagBinding: tagBinding,
		}

		op, err := tagBindingsClient.CreateTagBinding(ctx, req)
		if err != nil {
			return instance, fmt.Errorf("API call to create tag binding failed for %s: %v", tagValue, err)
		}

		_, err = op.Wait(ctx)
		if err != nil {
			return instance, fmt.Errorf("Long-running operation for tag binding %s failed: %v", tagValue, err)
		}

		logger.Printf("Created tag binding for %s on %s successfully", tagValue, parent)
	}

	ips, err := getIPs(gcpInstance.GetNetworkInterfaces(), p.serviceConfig.UsePublicIP)
	if err != nil {
		logger.Printf("failed to get IPs for the instance: %v", err)
		return instance, err
	}

	logger.Printf("Found pod node IP(s): %v", ips)

	instance.IPs = ips

	return instance, nil
}

func (p *gcpProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	req := &computepb.DeleteInstanceRequest{
		Project:  p.serviceConfig.ProjectId,
		Zone:     p.serviceConfig.Zone,
		Instance: instanceID,
	}
	op, err := p.instancesClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("Instances.Delete error: %w, req: %v", err, req)
	}
	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for Instances.Delete error: %s. req: %v", err, req)
	}
	logger.Printf("deleted an instance %s", instanceID)
	return nil
}

func (p *gcpProvider) Teardown() error {
	return nil
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
