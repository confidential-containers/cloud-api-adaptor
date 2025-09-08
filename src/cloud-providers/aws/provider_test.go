// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

// Mock EC2 API
type mockEC2Client struct{}

// Return a new mock EC2 API
func newMockEC2Client() *mockEC2Client {
	return &mockEC2Client{}
}

// Create a mock EC2 RunInstances method
func (m mockEC2Client) RunInstances(ctx context.Context,
	params *ec2.RunInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {

	// Create a mock instance ID
	mockInstanceID := "i-1234567890abcdef0"
	// Return a mock RunInstancesOutput
	return &ec2.RunInstancesOutput{
		Instances: []types.Instance{
			{
				InstanceId: &mockInstanceID,
				// Add public DNS name
				PublicDnsName: aws.String("ec2-192-168-100-1.compute-1.amazonaws.com"),
				// Add private IP address to mock instance
				PrivateIpAddress: aws.String("10.0.0.2"),
				// Add private IP address to network interface
				NetworkInterfaces: []types.InstanceNetworkInterface{
					{
						PrivateIpAddress: aws.String("10.0.0.2"),
					},
				},
			},
		},
	}, nil
}

// Create a mock EC2 TerminateInstances method
func (m mockEC2Client) TerminateInstances(ctx context.Context,
	params *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {

	// Return a mock TerminateInstancesOutput
	return &ec2.TerminateInstancesOutput{}, nil
}

// Create a mock EC2 DescribeInstanceTypes method
func (m mockEC2Client) DescribeInstanceTypes(ctx context.Context,
	params *ec2.DescribeInstanceTypesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {

	// Take instance type from params
	instanceType := params.InstanceTypes[0]

	var instanceInfo types.InstanceTypeInfo

	switch instanceType {
	case "t2.medium":
		instanceInfo = types.InstanceTypeInfo{
			InstanceType: types.InstanceTypeT2Medium,
			VCpuInfo: &types.VCpuInfo{
				DefaultVCpus: aws.Int32(2),
			},
			MemoryInfo: &types.MemoryInfo{
				SizeInMiB: aws.Int64(4096),
			},
		}
	case "p3.8xlarge":
		instanceInfo = types.InstanceTypeInfo{
			InstanceType: types.InstanceTypeP38xlarge,
			VCpuInfo: &types.VCpuInfo{
				DefaultVCpus: aws.Int32(32),
			},
			MemoryInfo: &types.MemoryInfo{
				SizeInMiB: aws.Int64(244000),
			},
			GpuInfo: &types.GpuInfo{
				Gpus: []types.GpuDeviceInfo{
					{
						Count:        aws.Int32(4),
						Manufacturer: aws.String("NVIDIA"),
						Name:         aws.String("Tesla"),
					},
				},
			},
		}
	default:
		return nil, fmt.Errorf("Unsupported instance type: %s", instanceType)
	}

	// Return a mock DescribeInstanceTypesOutput with only the requested instance type
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []types.InstanceTypeInfo{instanceInfo},
	}, nil
}

// Create a mock EC2 DescribeInstances method
func (m mockEC2Client) DescribeInstances(ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {

	// Create a mock instance ID
	mockInstanceID := "i-1234567890abcdef0"
	// Return a mock DescribeInstancesOutput
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: &mockInstanceID,
						// Add private IP address to mock instance
						PrivateIpAddress: aws.String("10.0.0.2"),
						// Add private IP address to network interface
						NetworkInterfaces: []types.InstanceNetworkInterface{
							{
								PrivateIpAddress: aws.String("10.0.0.2"),
								// Add public IP address to network interface
								Association: &types.InstanceNetworkInterfaceAssociation{
									PublicIp:      aws.String("192.168.100.1"),
									PublicDnsName: aws.String("ec2-192-168-100-1.compute-1.amazonaws.com"),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// Create a mock for EC2 AllocateAddress method
func (m mockEC2Client) AllocateAddress(ctx context.Context,
	params *ec2.AllocateAddressInput,
	optFns ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error) {

	return nil, nil
}

// Create a mock for EC2 AssociateAddress method
func (m mockEC2Client) AssociateAddress(ctx context.Context,
	params *ec2.AssociateAddressInput,
	optFns ...func(*ec2.Options)) (*ec2.AssociateAddressOutput, error) {

	return nil, nil
}

// Create a mock for EC2 DescribeAddresses method
func (m mockEC2Client) DescribeAddresses(ctx context.Context,
	params *ec2.DescribeAddressesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {

	// Return a mock Elastic IP address
	mockAssociationId := "eipassoc-12345678"
	mockAllocationId := "eipalloc-12345678"
	mockPublicIp := "192.168.100.1"

	return &ec2.DescribeAddressesOutput{
		Addresses: []types.Address{
			{
				AssociationId: &mockAssociationId,
				AllocationId:  &mockAllocationId,
				PublicIp:      &mockPublicIp,
				InstanceId:    aws.String("i-1234567890abcdef0"),
			},
		},
	}, nil
}

// Create a mock for EC2 DisassociateAddress method
func (m mockEC2Client) DisassociateAddress(ctx context.Context,
	params *ec2.DisassociateAddressInput,
	optFns ...func(*ec2.Options)) (*ec2.DisassociateAddressOutput, error) {

	return nil, nil
}

// Create a mock for EC2 ReleaseAddress method
func (m mockEC2Client) ReleaseAddress(ctx context.Context,
	params *ec2.ReleaseAddressInput,
	optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {

	return nil, nil
}

// Create a mock for EC2 CreateNetworkInterface method
func (m mockEC2Client) CreateNetworkInterface(ctx context.Context,
	params *ec2.CreateNetworkInterfaceInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInterfaceOutput, error) {

	return nil, nil
}

// Create a mock for EC2 AttachNetworkInterface method
func (m mockEC2Client) AttachNetworkInterface(ctx context.Context,
	params *ec2.AttachNetworkInterfaceInput,
	optFns ...func(*ec2.Options)) (*ec2.AttachNetworkInterfaceOutput, error) {

	return nil, nil
}

// Create a mock for EC2 DeleteNetworkInterface method
func (m mockEC2Client) DeleteNetworkInterface(ctx context.Context,
	params *ec2.DeleteNetworkInterfaceInput,
	optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInterfaceOutput, error) {

	return nil, nil
}

// Create a mock for EC2 ModifyNetworkInterfaceAttribute method
func (m mockEC2Client) ModifyNetworkInterfaceAttribute(ctx context.Context,
	params *ec2.ModifyNetworkInterfaceAttributeInput,
	optFns ...func(*ec2.Options)) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {

	return nil, nil
}

// Mock instanceRunningWaiter
type MockAWSInstanceWaiter struct{}

// Return a new mock waiter
func (m *MockAWSInstanceWaiter) Wait(ctx context.Context, params *ec2.DescribeInstancesInput, maxWaitDur time.Duration, optFns ...func(*ec2.InstanceRunningWaiterOptions)) error {

	return nil
}

// Create a new mock AWSInstanceWaiter
func newMockAWSInstanceWaiter() *MockAWSInstanceWaiter {
	return &MockAWSInstanceWaiter{}
}

// Create a mock EC2 DescribeImages method
func (m mockEC2Client) DescribeImages(ctx context.Context,
	params *ec2.DescribeImagesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {

	// Create a mock image ID
	mockImageID := "ami-1234567890abcdef0"
	// Return a mock DescribeImagesOutput
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId: &mockImageID,
			},
		},
	}, nil
}

// Create a serviceConfig struct without public IP
var serviceConfig = &Config{
	Region: "us-east-1",
	// Add instance type to serviceConfig
	InstanceType: "t2.small",
	// Add subnet ID to serviceConfig
	SubnetId: "subnet-1234567890abcdef0",
	// Add security group ID to serviceConfig
	SecurityGroupIds: []string{"sg-1234567890abcdef0"},
	// Add image ID to serviceConfig
	ImageId: "ami-1234567890abcdef0",
	// Add InstanceTypes to serviceConfig
	InstanceTypes: []string{"t2.small", "t2.medium"},
}

// Create a serviceConfig struct with public IP
var serviceConfigPublicIP = &Config{
	Region: "us-east-1",
	// Add instance type to serviceConfig
	InstanceType: "t2.small",
	// Add subnet ID to serviceConfig
	SubnetId: "subnet-1234567890abcdef0",
	// Add security group ID to serviceConfig
	SecurityGroupIds: []string{"sg-1234567890abcdef0"},
	// Add image ID to serviceConfig
	ImageId: "ami-1234567890abcdef0",
	// Add InstanceTypes to serviceConfig
	InstanceTypes: []string{"t2.small", "t2.medium"},
	// Add public IP to serviceConfig
	UsePublicIP: true,
}

// Create a serviceConfig struct with invalid instance type
var serviceConfigInvalidInstanceType = &Config{
	Region: "us-east-1",
	// Add instance type to serviceConfig
	InstanceType: "t2.small",
	// Add subnet ID to serviceConfig
	SubnetId: "subnet-1234567890abcdef0",
	// Add security group ID to serviceConfig
	SecurityGroupIds: []string{"sg-1234567890abcdef0"},
	// Add image ID to serviceConfig
	ImageId: "ami-1234567890abcdef0",
	// Add InstanceTypes to serviceConfig
	InstanceTypes: []string{"t2.large", "t2.medium"},
}

// Create a serviceConfig with emtpy InstanceTypes
var serviceConfigEmptyInstanceTypes = &Config{
	Region: "us-east-1",
	// Add instance type to serviceConfig
	InstanceType: "t2.small",
	// Add subnet ID to serviceConfig
	SubnetId: "subnet-1234567890abcdef0",
	// Add security group ID to serviceConfig
	SecurityGroupIds: []string{"sg-1234567890abcdef0"},
	// Add image ID to serviceConfig
	ImageId: "ami-1234567890abcdef0",
	// Add InstanceTypes to serviceConfig
	InstanceTypes: []string{},
}

// Create a serviceConfig with empty ImageId
var serviceConfigEmptyImageId = &Config{
	Region: "us-east-1",
	// Add instance type to serviceConfig
	InstanceType: "t2.small",
	// Add subnet ID to serviceConfig
	SubnetId: "subnet-1234567890abcdef0",
	// Add security group ID to serviceConfig
	SecurityGroupIds: []string{"sg-1234567890abcdef0"},
	// Add image ID to serviceConfig
	ImageId: "",
	// Add InstanceTypes to serviceConfig
	InstanceTypes: []string{"t2.large", "t2.medium"},
}

type mockCloudConfig struct{}

func (c *mockCloudConfig) Generate() (string, error) {
	return "cloud config", nil
}

func TestCreateInstance(t *testing.T) {
	type fields struct {
		ec2Client     ec2Client
		waiter        *MockAWSInstanceWaiter
		serviceConfig *Config
	}
	type args struct {
		ctx         context.Context
		podName     string
		sandboxID   string
		cloudConfig cloudinit.CloudConfigGenerator
		spec        provider.InstanceTypeSpec
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *provider.Instance
		wantErr bool
	}{
		// Test creating an instance
		{
			name: "CreateInstance",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfig to fields
				serviceConfig: serviceConfig,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podtest",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: "t2.small"},
			},
			want: &provider.Instance{
				ID:   "i-1234567890abcdef0",
				Name: "podvm-podtest-123",
				IPs:  []netip.Addr{netip.MustParseAddr("10.0.0.2")},
			},
			// Test should not return an error
			wantErr: false,
		},
		// Test creating an instance with public IP
		{
			name: "CreateInstancePublicIP",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfigPublicIP to fields
				serviceConfig: serviceConfigPublicIP,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podpublicip",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: "t2.small"},
			},
			want: &provider.Instance{
				ID:   "i-1234567890abcdef0",
				Name: "podvm-podpublicip-123",
				IPs:  []netip.Addr{netip.MustParseAddr("192.168.100.1")},
			},
			// Test should not return an error
			wantErr: false,
		},
		// Test creating an instance with invalid instanceType
		{
			name: "CreateInstanceInvalidInstanceType",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfigInvalidInstanceType to fields
				serviceConfig: serviceConfigInvalidInstanceType,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podinvalidinstance",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: "t2.small"},
			},
			want: nil,
			// Test should return an error
			wantErr: true,
		},
		// Test creating an instance with empty instanceTypes and instanceType set to non-default value
		{
			name: "CreateInstanceEmptyInstanceTypes",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfigEmptyInstanceTypes to fields
				serviceConfig: serviceConfigEmptyInstanceTypes,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podemptyinstance",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: "t2.large"},
			},
			want: nil,
			// Test should return an error
			wantErr: true,
		},
		// Test creating an instance with empty instanceTypes and instanceType
		{
			name: "CreateInstanceEmptyInstanceTypeAndTypes",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfigEmptyInstanceTypes to fields
				serviceConfig: serviceConfigEmptyInstanceTypes,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podemptyinstance",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: ""},
			},
			want: &provider.Instance{
				ID:   "i-1234567890abcdef0",
				Name: "podvm-podemptyinstance-123",
				IPs:  []netip.Addr{netip.MustParseAddr("10.0.0.2")},
			},
			// Test should not return an error
			wantErr: false,
		},
		// Test creating an instance with empty instanceType
		// The instanceType is set to default value
		{
			name: "CreateInstanceEmptyInstanceType",
			// Add fields to test
			fields: fields{
				// Add mock EC2 client to fields
				ec2Client: newMockEC2Client(),
				// Add mock waiter to fields
				waiter: newMockAWSInstanceWaiter(),
				// Add serviceConfigEmptyInstanceType to fields
				serviceConfig: serviceConfig,
			},
			args: args{
				ctx:         context.Background(),
				podName:     "podemptyinstance",
				sandboxID:   "123",
				cloudConfig: &mockCloudConfig{},
				spec:        provider.InstanceTypeSpec{InstanceType: ""},
			},
			want: &provider.Instance{
				ID:   "i-1234567890abcdef0",
				Name: "podvm-podemptyinstance-123",
				IPs:  []netip.Addr{netip.MustParseAddr("10.0.0.2")},
			},
			// Test should not return an error
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			p := &awsProvider{
				ec2Client:     tt.fields.ec2Client,
				waiter:        tt.fields.waiter,
				serviceConfig: tt.fields.serviceConfig,
			}

			got, err := p.CreateInstance(tt.args.ctx, tt.args.podName, tt.args.sandboxID, tt.args.cloudConfig, tt.args.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("awsProvider.CreateInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("awsProvider.CreateInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteInstance(t *testing.T) {
	type fields struct {
		ec2Client     ec2Client
		serviceConfig *Config
	}
	type args struct {
		ctx        context.Context
		instanceID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// Test deleting an instance
		{
			name: "DeleteInstance",
			fields: fields{
				ec2Client:     newMockEC2Client(),
				serviceConfig: serviceConfig,
			},
			args: args{
				ctx:        context.Background(),
				instanceID: "i-1234567890abcdef0",
			},
			// Test should not return an error
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &awsProvider{
				ec2Client:     tt.fields.ec2Client,
				serviceConfig: tt.fields.serviceConfig,
			}
			if err := p.DeleteInstance(tt.args.ctx, tt.args.instanceID); (err != nil) != tt.wantErr {
				t.Errorf("awsProvider.DeleteInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetInstanceTypeInformation(t *testing.T) {
	type fields struct {
		ec2Client     ec2Client
		serviceConfig *Config
	}
	type args struct {
		instanceType string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantVcpu   int64
		wantMemory int64
		wantGpu    int64
		wantErr    bool
	}{
		// Test getting instance type information for a valid instance type
		{
			name: "getInstanceTypeInformationValidInstanceType",
			fields: fields{
				ec2Client:     newMockEC2Client(),
				serviceConfig: serviceConfig,
			},
			args: args{
				instanceType: "t2.medium",
			},
			wantVcpu:   2,
			wantMemory: 4096,
			wantGpu:    0,
			// Test should not return an error
			wantErr: false,
		},
		// Test getting instance type information for a valid instance type
		{
			name: "getInstanceTypeInformationValidInstanceType",
			fields: fields{
				ec2Client:     newMockEC2Client(),
				serviceConfig: serviceConfig,
			},
			args: args{
				instanceType: "p3.8xlarge",
			},
			wantVcpu:   32,
			wantMemory: 244000,
			wantGpu:    4,
			// Test should not return an error
			wantErr: false,
		},
		// Test getting instance type information for an invalid instance type
		{
			name: "getInstanceTypeInformationInvalidInstanceType",
			fields: fields{
				ec2Client:     newMockEC2Client(),
				serviceConfig: serviceConfig,
			},
			args: args{
				instanceType: "mycustominstance",
			},
			wantVcpu:   0,
			wantMemory: 0,
			wantGpu:    0,
			// Test should return an error
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &awsProvider{
				ec2Client:     tt.fields.ec2Client,
				serviceConfig: tt.fields.serviceConfig,
			}
			gotVcpu, gotMemory, gotGpu, err := p.getInstanceTypeInformation(tt.args.instanceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("awsProvider.getInstanceTypeInformation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("Instance info => type: %s, vcpu: %d, mem: %d, gpu: %d",
				tt.args.instanceType, gotVcpu, gotMemory, gotGpu)

			if gotVcpu != tt.wantVcpu {
				t.Errorf("awsProvider.getInstanceTypeInformation() gotVcpu = %v, want %v", gotVcpu, tt.wantVcpu)
			}
			if gotMemory != tt.wantMemory {
				t.Errorf("awsProvider.getInstanceTypeInformation() gotMemory = %v, want %v", gotMemory, tt.wantMemory)
			}
			if gotGpu != tt.wantGpu {
				t.Errorf("awsProvider.getInstanceTypeInformation() gotGpu = %v, want %v", gotGpu, tt.wantGpu)
			}
		})
	}
}

func TestConfigVerifier(t *testing.T) {
	type fields struct {
		serviceConfig *Config
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// Test check with valid ImageId
		{
			name: "checkValidImageId",
			fields: fields{
				serviceConfig: serviceConfig,
			},
			wantErr: false,
		},
		// Test check with invalid ImageId
		{
			name: "checkInvalidImageId",
			fields: fields{
				serviceConfig: serviceConfigEmptyImageId,
			},
			// Test should return an error
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &awsProvider{
				serviceConfig: tt.fields.serviceConfig,
			}
			err := p.ConfigVerifier()
			if tt.wantErr {
				if err == nil {
					t.Errorf("awsProvider.ConfigVerifier() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("awsProvider.ConfigVerifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
