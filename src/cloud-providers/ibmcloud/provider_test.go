// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
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
		ID:  ptr("123"),
		CRN: ptr("crn-123"),
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
		ID:  ptr("123"),
		CRN: ptr("crn-123"),
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
	arch := "amd64"
	return &vpcv1.InstanceProfile{VcpuCount: &vpcv1.InstanceProfileVcpu{Value: &vcpu}, Memory: &vpcv1.InstanceProfileMemory{Value: &mem}, VcpuArchitecture: &vpcv1.InstanceProfileVcpuArchitecture{Value: &arch}}, nil, nil
}

func (v *mockVPC) GetImageWithContext(context context.Context, options *vpcv1.GetImageOptions) (*vpcv1.Image, *core.DetailedResponse, error) {

	imageID := options.ID
	if strings.HasPrefix(*imageID, "notfound") {
		return nil, nil, fmt.Errorf("image not found")
	}

	arch := "s390x"
	os := "ubuntu"

	return &vpcv1.Image{
		OperatingSystem: &vpcv1.OperatingSystem{
			Architecture: &arch,
			Name:         &os,
		},
	}, nil, nil
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

type mockTagging struct{}

func (t *mockTagging) AttachTagWithContext(ctx context.Context, attachTagOptions *globaltaggingv1.AttachTagOptions) (*globaltaggingv1.TagResults, *core.DetailedResponse, error) {
	tagRes := globaltaggingv1.TagResults{
		Results: []globaltaggingv1.TagResultsItem{{ResourceID: ptr("123")}},
	}

	res := &core.DetailedResponse{
		StatusCode: http.StatusOK,
	}

	return &tagRes, res, nil
}
func TestCreateInstance(t *testing.T) {

	vpc := &mockVPC{}
	globalTagging := &mockTagging{}

	images := make(Images, 0)
	err := images.Set("valid-image-id")
	if err != nil {
		t.Errorf("Images.Set() error %v", err)
	}
	mockProvider := &ibmcloudVPCProvider{
		vpc:           vpc,
		globalTagging: globalTagging,
		serviceConfig: &Config{
			ProfileName: "bx2-2x8",
			Images:      images,
			DisableCVM:  true,
		},
	}

	instance, err := mockProvider.CreateInstance(context.Background(), "pod1", "999", &mockCloudConfig{}, provider.InstanceTypeSpec{InstanceType: "bx2-2x8"})

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
	assert.Equal(t, false, *p.EnableSecureBoot)
	assert.Equal(t, "disabled", *p.ConfidentialComputeMode)
}

func TestDeleteInstance(t *testing.T) {

	provider := &ibmcloudVPCProvider{
		vpc:           &mockVPC{},
		serviceConfig: &Config{},
		globalTagging: &mockTagging{},
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
		wantArch   string
	}{
		// Test getting instance type information for a valid instance type
		{
			name: "getInstanceTypeInformationValidInstanceType",
			provider: &ibmcloudVPCProvider{
				vpc:           &mockVPC{},
				serviceConfig: &Config{},
				globalTagging: &mockTagging{},
			},
			args: args{
				instanceType: "bx2-2x8",
			},
			wantVcpu:   2,
			wantMemory: 8192,
			wantArch:   "amd64",
			// Test should not return an error
			wantErr: false,
		},
		// Test getting instance type information for an invalid instance type
		{
			name: "getInstanceTypeInformationInvalidInstanceType",
			provider: &ibmcloudVPCProvider{
				vpc:           &mockVPC{},
				serviceConfig: &Config{},
				globalTagging: &mockTagging{},
			},
			args: args{
				instanceType: "mycustominstance",
			},
			wantVcpu:   0,
			wantMemory: 0,
			wantArch:   "",
			// Test should return an error
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVcpu, gotMemory, gotArch, err := tt.provider.getProfileNameInformation(tt.args.instanceType)
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
			if gotArch != tt.wantArch {
				t.Errorf("ibmcloudProvider.getProfileNameInformation() gotArch = %v, want %v", gotArch, tt.wantArch)
			}
		})
	}
}

func TestGetImageDetails(t *testing.T) {

	validImageList := make(Images, 0)
	err := validImageList.Set("valid-id-1,valid-id-2,valid-id-3")
	if err != nil {
		t.Errorf("Images.Set() error %v", err)
	}
	emptyImageList := make(Images, 0)
	invalidImageList := make(Images, 0)
	err = invalidImageList.Set("notfound-id-1")
	if err != nil {
		t.Errorf("Images.Set() error %v", err)
	}

	tests := []struct {
		name            string
		provider        *ibmcloudVPCProvider
		instanceSpec    provider.InstanceTypeSpec
		expectListErr   bool
		expectSelectErr bool
		wantID          string
		profileInstance string
	}{
		// Test selecting an image from a valid image list
		{
			name: "selectImageForValidIDs",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: validImageList,
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec: provider.InstanceTypeSpec{
				Arch: "s390x",
			},
			expectListErr:   false,
			expectSelectErr: false,
			wantID:          "valid-id-1",
			profileInstance: "bz2-2x8",
		},
		// Test selecting an image from an empty image list
		{
			name: "selectImageForEmptyList",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: emptyImageList,
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec: provider.InstanceTypeSpec{
				Arch: "s390x",
			},
			expectListErr:   true,
			expectSelectErr: false,
			wantID:          "",
			profileInstance: "bz2-2x8",
		},
		// Test selecting an image from an image list with no valid ids
		{
			name: "selectImageForInvalidList",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: invalidImageList,
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec: provider.InstanceTypeSpec{
				Arch: "s390x",
			},
			expectListErr:   true,
			expectSelectErr: false,
			wantID:          "",
			profileInstance: "bz2-2x8",
		},
		// Test selecting an image from an image list with no valid archs
		{
			name: "selectImageForInvalidArch",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: validImageList,
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec: provider.InstanceTypeSpec{
				Arch: "amd64",
			},
			expectListErr:   false,
			expectSelectErr: true,
			wantID:          "",
			profileInstance: "bx2-2x8",
		},
		{
			name: "selectImageForValidInstanceArch",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images:                  validImageList,
					InstanceProfileSpecList: []provider.InstanceTypeSpec{{InstanceType: "bz2-2x8", Arch: "s390x"}},
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec:    provider.InstanceTypeSpec{},
			expectListErr:   false,
			expectSelectErr: false,
			wantID:          "valid-id-1",
			profileInstance: "bz2-2x8",
		},
		// Test selecting an image from an image list with no valid archs because of profile instance arch difference
		{
			name: "selectImageForInvalidInstanceArch",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images:                  validImageList,
					InstanceProfileSpecList: []provider.InstanceTypeSpec{{InstanceType: "bx2-2x8", Arch: "amd64"}},
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec:    provider.InstanceTypeSpec{},
			expectListErr:   false,
			expectSelectErr: true,
			wantID:          "",
			profileInstance: "bx2-2x8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.updateImageList(context.Background())
			if tt.expectListErr {
				if err == nil {
					t.Errorf("ibmcloudProvider.updateImageList() error = %v, expectListErr %v", err, tt.expectListErr)
				}
				return
			}
			id, err := tt.provider.selectImage(context.Background(), tt.instanceSpec, tt.profileInstance)
			if tt.expectSelectErr {
				if err == nil {
					t.Errorf("ibmcloudProvider.selectImage() error = %v, expectSelectErr %v", err, tt.expectSelectErr)
				}
				return
			}
			if id != tt.wantID {
				t.Errorf("ibmcloudProvider.selectImage() gotID = %v, want %v", id, tt.wantID)
			}
		})
	}
}

func TestConfigVerifier(t *testing.T) {

	validImageList := make(Images, 0)
	err := validImageList.Set("valid-id-1,valid-id-2,valid-id-3")
	if err != nil {
		t.Errorf("Images.Set() error %v", err)
	}
	emptyImageList := make(Images, 0)

	tests := []struct {
		name     string
		provider *ibmcloudVPCProvider
		wantErr  bool
	}{
		// Test selecting an image from a valid image list
		{
			name: "checkValidImageId",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: validImageList,
				},
				globalTagging: &mockTagging{},
			},
			wantErr: false,
		},
		// Test selecting an image from an empty image list
		{
			name: "checkInvalidImageId",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images: emptyImageList,
				},
				globalTagging: &mockTagging{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.ConfigVerifier()
			if tt.wantErr {
				if err == nil {
					t.Errorf("ibmcloudProvider.ConfigVerifier() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("ibmcloudProvider.ConfigVerifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
