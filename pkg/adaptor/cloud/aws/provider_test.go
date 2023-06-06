// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"net"
	"reflect"
	"testing"

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

// Create a mock CreateTags method
func (m mockEC2Client) CreateTags(ctx context.Context,
	params *ec2.CreateTagsInput,
	optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {

	// Return a mock CreateTagsOutput
	return &ec2.CreateTagsOutput{}, nil
}

// Create a mock EC2 TerminateInstances method
func (m mockEC2Client) TerminateInstances(ctx context.Context,
	params *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {

	// Return a mock TerminateInstancesOutput
	return &ec2.TerminateInstancesOutput{}, nil
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
}

type mockCloudConfig struct{}

func (c *mockCloudConfig) Generate() (string, error) {
	return "cloud config", nil
}

func TestCreateInstance(t *testing.T) {
	type fields struct {
		ec2Client     ec2Client
		serviceConfig *Config
	}
	type args struct {
		ctx          context.Context
		podName      string
		sandboxID    string
		cloudConfig  cloudinit.CloudConfigGenerator
		instanceType string
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
				// Add serviceConfig to fields
				serviceConfig: serviceConfig,
			},
			args: args{
				ctx:          context.Background(),
				podName:      "podtest",
				sandboxID:    "123",
				cloudConfig:  &mockCloudConfig{},
				instanceType: "t2.small",
			},
			want: &cloud.Instance{
				ID:   "i-1234567890abcdef0",
				Name: "podvm-podtest-123",
				IPs:  []net.IP{net.ParseIP("10.0.0.2")},
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

			got, err := p.CreateInstance(tt.args.ctx, tt.args.podName, tt.args.sandboxID, tt.args.cloudConfig)
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
