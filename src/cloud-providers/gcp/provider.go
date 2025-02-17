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
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"golang.org/x/oauth2/google"
	option "google.golang.org/api/option"
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

func getIPs(instance *computepb.Instance) ([]netip.Addr, error) {
	var podNodeIPs []netip.Addr
	for _, nic := range instance.GetNetworkInterfaces() {
		for _, access := range nic.GetAccessConfigs() {
			ipStr := access.GetNatIP()
			ip, err := netip.ParseAddr(ipStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pod node IP %q: %w", ipStr, err)
			}
			podNodeIPs = append(podNodeIPs, ip)
			logger.Printf("Found pod node IP: %s", ip.String())
		}
	}
	return podNodeIPs, nil
}

func (p *gcpProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)
	logger.Printf("CreateInstance: name: %q", instanceName)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))
	logger.Printf("userDataEnc:  %s", userDataEnc)

	// It's expected that the image from the annotation will follow the format
	// "projects/<project>/global/images/<imageid>" or just the "<imageid>" if the
	// image is present on the same project.
	var srcImage *string
	if strings.HasPrefix(p.serviceConfig.ImageName, "projects/") {
		srcImage = proto.String(p.serviceConfig.ImageName)
	} else {
		srcImage = proto.String(fmt.Sprintf("projects/%s/global/images/%s", p.serviceConfig.ProjectId, p.serviceConfig.ImageName))
	}

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the GCP image for the PodVM image", spec.Image)
		srcImage = proto.String(spec.Image)
	}

	insertReq := &computepb.InsertInstanceRequest{
		Project: p.serviceConfig.ProjectId,
		Zone:    p.serviceConfig.Zone,
		InstanceResource: &computepb.Instance{
			Name: proto.String(instanceName),
			Disks: []*computepb.AttachedDisk{
				{
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskSizeGb:  proto.Int64(20),
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
			MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.serviceConfig.Zone, p.serviceConfig.MachineType)),
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name:        proto.String("External NAT"),
							NetworkTier: proto.String("STANDARD"),
						},
					},
					StackType: proto.String("IPV4_Only"),
					Name:      proto.String(p.serviceConfig.Network),
				},
			},
		},
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

	getReq := &computepb.GetInstanceRequest{
		Project:  p.serviceConfig.ProjectId,
		Zone:     p.serviceConfig.Zone,
		Instance: instanceName,
	}

	instance, err := p.instancesClient.Get(ctx, getReq)
	if err != nil {
		return nil, fmt.Errorf("unable to get instance: %w, req: %v", err, getReq)
	}
	logger.Printf("instance name %s, id %d", instance.GetName(), instance.GetId())

	ips, err := getIPs(instance)
	if err != nil {
		logger.Printf("failed to get IPs for the instance: %v", err)
		return nil, err
	}

	return &provider.Instance{
		ID:   instance.GetName(),
		Name: instance.GetName(),
		IPs:  ips,
	}, nil
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
