// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/stretchr/testify/assert"
)

type mockVPC struct {
	prototype vpcv1.InstancePrototypeIntf
}

func ptr(s string) *string {
	return &s
}

func (v *mockVPC) CreateInstanceWithContext(ctx context.Context, opt *vpcv1.CreateInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error) {

	v.prototype = opt.InstancePrototype

	instance := &vpcv1.Instance{
		ID: ptr("123"),
		PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
			ID: ptr("111"),
			PrimaryIP: &vpcv1.ReservedIPReference{
				Address:      ptr("0.0.0.0"),
				Href:         ptr("href"),
				ID:           ptr("id"),
				Name:         ptr("name"),
				ResourceType: ptr("resource type"),
			},
		},
	}
	return instance, nil, nil
}

func (v *mockVPC) GetInstanceWithContext(ctx context.Context, opt *vpcv1.GetInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error) {

	instance := &vpcv1.Instance{
		ID: ptr("123"),
		PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
			ID: ptr("111"),
			PrimaryIP: &vpcv1.ReservedIPReference{
				Address:      ptr("192.0.1.1"),
				Href:         ptr("href"),
				ID:           ptr("id"),
				Name:         ptr("name"),
				ResourceType: ptr("resource type"),
			},
		},
		NetworkInterfaces: []vpcv1.NetworkInterfaceInstanceContextReference{
			{
				ID: ptr("111"),
				PrimaryIP: &vpcv1.ReservedIPReference{
					Address:      ptr("192.0.1.1"),
					Href:         ptr("href"),
					ID:           ptr("id1"),
					Name:         ptr("name"),
					ResourceType: ptr("resource type"),
				},
			},
			{
				ID: ptr("222"),
				PrimaryIP: &vpcv1.ReservedIPReference{
					Address:      ptr("192.0.2.1"),
					Href:         ptr("href"),
					ID:           ptr("id2"),
					Name:         ptr("name"),
					ResourceType: ptr("resource type"),
				},
			},
		},
	}
	return instance, nil, nil
}

func (v *mockVPC) GetInstanceProfileWithContext(context context.Context, options *vpcv1.GetInstanceProfileOptions) (*vpcv1.InstanceProfile, *core.DetailedResponse, error) {
	profileType := options.Name

	if *profileType != "bx2-2x8" {
		return nil, nil, fmt.Errorf("Unsupported instance type")
	}

	vcpu := int64(2)
	mem := int64(8)
	return &vpcv1.InstanceProfile{VcpuCount: &vpcv1.InstanceProfileVcpu{Value: &vcpu}, Memory: &vpcv1.InstanceProfileMemory{Value: &mem}}, nil, nil
}

type mockCloudConfig struct{}

func (c *mockCloudConfig) Generate() (string, error) {
	return "cloud config", nil
}

func (v *mockVPC) DeleteInstanceWithContext(context.Context, *vpcv1.DeleteInstanceOptions) (*core.DetailedResponse, error) {

	res := &core.DetailedResponse{
		StatusCode: http.StatusOK,
	}

	return res, nil
}

func TestCreateInstance(t *testing.T) {

	vpc := &mockVPC{}

	provider := &ibmcloudVPCProvider{
		vpc: vpc,
		serviceConfig: &Config{
			ProfileName: "bx2-2x8",
		},
	}

	instance, err := provider.CreateInstance(context.Background(), "pod1", "999", &mockCloudConfig{}, cloud.InstanceTypeSpec{InstanceType: "bx2-2x8"})

	assert.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, "123", instance.ID)
	assert.Equal(t, "podvm-pod1-999", instance.Name)
	assert.Len(t, instance.IPs, 2)
	assert.Equal(t, "192.0.1.1", instance.IPs[0].String())
	assert.Equal(t, "192.0.2.1", instance.IPs[1].String())

	assert.NotNil(t, vpc.prototype)
	p, ok := vpc.prototype.(*vpcv1.InstancePrototype)
	assert.True(t, ok)
	assert.Equal(t, "cloud config", *p.UserData)
}

func TestDeleteInstance(t *testing.T) {

	provider := &ibmcloudVPCProvider{
		vpc:           &mockVPC{},
		serviceConfig: &Config{},
	}

	err := provider.DeleteInstance(context.Background(), "123")
	assert.NoError(t, err)
}

func TestGetInstanceTypeInformation(t *testing.T) {
	type args struct {
		instanceType string
	}
	tests := []struct {
		name       string
		provider   *ibmcloudVPCProvider
		args       args
		wantVcpu   int64
		wantMemory int64
		wantErr    bool
	}{
		// Test getting instance type information for a valid instance type
		{
			name: "getInstanceTypeInformationValidInstanceType",
			provider: &ibmcloudVPCProvider{
				vpc:           &mockVPC{},
				serviceConfig: &Config{},
			},
			args: args{
				instanceType: "bx2-2x8",
			},
			wantVcpu:   2,
			wantMemory: 8192,
			// Test should not return an error
			wantErr: false,
		},
		// Test getting instance type information for an invalid instance type
		{
			name: "getInstanceTypeInformationInvalidInstanceType",
			provider: &ibmcloudVPCProvider{
				vpc:           &mockVPC{},
				serviceConfig: &Config{},
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
			gotVcpu, gotMemory, err := tt.provider.getProfileNameInformation(tt.args.instanceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ibmcloudProvider.getProfileNameInformation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotVcpu != tt.wantVcpu {
				t.Errorf("ibmcloudProvider.getProfileNameInformation() gotVcpu = %v, want %v", gotVcpu, tt.wantVcpu)
			}
			if gotMemory != tt.wantMemory {
				t.Errorf("ibmcloudProvider.getProfileNameInformation() gotMemory = %v, want %v", gotMemory, tt.wantMemory)
			}
		})
	}
}
