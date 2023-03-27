// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"net/http"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
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
		vpc:           vpc,
		serviceConfig: &Config{},
	}

	instance, err := provider.CreateInstance(context.Background(), "pod1", "999", &mockCloudConfig{})

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
