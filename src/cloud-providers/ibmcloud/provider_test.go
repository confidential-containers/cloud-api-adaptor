// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockVPC struct {
	prototype vpcv1.InstancePrototypeIntf
	// createErr, when set, decides whether each CreateInstanceWithContext call fails
	createErr   func(call int, prototype *vpcv1.InstancePrototype) error
	createResp  *core.DetailedResponse
	createCalls int
	// getInstance, when set, replaces the default GetInstanceWithContext response
	getInstance func(call int) *vpcv1.Instance
	getCalls    int
	volumes     map[string]*vpcv1.Volume
	attachments []vpcv1.VolumeAttachmentReferenceInstanceContext
	readyAfter  int
	volumeCalls int
	detachAfter int
}

func readyNIC(id, address string) *vpcv1.NetworkInterfaceInstanceContextReference {
	return &vpcv1.NetworkInterfaceInstanceContextReference{
		ID:        ptr(id),
		PrimaryIP: &vpcv1.ReservedIPReference{Address: ptr(address)},
	}
}

func ptr(s string) *string {
	return &s
}

func (v *mockVPC) CreateInstanceWithContext(ctx context.Context, opt *vpcv1.CreateInstanceOptions) (*vpcv1.Instance, *core.DetailedResponse, error) {

	v.createCalls++
	v.prototype = opt.InstancePrototype
	if v.createErr != nil {
		if err := v.createErr(v.createCalls, opt.InstancePrototype.(*vpcv1.InstancePrototype)); err != nil {
			return nil, v.createResp, err
		}
	}

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

	v.getCalls++
	if v.getInstance != nil {
		return v.getInstance(v.getCalls), nil, nil
	}

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
	if v.getCalls >= v.readyAfter {
		instance.VolumeAttachments = v.attachments
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

func (v *mockVPC) GetVolumeWithContext(ctx context.Context, options *vpcv1.GetVolumeOptions) (*vpcv1.Volume, *core.DetailedResponse, error) {
	volume, ok := v.volumes[*options.ID]
	if !ok {
		return nil, &core.DetailedResponse{StatusCode: 404}, errors.New("volume not found")
	}
	v.volumeCalls++
	if v.detachAfter > 0 && v.volumeCalls >= v.detachAfter {
		detached := *volume
		detached.AttachmentState = ptr(vpcv1.VolumeAttachmentStateUnattachedConst)
		return &detached, nil, nil
	}
	return volume, nil, nil
}

func (v *mockVPC) GetImageWithContext(context context.Context, options *vpcv1.GetImageOptions) (*vpcv1.Image, *core.DetailedResponse, error) {

	imageID := options.ID
	if strings.HasPrefix(*imageID, "notfound") {
		return nil, nil, fmt.Errorf("image not found")
	}

	arch := "amd64"
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
	const zone = "us-south-1"
	const volumeID1 = "r006-0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	const volumeID2 = "0738-1b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"

	newVolume := func(state, zone string) *vpcv1.Volume {
		return &vpcv1.Volume{
			AttachmentState: ptr(state),
			Zone:            &vpcv1.ZoneReference{Name: ptr(zone)},
		}
	}
	newAttachment := func(volumeID, deviceID string) vpcv1.VolumeAttachmentReferenceInstanceContext {
		return vpcv1.VolumeAttachmentReferenceInstanceContext{
			ID:     ptr(deviceID[:len(deviceID)-6]),
			Device: &vpcv1.VolumeAttachmentDevice{ID: ptr(deviceID)},
			Volume: &vpcv1.VolumeReferenceVolumeAttachmentContext{ID: ptr(volumeID)},
		}
	}

	newProvider := func(t *testing.T, vpc *mockVPC, config *Config) *ibmcloudVPCProvider {
		t.Helper()
		images := make(Images, 0)
		require.NoError(t, images.Set("valid-image-id"))
		config.ProfileName = "bx2-2x8"
		config.Images = images
		config.ZoneName = zone
		config.VolumeAttachTimeout = 2 * time.Minute
		return &ibmcloudVPCProvider{
			vpc:           vpc,
			globalTagging: &mockTagging{},
			serviceConfig: config,
		}
	}
	createInstanceWithContext := func(ctx context.Context, t *testing.T, vpc *mockVPC, volumes []provider.CloudVolume) (*provider.Instance, error) {
		t.Helper()
		return newProvider(t, vpc, &Config{DisableCVM: true}).CreateInstance(ctx, "pod1", "999", &mockCloudConfig{}, provider.InstanceTypeSpec{
			InstanceType: "bx2-2x8",
			Volumes:      volumes,
		})
	}
	createInstance := func(t *testing.T, vpc *mockVPC, volumes []provider.CloudVolume) (*provider.Instance, error) {
		t.Helper()
		return createInstanceWithContext(t.Context(), t, vpc, volumes)
	}

	t.Run("without volumes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{}
			instance, err := createInstance(t, vpc, nil)
			require.NoError(t, err)
			require.NotNil(t, instance)
			assert.Equal(t, "123", instance.ID)
			assert.Equal(t, "podvm-pod1-999", instance.Name)
			require.Len(t, instance.IPs, 2)
			assert.Equal(t, "192.0.1.1", instance.IPs[0].String())
			assert.Equal(t, "192.0.2.1", instance.IPs[1].String())
			prototype, ok := vpc.prototype.(*vpcv1.InstancePrototype)
			require.True(t, ok)
			assert.Equal(t, "cloud config", *prototype.UserData)
			assert.Equal(t, false, *prototype.EnableSecureBoot)
			assert.Equal(t, "disabled", *prototype.ConfidentialComputeMode)
			assert.Empty(t, prototype.VolumeAttachments)
		})
	})

	t.Run("attaches volumes in order and keeps them on delete", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{
				volumes: map[string]*vpcv1.Volume{
					volumeID1: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, zone),
					volumeID2: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, zone),
				},
				attachments: []vpcv1.VolumeAttachmentReferenceInstanceContext{
					newAttachment("r006-boot", "0757-00000000-0000-4000-8000-000000000000-boot0"),
					newAttachment(volumeID1, "0757-9c534b6c-b446-4619-9339-0a3a54ef84be-dnjhs"),
					newAttachment(volumeID2, "0757-cee59332-6332-4d02-8bd3-86e3433dbc1b-b8vgr"),
				},
			}
			instance, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}, {DiskID: volumeID2}})
			require.NoError(t, err)
			assert.Equal(t, map[string]string{
				volumeID1: "/dev/disk/by-id/virtio-0757-9c534b6c-b446-4",
				volumeID2: "/dev/disk/by-id/virtio-0757-cee59332-6332-4",
			}, instance.VolumeDevices)
			prototype, ok := vpc.prototype.(*vpcv1.InstancePrototype)
			require.True(t, ok)
			assert.Equal(t, []vpcv1.VolumeAttachmentPrototype{
				{
					DeleteVolumeOnInstanceDelete: core.BoolPtr(false),
					Volume:                       &vpcv1.VolumeAttachmentPrototypeVolumeVolumeIdentity{ID: core.StringPtr(volumeID1)},
				},
				{
					DeleteVolumeOnInstanceDelete: core.BoolPtr(false),
					Volume:                       &vpcv1.VolumeAttachmentPrototypeVolumeVolumeIdentity{ID: core.StringPtr(volumeID2)},
				},
			}, prototype.VolumeAttachments)
		})
	})

	t.Run("keeps waiting for volumes after the address poll budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{
				readyAfter: 3 * maxRetries,
				volumes: map[string]*vpcv1.Volume{
					volumeID1: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, zone),
				},
				attachments: []vpcv1.VolumeAttachmentReferenceInstanceContext{
					newAttachment(volumeID1, "0757-9c534b6c-b446-4619-9339-0a3a54ef84be-dnjhs"),
				},
			}
			instance, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
			require.NoError(t, err)
			require.NotNil(t, instance)
			assert.Equal(t, map[string]string{
				volumeID1: "/dev/disk/by-id/virtio-0757-9c534b6c-b446-4",
			}, instance.VolumeDevices)
			require.Len(t, instance.IPs, 2)
			assert.Equal(t, "192.0.1.1", instance.IPs[0].String())
		})
	})

	t.Run("returns the instance for cleanup when volumes never attach", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{volumes: map[string]*vpcv1.Volume{
				volumeID1: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, zone),
			}}
			instance, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
			require.ErrorIs(t, err, errVolumeNotReady)
			require.ErrorContains(t, err, "volumes not attached after 2m0s")
			require.NotNil(t, instance)
			assert.Equal(t, "123", instance.ID)
		})
	})

	t.Run("fails when addresses never become ready", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{getInstance: func(int) *vpcv1.Instance {
				return &vpcv1.Instance{ID: ptr("123"), CRN: ptr("crn-123"), PrimaryNetworkInterface: readyNIC("111", "0.0.0.0")}
			}}
			instance, err := createInstance(t, vpc, nil)
			require.ErrorIs(t, err, errNotReady)
			require.ErrorContains(t, err, "network addresses not ready after 10 attempts")
			// the partial instance lets the caller delete the VM
			require.NotNil(t, instance)
			assert.Equal(t, "123", instance.ID)
			assert.Equal(t, maxRetries, vpc.getCalls)
		})
	})

	t.Run("waits for the secondary interface address", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{getInstance: func(call int) *vpcv1.Instance {
				instance := &vpcv1.Instance{ID: ptr("123"), CRN: ptr("crn-123"), PrimaryNetworkInterface: readyNIC("111", "192.0.1.1")}
				// the secondary interface only shows up on the second poll
				if call > 1 {
					instance.NetworkInterfaces = []vpcv1.NetworkInterfaceInstanceContextReference{
						*readyNIC("111", "192.0.1.1"),
						*readyNIC("222", "192.0.2.1"),
					}
				}
				return instance
			}}
			mockProvider := newProvider(t, vpc, &Config{SecondarySubnetID: "subnet-2", SecondarySecurityGroupID: "sg-2"})

			instance, err := mockProvider.CreateInstance(t.Context(), "pod1", "999", &mockCloudConfig{}, provider.InstanceTypeSpec{InstanceType: "bx2-2x8"})

			require.NoError(t, err)
			require.Len(t, instance.IPs, 2)
			assert.Equal(t, "192.0.1.1", instance.IPs[0].String())
			assert.Equal(t, "192.0.2.1", instance.IPs[1].String())
			assert.Equal(t, 2, vpc.getCalls)
		})
	})

	t.Run("stops polling when the context ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			vpc := &mockVPC{volumes: map[string]*vpcv1.Volume{
				volumeID1: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, zone),
			}}
			go func() {
				time.Sleep(3 * time.Second)
				cancel()
			}()
			instance, err := createInstanceWithContext(ctx, t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
			require.ErrorIs(t, err, context.Canceled)
			require.NotNil(t, instance)
			assert.Equal(t, "123", instance.ID)
		})
	})

	t.Run("rejects an empty volume ID", func(t *testing.T) {
		t.Parallel()
		vpc := &mockVPC{}
		_, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: ""}})
		require.ErrorContains(t, err, `volume "" has an empty ID`)
		assert.Equal(t, 0, vpc.createCalls)
	})

	t.Run("rejects a volume that does not exist", func(t *testing.T) {
		t.Parallel()
		vpc := &mockVPC{}
		_, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
		require.ErrorContains(t, err, fmt.Sprintf("failed to get volume %q", volumeID1))
		assert.Equal(t, 0, vpc.createCalls)
	})

	t.Run("waits for a volume to detach from the previous pod VM", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{
				detachAfter: 5,
				volumes: map[string]*vpcv1.Volume{
					volumeID1: newVolume(vpcv1.VolumeAttachmentStateAttachedConst, zone),
				},
				attachments: []vpcv1.VolumeAttachmentReferenceInstanceContext{
					newAttachment(volumeID1, "0757-9c534b6c-b446-4619-9339-0a3a54ef84be-dnjhs"),
				},
			}
			instance, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
			require.NoError(t, err)
			assert.Equal(t, 5, vpc.volumeCalls)
			assert.Equal(t, 1, vpc.createCalls)
			assert.Equal(t, map[string]string{
				volumeID1: "/dev/disk/by-id/virtio-0757-9c534b6c-b446-4",
			}, instance.VolumeDevices)
		})
	})

	t.Run("rejects a volume that stays attached", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			vpc := &mockVPC{volumes: map[string]*vpcv1.Volume{
				volumeID1: newVolume(vpcv1.VolumeAttachmentStateAttachedConst, zone),
			}}
			_, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
			require.ErrorContains(t, err, `has attachment state "attached", expected "unattached"`)
			assert.Equal(t, 0, vpc.createCalls)
		})
	})

	t.Run("rejects a volume in another zone", func(t *testing.T) {
		t.Parallel()
		vpc := &mockVPC{volumes: map[string]*vpcv1.Volume{
			volumeID1: newVolume(vpcv1.VolumeAttachmentStateUnattachedConst, "us-south-2"),
		}}
		_, err := createInstance(t, vpc, []provider.CloudVolume{{DiskID: volumeID1}})
		require.ErrorContains(t, err, `is in zone "us-south-2", expected "us-south-1"`)
		assert.Equal(t, 0, vpc.createCalls)
	})
}

func TestGetVolumeDevices(t *testing.T) {
	const volumeID = "r006-0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	volumes := []provider.CloudVolume{{DiskID: volumeID}}
	attachment := func(deviceID *string) *vpcv1.Instance {
		va := vpcv1.VolumeAttachmentReferenceInstanceContext{
			Volume: &vpcv1.VolumeReferenceVolumeAttachmentContext{ID: ptr(volumeID)},
		}
		if deviceID != nil {
			va.Device = &vpcv1.VolumeAttachmentDevice{ID: deviceID}
		}
		return &vpcv1.Instance{VolumeAttachments: []vpcv1.VolumeAttachmentReferenceInstanceContext{va}}
	}

	t.Run("nothing to map without volumes", func(t *testing.T) {
		devices, err := getVolumeDevices(&vpcv1.Instance{}, nil)
		require.NoError(t, err)
		assert.Nil(t, devices)
	})

	t.Run("not ready until the attachment reports a device", func(t *testing.T) {
		_, err := getVolumeDevices(attachment(nil), volumes)
		require.ErrorIs(t, err, errVolumeNotReady)
	})

	t.Run("not ready until the volume appears in the attachments", func(t *testing.T) {
		_, err := getVolumeDevices(&vpcv1.Instance{}, volumes)
		require.ErrorIs(t, err, errVolumeNotReady)
	})

	t.Run("rejects a device ID shorter than the virtio serial", func(t *testing.T) {
		_, err := getVolumeDevices(attachment(ptr("short")), volumes)
		require.ErrorContains(t, err, `unexpected device ID "short"`)
	})
}

func TestCreateInstanceWithFallback(t *testing.T) {

	errHost := errors.New("dedicated host is full")
	errGroup := errors.New("dedicated host group is full")
	rejected := &core.DetailedResponse{StatusCode: http.StatusBadRequest}
	fail := func(err error) func(int, *vpcv1.InstancePrototype) error {
		return func(int, *vpcv1.InstancePrototype) error { return err }
	}

	newProvider := func(vpc *mockVPC, hostID, groupID string) *ibmcloudVPCProvider {
		return &ibmcloudVPCProvider{
			vpc: vpc,
			serviceConfig: &Config{
				selectedDedicatedHostID:      hostID,
				selectedDedicatedHostGroupID: groupID,
			},
		}
	}
	prototype := func(p *ibmcloudVPCProvider) *vpcv1.InstancePrototype {
		return p.getInstancePrototype("podvm-pod1-999", "cloud config", "bx2-2x8", "image-1", nil)
	}
	groupTarget := func(proto *vpcv1.InstancePrototype) (string, bool) {
		target, ok := proto.PlacementTarget.(*vpcv1.InstancePlacementTargetPrototypeDedicatedHostGroupIdentityDedicatedHostGroupIdentityByID)
		if !ok {
			return "", false
		}
		return *target.ID, true
	}

	t.Run("does not retry when creation succeeds", func(t *testing.T) {
		vpc := &mockVPC{}
		p := newProvider(vpc, "host-1", "group-1")

		instance, err := p.createInstanceWithFallback(t.Context(), prototype(p))

		require.NoError(t, err)
		assert.Equal(t, "123", *instance.ID)
		assert.Equal(t, 1, vpc.createCalls)
	})

	t.Run("does not retry without a dedicated host group", func(t *testing.T) {
		vpc := &mockVPC{createErr: fail(errHost), createResp: rejected}
		p := newProvider(vpc, "host-1", "")

		instance, err := p.createInstanceWithFallback(t.Context(), prototype(p))

		require.ErrorIs(t, err, errHost)
		assert.Nil(t, instance)
		assert.Equal(t, 1, vpc.createCalls)
	})

	t.Run("retries on the dedicated host group when the host fails", func(t *testing.T) {
		var retryTarget string
		vpc := &mockVPC{createResp: rejected, createErr: func(call int, proto *vpcv1.InstancePrototype) error {
			if call == 1 {
				return errHost
			}
			retryTarget, _ = groupTarget(proto)
			return nil
		}}
		p := newProvider(vpc, "host-1", "group-1")

		instance, err := p.createInstanceWithFallback(t.Context(), prototype(p))

		require.NoError(t, err)
		assert.Equal(t, "123", *instance.ID)
		assert.Equal(t, 2, vpc.createCalls)
		assert.Equal(t, "group-1", retryTarget)
	})

	t.Run("reports both errors when the fallback also fails", func(t *testing.T) {
		vpc := &mockVPC{createResp: rejected, createErr: func(call int, _ *vpcv1.InstancePrototype) error {
			if call == 1 {
				return errHost
			}
			return errGroup
		}}
		p := newProvider(vpc, "host-1", "group-1")

		instance, err := p.createInstanceWithFallback(t.Context(), prototype(p))

		require.ErrorIs(t, err, errHost)
		require.ErrorIs(t, err, errGroup)
		assert.Nil(t, instance)
		assert.Equal(t, 2, vpc.createCalls)
	})

	t.Run("retries on any client error status", func(t *testing.T) {
		for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusTooManyRequests} {
			vpc := &mockVPC{createErr: fail(errHost), createResp: &core.DetailedResponse{StatusCode: status}}
			p := newProvider(vpc, "host-1", "group-1")

			_, err := p.createInstanceWithFallback(t.Context(), prototype(p))

			require.ErrorContains(t, err, `fallback instance creation on dedicated host group "group-1" failed`, status)
			assert.Equal(t, 2, vpc.createCalls, status)
			target, ok := groupTarget(vpc.prototype.(*vpcv1.InstancePrototype))
			require.True(t, ok, status)
			assert.Equal(t, "group-1", target, status)
		}
	})

	t.Run("does not retry when no response was received", func(t *testing.T) {
		vpc := &mockVPC{createErr: fail(errors.New("timeout"))}
		p := newProvider(vpc, "host-1", "group-1")

		_, err := p.createInstanceWithFallback(t.Context(), prototype(p))

		require.ErrorContains(t, err, "failed to create an instance: timeout")
		assert.Equal(t, 1, vpc.createCalls)
	})

	t.Run("does not retry an ambiguous server response", func(t *testing.T) {
		for _, status := range []int{http.StatusInternalServerError, http.StatusGatewayTimeout} {
			vpc := &mockVPC{createErr: fail(errors.New("server error")), createResp: &core.DetailedResponse{StatusCode: status}}
			p := newProvider(vpc, "host-1", "group-1")

			_, err := p.createInstanceWithFallback(t.Context(), prototype(p))

			require.Error(t, err)
			assert.Equal(t, 1, vpc.createCalls, status)
		}
	})

	t.Run("does not retry a cancelled request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		vpc := &mockVPC{createErr: fail(context.Canceled), createResp: rejected}
		p := newProvider(vpc, "host-1", "group-1")

		_, err := p.createInstanceWithFallback(ctx, prototype(p))

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, vpc.createCalls)
	})
}

func TestPickIDInZone(t *testing.T) {

	zones := map[string]string{
		"host-a": "eu-gb-1",
		"host-b": "eu-gb-2",
		"host-c": "eu-gb-2",
	}
	getZone := func(id string) (string, error) {
		zone, ok := zones[id]
		if !ok {
			return "", fmt.Errorf("unknown id %s", id)
		}
		return zone, nil
	}

	t.Run("returns the id in the zone", func(t *testing.T) {
		id, err := pickIDInZone([]string{"host-a", "host-b"}, "eu-gb-1", getZone, "Dedicated Host")

		require.NoError(t, err)
		assert.Equal(t, "host-a", id)
	})

	t.Run("returns the first of several ids in the zone", func(t *testing.T) {
		id, err := pickIDInZone([]string{"host-a", "host-b", "host-c"}, "eu-gb-2", getZone, "Dedicated Host")

		require.NoError(t, err)
		assert.Equal(t, "host-b", id)
	})

	t.Run("fails when no id is in the zone", func(t *testing.T) {
		id, err := pickIDInZone([]string{"host-a", "host-b"}, "eu-de-1", getZone, "Dedicated Host")

		require.ErrorContains(t, err, "no Dedicated Host in zone eu-de-1")
		assert.Equal(t, "", id)
	})

	t.Run("fails when a zone lookup fails", func(t *testing.T) {
		errLookup := errors.New("lookup failed")
		failing := func(string) (string, error) { return "", errLookup }

		id, err := pickIDInZone([]string{"host-a"}, "eu-gb-1", failing, "Dedicated Host")

		require.ErrorIs(t, err, errLookup)
		assert.Equal(t, "", id)
	})
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
				Arch: "amd64",
			},
			expectListErr:   false,
			expectSelectErr: false,
			wantID:          "valid-id-1",
			profileInstance: "bx2-2x8",
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
				Arch: "amd64",
			},
			expectListErr:   true,
			expectSelectErr: false,
			wantID:          "",
			profileInstance: "bx2-2x8",
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
				Arch: "amd64",
			},
			expectListErr:   true,
			expectSelectErr: false,
			wantID:          "",
			profileInstance: "bx2-2x8",
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
				Arch: "junk",
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
					InstanceProfileSpecList: []provider.InstanceTypeSpec{{InstanceType: "bx2-2x8", Arch: "amd64"}},
				},
				globalTagging: &mockTagging{},
			},
			instanceSpec:    provider.InstanceTypeSpec{},
			expectListErr:   false,
			expectSelectErr: false,
			wantID:          "valid-id-1",
			profileInstance: "bx2-2x8",
		},
		// Test selecting an image from an image list with no valid archs because of profile instance arch difference
		{
			name: "selectImageForInvalidInstanceArch",
			provider: &ibmcloudVPCProvider{
				vpc: &mockVPC{},
				serviceConfig: &Config{
					Images:                  validImageList,
					InstanceProfileSpecList: []provider.InstanceTypeSpec{{InstanceType: "bx2-2x8", Arch: "junk"}},
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
				t.Errorf("ibmcloudProvider.selectImage() gotID: %v, expected: %v, err: %v", id, tt.wantID, err)
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
