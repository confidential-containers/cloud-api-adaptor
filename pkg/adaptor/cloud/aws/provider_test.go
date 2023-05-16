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
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
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
	// Check if instance type is t2.medium, else return an error
	if instanceType != "t2.medium" {
		return nil, fmt.Errorf("Unsupported instance type")
	}

	// Return a mock DescribeInstanceTypesOutput
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []types.InstanceTypeInfo{
			{
				InstanceType: instanceType,
				// Add vCPU info for t2.medium
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(2),
				},
				// Add memory info for t2.medium
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(4096),
				},
			},
		},
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
		spec        cloud.InstanceTypeSpec
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *cloud.Instance
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
				spec:        cloud.InstanceTypeSpec{InstanceType: "t2.small"},
			},
			want: &cloud.Instance{
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
				spec:        cloud.InstanceTypeSpec{InstanceType: "t2.small"},
			},
			want: &cloud.Instance{
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
				spec:        cloud.InstanceTypeSpec{InstanceType: "t2.small"},
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
				spec:        cloud.InstanceTypeSpec{InstanceType: "t2.large"},
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
				spec:        cloud.InstanceTypeSpec{InstanceType: ""},
			},
			want: &cloud.Instance{
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
				spec:        cloud.InstanceTypeSpec{InstanceType: ""},
			},
			want: &cloud.Instance{
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
			gotVcpu, gotMemory, err := p.getInstanceTypeInformation(tt.args.instanceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("awsProvider.getInstanceTypeInformation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotVcpu != tt.wantVcpu {
				t.Errorf("awsProvider.getInstanceTypeInformation() gotVcpu = %v, want %v", gotVcpu, tt.wantVcpu)
			}
			if gotMemory != tt.wantMemory {
				t.Errorf("awsProvider.getInstanceTypeInformation() gotMemory = %v, want %v", gotMemory, tt.wantMemory)
			}
		})
	}
}
