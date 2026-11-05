// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

// mockEcloudClient implements ecsClient for testing.
type mockEcloudClient struct {
	createInstancesFunc         func(ctx context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error)
	deleteInstancesFunc         func(ctx context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error)
	describeInstanceFunc        func(ctx context.Context, instanceID string, moreDetail bool) (*describeInstanceResponseBody, error)
	describeInstancesFunc       func(ctx context.Context, req *describeInstancesRequest) (*describeInstancesResponseBody, error)
	describeInstanceStatusFunc  func(ctx context.Context, instanceIDs []string) (*describeInstanceStatusResponseBody, error)
	describeFlavorsFunc         func(ctx context.Context, zoneID string) ([]flavorsResponseBody, error)
}

func (m *mockEcloudClient) CreateInstances(ctx context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
	return m.createInstancesFunc(ctx, req)
}

func (m *mockEcloudClient) DeleteInstances(ctx context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
	return m.deleteInstancesFunc(ctx, req)
}

func (m *mockEcloudClient) DescribeInstance(ctx context.Context, instanceID string, moreDetail bool) (*describeInstanceResponseBody, error) {
	return m.describeInstanceFunc(ctx, instanceID, moreDetail)
}

func (m *mockEcloudClient) DescribeInstances(ctx context.Context, req *describeInstancesRequest) (*describeInstancesResponseBody, error) {
	return m.describeInstancesFunc(ctx, req)
}

func (m *mockEcloudClient) DescribeInstanceStatus(ctx context.Context, instanceIDs []string) (*describeInstanceStatusResponseBody, error) {
	return m.describeInstanceStatusFunc(ctx, instanceIDs)
}

func (m *mockEcloudClient) DescribeFlavors(ctx context.Context, zoneID string) ([]flavorsResponseBody, error) {
	return m.describeFlavorsFunc(ctx, zoneID)
}

func newTestConfig() *Config {
	return &Config{
		AccessKeyID:      "ak",
		SecretKey:        "sk",
		Region:           "cn-jiangsu-1",
		ZoneID:           "cn-jiangsu-1a",
		ImageID:          "image-1",
		InstanceType:     "s1.large.2",
		NetworkID:        "network-1",
		SecurityGroupIDs: securityGroupIDs{"sg-1"},
		SystemDiskSize:   40,
		SystemDiskType:   "highPerformance",
	}
}

func newMockClient() *mockEcloudClient {
	return &mockEcloudClient{
		createInstancesFunc: func(_ context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
			return &createInstancesResponseBody{
				ProcedureCode: "POSTPAID",
				OrderID:       "MOP-O-1",
				InstanceIDs:   []string{"instance-id-1"},
			}, nil
		},
		deleteInstancesFunc: func(_ context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
			results := make([]instanceBatchResult, 0, len(req.InstanceIDs))
			for _, id := range req.InstanceIDs {
				results = append(results, instanceBatchResult{InstanceID: id, Result: true})
			}
			return &deleteInstancesResponseBody{InstanceBatchResult: results}, nil
		},
		describeInstanceFunc: func(_ context.Context, instanceID string, moreDetail bool) (*describeInstanceResponseBody, error) {
			return &describeInstanceResponseBody{
				ID:     instanceID,
				Status: "active",
				Ports: []instancePort{
					{ID: "port-1", PrivateIPs: []string{"192.168.1.10"}},
				},
			}, nil
		},
		describeInstancesFunc: func(_ context.Context, req *describeInstancesRequest) (*describeInstancesResponseBody, error) {
			return &describeInstancesResponseBody{
				Total: 1,
				Content: []describeInstancesContent{
					{
						InstanceID: req.InstanceID,
						Status:     "active",
						PortDetail: []portDetail{
							{PrivateIP: "192.168.1.10", FipAddress: "123.10.20.1"},
						},
					},
				},
			}, nil
		},
		describeInstanceStatusFunc: func(_ context.Context, instanceIDs []string) (*describeInstanceStatusResponseBody, error) {
			content := make([]instanceStatusEntry, 0, len(instanceIDs))
			for _, id := range instanceIDs {
				content = append(content, instanceStatusEntry{InstanceID: id, Status: "active"})
			}
			return &describeInstanceStatusResponseBody{Total: len(content), Content: content}, nil
		},
		describeFlavorsFunc: func(_ context.Context, zoneID string) ([]flavorsResponseBody, error) {
			return []flavorsResponseBody{
				{
					ZoneID: "cn-jiangsu-1a",
					FlavorInfos: []flavorInfo{
						{VMType: "common", CPU: 2, Ram: 4, FlavorName: "s1.large.2", FlavorType: "s1"},
						{VMType: "common", CPU: 4, Ram: 8, FlavorName: "s1.xlarge.2", FlavorType: "s1"},
					},
				},
			}, nil
		},
	}
}

type testCloudConfigGenerator struct{}

func (t *testCloudConfigGenerator) Generate() (string, error) {
	return "#cloud-config\ntest", nil
}

var _ cloudinit.CloudConfigGenerator = (*testCloudConfigGenerator)(nil)

func TestCreateInstance(t *testing.T) {
	p := &ecloudProvider{
		client:        newMockClient(),
		serviceConfig: newTestConfig(),
	}

	instance, err := p.CreateInstance(context.Background(), "podname", "sandbox-123", &testCloudConfigGenerator{}, provider.InstanceTypeSpec{})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if instance.ID != "instance-id-1" {
		t.Errorf("instance.ID = %q, expect %q", instance.ID, "instance-id-1")
	}
	if !strings.HasPrefix(instance.Name, "podvm-podname") {
		t.Errorf("instance.Name = %q, expect prefix podvm-podname", instance.Name)
	}
	if len(instance.IPs) != 1 || instance.IPs[0].String() != "192.168.1.10" {
		t.Errorf("instance.IPs = %v", instance.IPs)
	}
}

func TestCreateInstanceRequestContents(t *testing.T) {
	var capturedReq *createInstancesRequest
	mock := newMockClient()
	mock.createInstancesFunc = func(_ context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
		capturedReq = req
		return &createInstancesResponseBody{InstanceIDs: []string{"instance-id-1"}}, nil
	}

	cfg := newTestConfig()
	cfg.KeyName = "keypair-1"
	cfg.Tags = provider.KeyValueFlag{"team": "coco"}

	p := &ecloudProvider{client: mock, serviceConfig: cfg}

	_, err := p.CreateInstance(context.Background(), "podname", "sandbox-123", &testCloudConfigGenerator{}, provider.InstanceTypeSpec{
		Image: "image-from-annotation",
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("create request was not captured")
	}
	if capturedReq.ImageID != "image-from-annotation" {
		t.Errorf("ImageID = %q, expect image-from-annotation", capturedReq.ImageID)
	}
	if capturedReq.FlavorName != "s1.large.2" {
		t.Errorf("FlavorName = %q, expect s1.large.2", capturedReq.FlavorName)
	}
	if capturedReq.ChargeMode != "HOUR" {
		t.Errorf("ChargeMode = %q, expect HOUR", capturedReq.ChargeMode)
	}
	if capturedReq.KeyPairName != "keypair-1" {
		t.Errorf("KeyPairName = %q, expect keypair-1", capturedReq.KeyPairName)
	}
	if capturedReq.PrivateNetwork == nil || capturedReq.PrivateNetwork.NetworkID != "network-1" {
		t.Errorf("PrivateNetwork = %+v", capturedReq.PrivateNetwork)
	}
	if capturedReq.BootVolume == nil || capturedReq.BootVolume.Size != 40 || capturedReq.BootVolume.VolumeType != "highPerformance" {
		t.Errorf("BootVolume = %+v", capturedReq.BootVolume)
	}
	if capturedReq.PublicNetwork != nil {
		t.Errorf("PublicNetwork = %+v, expect nil", capturedReq.PublicNetwork)
	}
	// userData must be base64 encoded
	decoded, err := base64.StdEncoding.DecodeString(capturedReq.UserData)
	if err != nil || string(decoded) != "#cloud-config\ntest" {
		t.Errorf("UserData = %q, decode error: %v", capturedReq.UserData, err)
	}
	if len(capturedReq.Tags) != 1 || capturedReq.Tags[0].Key != "team" {
		t.Errorf("Tags = %+v", capturedReq.Tags)
	}
}

func TestCreateInstanceWithPublicIP(t *testing.T) {
	cfg := newTestConfig()
	cfg.UsePublicIP = true

	p := &ecloudProvider{
		client:        newMockClient(),
		serviceConfig: cfg,
	}

	instance, err := p.CreateInstance(context.Background(), "podname", "sandbox-123", &testCloudConfigGenerator{}, provider.InstanceTypeSpec{})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// The public IP must be the first address
	if len(instance.IPs) != 2 {
		t.Fatalf("instance.IPs = %v", instance.IPs)
	}
	if instance.IPs[0].String() != "123.10.20.1" {
		t.Errorf("instance.IPs[0] = %s, expect public IP 123.10.20.1", instance.IPs[0])
	}
	if instance.IPs[1].String() != "192.168.1.10" {
		t.Errorf("instance.IPs[1] = %s, expect private IP 192.168.1.10", instance.IPs[1])
	}
}

func TestCreateInstanceSelectsInstanceTypeFromSpecList(t *testing.T) {
	var capturedReq *createInstancesRequest
	mock := newMockClient()
	mock.createInstancesFunc = func(_ context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
		capturedReq = req
		return &createInstancesResponseBody{InstanceIDs: []string{"instance-id-1"}}, nil
	}

	cfg := newTestConfig()
	cfg.InstanceTypes = instanceTypes{"s1.large.2", "s1.xlarge.2"}

	p := &ecloudProvider{client: mock, serviceConfig: cfg}
	if err := p.updateInstanceTypeSpecList(context.Background()); err != nil {
		t.Fatalf("updateInstanceTypeSpecList failed: %v", err)
	}

	// Requesting 4 vCPUs and 8GiB must select s1.xlarge.2
	_, err := p.CreateInstance(context.Background(), "podname", "sandbox-123", &testCloudConfigGenerator{}, provider.InstanceTypeSpec{
		VCPUs:  4,
		Memory: 8192,
	})
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if capturedReq.FlavorName != "s1.xlarge.2" {
		t.Errorf("FlavorName = %q, expect s1.xlarge.2", capturedReq.FlavorName)
	}
}

func TestCreateInstanceFailsWithoutInstanceIDs(t *testing.T) {
	mock := newMockClient()
	mock.createInstancesFunc = func(_ context.Context, req *createInstancesRequest) (*createInstancesResponseBody, error) {
		return &createInstancesResponseBody{}, nil
	}

	p := &ecloudProvider{client: mock, serviceConfig: newTestConfig()}

	_, err := p.CreateInstance(context.Background(), "podname", "sandbox-123", &testCloudConfigGenerator{}, provider.InstanceTypeSpec{})
	if err == nil {
		t.Fatal("expect an error when no instance IDs are returned")
	}
}

func TestDeleteInstance(t *testing.T) {
	var capturedReq *deleteInstancesRequest
	mock := newMockClient()
	mock.deleteInstancesFunc = func(_ context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
		capturedReq = req
		return &deleteInstancesResponseBody{
			InstanceBatchResult: []instanceBatchResult{{InstanceID: req.InstanceIDs[0], Result: true}},
		}, nil
	}

	p := &ecloudProvider{client: mock, serviceConfig: newTestConfig()}

	if err := p.DeleteInstance(context.Background(), "instance-id-1"); err != nil {
		t.Fatalf("DeleteInstance failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("delete request was not captured")
	}
	if len(capturedReq.InstanceIDs) != 1 || capturedReq.InstanceIDs[0] != "instance-id-1" {
		t.Errorf("InstanceIDs = %v", capturedReq.InstanceIDs)
	}
	if !capturedReq.DeletePublicNetwork || !capturedReq.DeleteDataVolumes {
		t.Errorf("expect DeletePublicNetwork and DeleteDataVolumes to be true: %+v", capturedReq)
	}
}

func TestDeleteInstanceNotFound(t *testing.T) {
	mock := newMockClient()
	mock.deleteInstancesFunc = func(_ context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
		return &deleteInstancesResponseBody{
			InstanceBatchResult: []instanceBatchResult{{InstanceID: req.InstanceIDs[0], Result: false, Message: "实例不存在"}},
		}, nil
	}

	p := &ecloudProvider{client: mock, serviceConfig: newTestConfig()}

	// Deleting a non-existent instance must be a no-op
	if err := p.DeleteInstance(context.Background(), "instance-id-1"); err != nil {
		t.Fatalf("DeleteInstance failed: %v", err)
	}
}

func TestDeleteInstanceError(t *testing.T) {
	mock := newMockClient()
	mock.deleteInstancesFunc = func(_ context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
		return &deleteInstancesResponseBody{
			InstanceBatchResult: []instanceBatchResult{{InstanceID: req.InstanceIDs[0], Result: false, Message: "quota exceeded"}},
		}, nil
	}

	p := &ecloudProvider{client: mock, serviceConfig: newTestConfig()}

	if err := p.DeleteInstance(context.Background(), "instance-id-1"); err == nil {
		t.Fatal("expect an error when deletion fails")
	}
}

func TestDeleteInstanceAPIError(t *testing.T) {
	mock := newMockClient()
	mock.deleteInstancesFunc = func(_ context.Context, req *deleteInstancesRequest) (*deleteInstancesResponseBody, error) {
		return nil, fmt.Errorf("request POST /api/openapi-instance/v4/delete-instances returned error InvalidInstanceId.NotFound: instance not found")
	}

	p := &ecloudProvider{client: mock, serviceConfig: newTestConfig()}

	if err := p.DeleteInstance(context.Background(), "instance-id-1"); err != nil {
		t.Fatalf("DeleteInstance must tolerate not-found errors: %v", err)
	}
}

func TestUpdateInstanceTypeSpecList(t *testing.T) {
	cfg := newTestConfig()
	cfg.InstanceTypes = instanceTypes{"s1.xlarge.2", "s1.large.2"}

	p := &ecloudProvider{client: newMockClient(), serviceConfig: cfg}
	if err := p.updateInstanceTypeSpecList(context.Background()); err != nil {
		t.Fatalf("updateInstanceTypeSpecList failed: %v", err)
	}

	specList := p.serviceConfig.InstanceTypeSpecList
	if len(specList) != 2 {
		t.Fatalf("InstanceTypeSpecList = %v", specList)
	}
	// The list must be sorted in ascending order of resources
	if specList[0].InstanceType != "s1.large.2" {
		t.Errorf("specList[0] = %+v, expect s1.large.2 first", specList[0])
	}
	if specList[0].VCPUs != 2 || specList[0].Memory != 4*1024 {
		t.Errorf("specList[0] resources = %dC/%dMiB", specList[0].VCPUs, specList[0].Memory)
	}
	if specList[1].VCPUs != 4 || specList[1].Memory != 8*1024 {
		t.Errorf("specList[1] resources = %dC/%dMiB", specList[1].VCPUs, specList[1].Memory)
	}
}

func TestUpdateInstanceTypeSpecListUnknownFlavor(t *testing.T) {
	cfg := newTestConfig()
	cfg.InstanceTypes = instanceTypes{"does.not.exist"}

	p := &ecloudProvider{client: newMockClient(), serviceConfig: cfg}
	if err := p.updateInstanceTypeSpecList(context.Background()); err == nil {
		t.Fatal("expect an error for an unknown flavor")
	}
}

func TestConfigVerifier(t *testing.T) {
	p := &ecloudProvider{serviceConfig: newTestConfig()}
	if err := p.ConfigVerifier(); err != nil {
		t.Fatalf("ConfigVerifier failed: %v", err)
	}

	p.serviceConfig.ImageID = ""
	if err := p.ConfigVerifier(); err == nil {
		t.Fatal("expect an error when ImageID is empty")
	}

	p.serviceConfig.ImageID = "image-1"
	p.serviceConfig.NetworkID = ""
	if err := p.ConfigVerifier(); err == nil {
		t.Fatal("expect an error when NetworkID is empty")
	}
}
