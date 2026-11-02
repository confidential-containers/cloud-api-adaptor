// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"strings"
	"time"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/ecloud] ", log.LstdFlags|log.Lmsgprefix)

const (
	maxInstanceNameLen = 63

	defaultSystemDiskType = "highPerformance"
	chargeModePostPaid    = "HOUR"
	instanceStatusActive  = "active"
	publicIPTypeSingle    = "MOBILE"
)

type ecloudProvider struct {
	client        ecsClient
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {
	logger.Printf("ecloud config: %#v", config.Redact())

	if config.AccessKeyID == "" || config.SecretKey == "" {
		return nil, errors.New("ECLOUD_ACCESS_KEY_ID and ECLOUD_ACCESS_KEY_SECRET must be provided")
	}
	if config.Region == "" {
		return nil, errors.New("Region must be provided")
	}

	client, err := newEcloudClient("", config.AccessKeyID, config.SecretKey, config.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create ecloud client: %w", err)
	}

	p := &ecloudProvider{
		client:        client,
		serviceConfig: config,
	}

	if err := p.updateInstanceTypeSpecList(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to update instance type spec list: %w", err)
	}

	return p, nil
}

func (p *ecloudProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (instance *provider.Instance, err error) {
	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	// Convert userData to base64
	b64EncData := base64.StdEncoding.EncodeToString([]byte(cloudConfigData))

	instanceType, err := p.selectInstanceType(ctx, spec)
	if err != nil {
		return nil, err
	}

	imageID := p.serviceConfig.ImageID
	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the image for the PodVM image", spec.Image)
		imageID = spec.Image
	}

	var tags []instanceTag
	for k, v := range p.serviceConfig.Tags {
		tags = append(tags, instanceTag{Key: k, Value: v})
	}

	req := &createInstancesRequest{
		ZoneID:           p.serviceConfig.ZoneID,
		ImageID:          imageID,
		FlavorName:       instanceType,
		ChargeMode:       chargeModePostPaid,
		Quantity:         1,
		InstanceName:     instanceName,
		UserData:         b64EncData,
		PrivateNetwork:   &privateNetwork{NetworkID: p.serviceConfig.NetworkID},
		SecurityGroupIDs: p.serviceConfig.SecurityGroupIDs,
		Tags:             tags,
	}

	if p.serviceConfig.KeyName != "" {
		req.KeyPairName = p.serviceConfig.KeyName
	}

	if p.serviceConfig.SystemDiskSize > 0 || p.serviceConfig.SystemDiskType != "" {
		volumeType := p.serviceConfig.SystemDiskType
		if volumeType == "" {
			volumeType = defaultSystemDiskType
		}
		req.BootVolume = &bootVolume{
			Size:       p.serviceConfig.SystemDiskSize,
			VolumeType: volumeType,
		}
	}

	// A bandwidth size greater than 0 makes the platform allocate an elastic
	// public IP for the instance automatically.
	if p.serviceConfig.UsePublicIP {
		bandwidthSize := p.serviceConfig.PublicBandwidthSize
		if bandwidthSize <= 0 {
			bandwidthSize = 100
		}
		req.PublicNetwork = &publicNetwork{
			IPType:        publicIPTypeSingle,
			BandwidthSize: bandwidthSize,
		}
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	result, err := p.client.CreateInstances(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creating instance returned error: %w", err)
	}

	if len(result.InstanceIDs) == 0 {
		return nil, fmt.Errorf("creating instance returned no instance IDs (order: %s)", result.OrderID)
	}

	instanceID := result.InstanceIDs[0]
	logger.Printf("created an instance %s for sandbox %s", instanceID, sandboxID)

	// Create partial instance to return on error (allows caller to cleanup)
	instance = &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
	}

	if err := p.waitForInstanceActive(ctx, instanceID, 5*time.Minute); err != nil {
		return instance, fmt.Errorf("failed to wait for instance %s to be active: %w", instanceID, err)
	}
	logger.Printf("Instance %s is active.", instanceID)

	ips, err := p.getPrivateIPs(ctx, instanceID)
	if err != nil {
		return instance, fmt.Errorf("failed to get IPs for the instance %s: %w", instanceID, err)
	}

	if p.serviceConfig.UsePublicIP {
		publicIP, err := p.getPublicIP(ctx, instanceID)
		if err != nil {
			return instance, fmt.Errorf("failed to get public IP for the instance %s: %w", instanceID, err)
		}
		// insert the public IP address in front of the private IP addresses
		ips = append([]netip.Addr{*publicIP}, ips...)
	}

	instance.IPs = ips

	return instance, nil
}

func (p *ecloudProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	logger.Printf("Deleting instance (%s)", instanceID)

	req := &deleteInstancesRequest{
		InstanceIDs:         []string{instanceID},
		DeletePublicNetwork: true,
		DeleteDataVolumes:   true,
	}

	resp, err := p.client.DeleteInstances(ctx, req)
	if err != nil {
		if isInstanceNotFoundError(err) {
			logger.Printf("instance %s is not found, skipping deletion", instanceID)
			return nil
		}
		return fmt.Errorf("failed to delete an instance %s: %w", instanceID, err)
	}

	for _, result := range resp.InstanceBatchResult {
		if result.InstanceID != instanceID {
			continue
		}
		if !result.Result {
			if isNotFoundMessage(result.Message) {
				logger.Printf("instance %s is not found, skipping deletion", instanceID)
				return nil
			}
			return fmt.Errorf("failed to delete an instance %s: %s", instanceID, result.Message)
		}
	}

	logger.Printf("Deleted an instance %s", instanceID)
	return nil
}

func (p *ecloudProvider) Teardown() error {
	return nil
}

func (p *ecloudProvider) ConfigVerifier() error {
	if len(p.serviceConfig.ImageID) == 0 {
		return fmt.Errorf("ImageID is empty")
	}
	if len(p.serviceConfig.NetworkID) == 0 {
		return fmt.Errorf("NetworkID is empty")
	}
	return nil
}

// selectInstanceType selects an instance type based on the pod annotations.
func (p *ecloudProvider) selectInstanceType(_ context.Context, spec provider.InstanceTypeSpec) (string, error) {
	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceTypeSpecList, p.serviceConfig.InstanceTypes, p.serviceConfig.InstanceType)
}

// updateInstanceTypeSpecList populates the spec list for all configured
// instance types by querying the flavors API.
func (p *ecloudProvider) updateInstanceTypeSpecList(ctx context.Context) error {
	instanceTypes := p.serviceConfig.InstanceTypes
	if len(instanceTypes) == 0 {
		instanceTypes = append(instanceTypes, p.serviceConfig.InstanceType)
	}

	flavors, err := p.client.DescribeFlavors(ctx, p.serviceConfig.ZoneID)
	if err != nil {
		return fmt.Errorf("failed to query flavors: %w", err)
	}

	flavorMap := make(map[string]flavorInfo)
	for _, zoneFlavors := range flavors {
		for _, f := range zoneFlavors.FlavorInfos {
			// The same flavor name can be listed in multiple zones; the
			// resources of a flavor are identical across zones.
			if _, exists := flavorMap[f.FlavorName]; !exists {
				flavorMap[f.FlavorName] = f
			}
		}
	}

	var instanceTypeSpecList []provider.InstanceTypeSpec
	for _, instanceType := range instanceTypes {
		flavor, exists := flavorMap[instanceType]
		if !exists {
			return fmt.Errorf("instance type %s not found in flavors of region %s (zone %s)", instanceType, p.serviceConfig.Region, p.serviceConfig.ZoneID)
		}
		// The flavors API reports memory (ram) in GB; InstanceTypeSpec
		// expresses memory in MiB.
		instanceTypeSpecList = append(instanceTypeSpecList,
			provider.InstanceTypeSpec{
				InstanceType: instanceType,
				VCPUs:        int64(flavor.CPU),
				Memory:       int64(flavor.Ram) * 1024,
			})
	}

	p.serviceConfig.InstanceTypeSpecList = provider.SortInstanceTypesOnResources(instanceTypeSpecList)
	logger.Printf("InstanceTypeSpecList (%v)", p.serviceConfig.InstanceTypeSpecList)
	return nil
}

// getPrivateIPs returns the IPv4 private addresses of the instance.
func (p *ecloudProvider) getPrivateIPs(ctx context.Context, instanceID string) ([]netip.Addr, error) {
	var podNodeIPs []netip.Addr

	err := p.waitUntilTimeout(1*time.Minute, func() (bool, error) {
		resp, err := p.client.DescribeInstance(ctx, instanceID, true)
		if err != nil {
			return false, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
		}
		for _, port := range resp.Ports {
			for _, addr := range port.PrivateIPs {
				ip, err := netip.ParseAddr(addr)
				if err != nil {
					logger.Printf("failed to parse private IP %s of instance %s: %v", addr, instanceID, err)
					continue
				}
				if ip.Is4() {
					podNodeIPs = append(podNodeIPs, ip)
				}
			}
		}
		// The port information may not be initialized yet
		return len(podNodeIPs) > 0, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get private IPs of instance %s: %w", instanceID, err)
	}

	for i, ip := range podNodeIPs {
		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}
	return podNodeIPs, nil
}

// getPublicIP returns the elastic public IPv4 address bound to the instance.
func (p *ecloudProvider) getPublicIP(ctx context.Context, instanceID string) (*netip.Addr, error) {
	var publicIP *netip.Addr

	err := p.waitUntilTimeout(2*time.Minute, func() (bool, error) {
		resp, err := p.client.DescribeInstances(ctx, &describeInstancesRequest{
			InstanceID:  instanceID,
			WithNetwork: true,
		})
		if err != nil {
			logger.Printf("failed to describe instance %s: %v", instanceID, err)
			return false, nil
		}
		if len(resp.Content) == 0 {
			return false, nil
		}
		for _, port := range resp.Content[0].PortDetail {
			if port.FipAddress == "" {
				continue
			}
			ip, err := netip.ParseAddr(port.FipAddress)
			if err != nil {
				logger.Printf("failed to parse public IP %s of instance %s: %v", port.FipAddress, instanceID, err)
				continue
			}
			publicIP = &ip
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP of instance %s: %w", instanceID, err)
	}

	return publicIP, nil
}

// waitForInstanceActive polls the instance status until it becomes active.
func (p *ecloudProvider) waitForInstanceActive(ctx context.Context, instanceID string, timeout time.Duration) error {
	return waitUntilTimeoutWithContext(ctx, timeout, func() (bool, error) {
		resp, err := p.client.DescribeInstanceStatus(ctx, []string{instanceID})
		if err != nil {
			return false, fmt.Errorf("failed to query instance %s status: %w", instanceID, err)
		}
		for _, entry := range resp.Content {
			if entry.InstanceID == instanceID && entry.Status == instanceStatusActive {
				return true, nil
			}
		}
		return false, nil
	})
}

func (p *ecloudProvider) waitUntilTimeout(timeout time.Duration, judgementFunc func() (bool, error)) error {
	return waitUntilTimeoutWithContext(context.Background(), timeout, judgementFunc)
}

func waitUntilTimeoutWithContext(ctx context.Context, timeout time.Duration, judgementFunc func() (bool, error)) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}

	deadline := time.Now().Add(timeout)

	for {
		condition, err := judgementFunc()
		if err != nil {
			return err
		}
		if condition {
			return nil
		}

		if time.Now().After(deadline) {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("exceeded max wait time to wait")
}

func isInstanceNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return isNotFoundMessage(err.Error())
}

func isNotFoundMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "notfound") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "not exist") ||
		strings.Contains(message, "不存在")
}

// InstanceExistsByName checks whether an instance with the given name exists.
// The ecloud API does not support filtering by name, so it pages through all
// instances in the configured zone. It is primarily used by e2e tests to
// assert PodVM creation.
func InstanceExistsByName(ctx context.Context, cfg *Config, instanceName string) (bool, error) {
	client, err := newEcloudClient("", cfg.AccessKeyID, cfg.SecretKey, cfg.Region)
	if err != nil {
		return false, fmt.Errorf("failed to create ecloud client: %w", err)
	}

	const pageSize = 100
	for page := 1; ; page++ {
		resp, err := client.DescribeInstances(ctx, &describeInstancesRequest{
			ZoneID:   cfg.ZoneID,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return false, fmt.Errorf("failed to describe instances: %w", err)
		}
		for _, inst := range resp.Content {
			if inst.InstanceName == instanceName {
				return true, nil
			}
		}
		if len(resp.Content) < pageSize {
			return false, nil
		}
	}
}
