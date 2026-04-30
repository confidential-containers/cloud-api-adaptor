// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants
const (
	// Volume and path constants
	testVolumePathCSI        = "/var/lib/kubelet/pods/abc/volumes/kubernetes.io~csi/pvc123/mount"
	testVolumePathKubelet    = "/var/lib/kubelet"
	testVolumePathInvalidCSI = "/var/lib/kubelet/kubernetes.io~csi/12345/mount"
	testMountDestination     = "/data"
	testMountType            = "bind"
	testMountPointData       = "/mnt/data"
	testMountPointImage      = "/mnt/image"
	testMountPointShared     = "/mnt/shared"
	testDeviceContainerPath  = "/dev/test"
	testDeviceVMPath         = "/dev/vda"
	testDeviceVMPathVdb      = "/dev/vdb"
	testDeviceType           = "b"

	// Container ID constants
	testContainerID123    = "test-123"
	testContainerID456    = "test-456"
	testContainerID789    = "test-789"
	testContainerIDDevice = "test-device"
	testContainerIDGPU    = "test-gpu"
	testContainerIDCSI    = "test-csi"
	testContainerIDError  = "test-error"
	testContainerIDCancel = "test-cancel"

	// Sandbox ID constants
	testSandboxID123 = "sandbox-123"
	testSandboxID456 = "sandbox-456"

	// Annotation constants
	testAnnotationKey1     = "key1"
	testAnnotationValue1   = "value1"
	testAnnotationGPUKey   = "io.katacontainers.config.hypervisor.default_gpus"
	testAnnotationGPUValue = "1"
	testAnnotationCDIKey   = "cdi.k8s.io/peer-pods"
	testAnnotationCDIValue = "nvidia.com/gpu=all"

	// Storage and filesystem constants
	testFstypeExt4              = "ext4"
	testDriverBlk               = "blk"
	testDriverImageGuestPull    = "image_guest_pull"
	testStorageSourceImageLayer = "image-layer"

	// Network and service constants
	testListenAddr = "127.0.0.1:0"
	testNetworkTCP = "tcp"
	testPauseImage = "pause:3.9"
	testHostname   = "test-host"
	testPolicyData = "test-policy-data"

	// File permission constant
	testDirPermission = 0700
)

func TestIsNodePublishVolumeTargetPath(t *testing.T) {
	volumePath := testVolumePathCSI
	directVolumesDir := t.TempDir()

	t.Run("Empty direct-volumes dir", func(t *testing.T) {
		assert.False(t, isNodePublishVolumeTargetPath(volumePath, directVolumesDir))
	})

	t.Run("Good path", func(t *testing.T) {
		err := prepareVolumeDir(directVolumesDir, volumePath)
		require.NoError(t, err, "Failed to add volume dir")

		assert.True(t, isNodePublishVolumeTargetPath(volumePath, directVolumesDir))
	})

	t.Run("Not CSI path", func(t *testing.T) {
		volumePath = testVolumePathKubelet

		err := prepareVolumeDir(directVolumesDir, volumePath)
		require.NoError(t, err, "Failed to add volume dir")

		assert.False(t, isNodePublishVolumeTargetPath(volumePath, directVolumesDir))
	})

	t.Run("Not much volumes/kubernetes.io~csi", func(t *testing.T) {
		volumePath = testVolumePathInvalidCSI

		err := prepareVolumeDir(directVolumesDir, volumePath)
		require.NoError(t, err, "Failed to add volume dir")

		assert.False(t, isNodePublishVolumeTargetPath(volumePath, directVolumesDir))
	})
}

func prepareVolumeDir(directVolumesDir, volumePath string) error {
	volumeDir := filepath.Join(directVolumesDir, b64.URLEncoding.EncodeToString([]byte(volumePath)))
	stat, err := os.Stat(volumeDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(volumeDir, testDirPermission); err != nil {
			return err
		}
	}
	if stat != nil && !stat.IsDir() {
		return fmt.Errorf("%s should be a directory", volumeDir)
	}

	return nil
}

// Test newProxyService
func TestNewProxyService(t *testing.T) {
	dialer := func(ctx context.Context) (net.Conn, error) {
		return nil, nil
	}

	service := newProxyService(dialer, testPauseImage)
	assert.NotNil(t, service, "expected non-nil service")
	assert.Equal(t, testPauseImage, service.pauseImage, "expected pause:3.9")
}

// createContainerRequestBuilder helps build CreateContainerRequest with fluent API
type createContainerRequestBuilder struct {
	req *pb.CreateContainerRequest
}

func newCreateContainerRequest(containerID string) *createContainerRequestBuilder {
	return &createContainerRequestBuilder{
		req: &pb.CreateContainerRequest{
			ContainerId: containerID,
			OCI: &pb.Spec{
				Annotations: make(map[string]string),
			},
		},
	}
}

func (b *createContainerRequestBuilder) withAnnotations(annotations map[string]string) *createContainerRequestBuilder {
	for k, v := range annotations {
		b.req.OCI.Annotations[k] = v
	}
	return b
}

func (b *createContainerRequestBuilder) withMounts(mounts ...*pb.Mount) *createContainerRequestBuilder {
	b.req.OCI.Mounts = append(b.req.OCI.Mounts, mounts...)
	return b
}

func (b *createContainerRequestBuilder) withStorages(storages ...*pb.Storage) *createContainerRequestBuilder {
	b.req.Storages = append(b.req.Storages, storages...)
	return b
}

func (b *createContainerRequestBuilder) withDevices(devices ...*pb.Device) *createContainerRequestBuilder {
	b.req.Devices = append(b.req.Devices, devices...)
	return b
}

func (b *createContainerRequestBuilder) build() *pb.CreateContainerRequest {
	return b.req
}

// Test CreateContainer with various scenarios
func TestProxyServiceCreateContainer(t *testing.T) {
	dir := t.TempDir()

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	tests := []struct {
		name           string
		setupFn        func(t *testing.T) (*proxyService, func())
		buildRequest   func() *pb.CreateContainerRequest
		getContext     func() context.Context
		expectError    bool
		validateResult func(t *testing.T, req *pb.CreateContainerRequest, err error)
	}{
		{
			name: "CreateContainer with mounts and annotations",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerID123).
					withAnnotations(map[string]string{testAnnotationKey1: testAnnotationValue1}).
					withMounts(&pb.Mount{
						Destination: testMountDestination,
						Source:      testVolumePathCSI,
						Type:        testMountType,
					}).
					build()
			},
			expectError: false,
		},
		{
			name: "CreateContainer with storages",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerID456).
					withStorages(&pb.Storage{
						MountPoint: testMountPointData,
						Source:     testDeviceVMPath,
						Fstype:     testFstypeExt4,
						Driver:     testDriverBlk,
					}).
					build()
			},
			expectError: false,
		},
		{
			name: "CreateContainer with image_guest_pull driver",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerID789).
					withStorages(&pb.Storage{
						MountPoint: testMountPointImage,
						Source:     testStorageSourceImageLayer,
						Driver:     testDriverImageGuestPull,
					}).
					build()
			},
			expectError: false,
		},
		{
			name: "CreateContainer with devices",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerIDDevice).
					withDevices(&pb.Device{
						ContainerPath: testDeviceContainerPath,
						VmPath:        testDeviceVMPath,
						Type:          testDeviceType,
					}).
					build()
			},
			expectError: false,
		},
		{
			name: "CreateContainer with GPU annotation",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerIDGPU).
					withAnnotations(map[string]string{testAnnotationGPUKey: testAnnotationGPUValue}).
					build()
			},
			expectError: false,
			validateResult: func(t *testing.T, req *pb.CreateContainerRequest, err error) {
				// Verify CDI annotation was added
				assert.Equal(t, testAnnotationCDIValue, req.OCI.Annotations[testAnnotationCDIKey], "expected CDI annotation to be set")
			},
		},
		{
			name: "CreateContainer with CSI volume mount",
			buildRequest: func() *pb.CreateContainerRequest {
				// Prepare volume directory
				volumePath := testVolumePathCSI
				err := prepareVolumeDir(dir, volumePath)
				require.NoError(t, err, "failed to prepare volume dir")

				return newCreateContainerRequest(testContainerIDCSI).
					withMounts(&pb.Mount{
						Destination: testMountDestination,
						Source:      volumePath,
						Type:        testMountType,
					}).
					build()
			},
			expectError: false,
		},
		{
			name: "CreateContainer with agent error",
			setupFn: func(t *testing.T) (*proxyService, func()) {
				// Setup a separate mock agent that returns errors
				errorAgentServer, err := ttrpc.NewServer()
				require.NoError(t, err, "failed to create ttrpc server")

				pb.RegisterAgentServiceService(errorAgentServer, &errorReturningMockAgent{})
				pb.RegisterHealthService(errorAgentServer, &errorReturningMockAgent{})

				errorAgentListener, err := net.Listen(testNetworkTCP, testListenAddr)
				require.NoError(t, err, "failed to create listener")

				go func() {
					_ = errorAgentServer.Serve(context.Background(), errorAgentListener)
				}()

				errorDialer := func(ctx context.Context) (net.Conn, error) {
					return net.Dial(testNetworkTCP, errorAgentListener.Addr().String())
				}

				errorService := newProxyService(errorDialer, "")
				err = errorService.Connect(context.Background())
				require.NoError(t, err, "failed to connect")

				cleanup := func() {
					errorService.Close()
					_ = errorAgentServer.Shutdown(context.Background())
					errorAgentListener.Close()
				}

				return errorService, cleanup
			},
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerIDError).build()
			},
			expectError: true,
		},
		{
			name: "CreateContainer with context cancellation",
			buildRequest: func() *pb.CreateContainerRequest {
				return newCreateContainerRequest(testContainerIDCancel).build()
			},
			getContext: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use custom service setup if provided, otherwise use the shared service
			testService := service
			var testCleanup func()
			if tt.setupFn != nil {
				testService, testCleanup = tt.setupFn(t)
				defer testCleanup()
			}

			// Build the request
			req := tt.buildRequest()

			// Get the context
			ctx := context.Background()
			if tt.getContext != nil {
				ctx = tt.getContext()
			}

			// Execute the test
			_, err := testService.CreateContainer(ctx, req)

			// Validate the result
			if tt.expectError {
				assert.Error(t, err, "expected error")
			} else {
				assert.NoError(t, err, "CreateContainer failed")
			}

			// Run custom validation if provided
			if tt.validateResult != nil {
				tt.validateResult(t, req, err)
			}
		})
	}
}

// Test simple proxy service methods with table-driven approach
func TestProxyServiceSimpleMethods(t *testing.T) {
	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "SetPolicy",
			fn: func() error {
				req := &pb.SetPolicyRequest{
					Policy: testPolicyData,
				}
				_, err := service.SetPolicy(context.Background(), req)
				return err
			},
		},
		{
			name: "StartContainer",
			fn: func() error {
				req := &pb.StartContainerRequest{
					ContainerId: testContainerID123,
				}
				_, err := service.StartContainer(context.Background(), req)
				return err
			},
		},
		{
			name: "RemoveContainer",
			fn: func() error {
				req := &pb.RemoveContainerRequest{
					ContainerId: testContainerID123,
				}
				_, err := service.RemoveContainer(context.Background(), req)
				return err
			},
		},
		{
			name: "CreateSandbox basic",
			fn: func() error {
				req := &pb.CreateSandboxRequest{
					Hostname:  testHostname,
					SandboxId: testSandboxID123,
				}
				_, err := service.CreateSandbox(context.Background(), req)
				return err
			},
		},
		{
			name: "CreateSandbox with storages",
			fn: func() error {
				req := &pb.CreateSandboxRequest{
					Hostname:  testHostname,
					SandboxId: testSandboxID456,
					Storages: []*pb.Storage{
						{
							MountPoint: testMountPointShared,
							Source:     testDeviceVMPathVdb,
							Fstype:     testFstypeExt4,
							Driver:     testDriverBlk,
						},
					},
				}
				_, err := service.CreateSandbox(context.Background(), req)
				return err
			},
		},
		{
			name: "DestroySandbox",
			fn: func() error {
				req := &pb.DestroySandboxRequest{}
				_, err := service.DestroySandbox(context.Background(), req)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.fn())
		})
	}
}

// setupMockAgentAndService is a helper that sets up a mock agent server and returns
// a connected proxy service along with cleanup functions
func setupMockAgentAndService(t *testing.T) (*proxyService, func()) {
	agentServer, agentListener := setupMockAgent(t)

	dialer := func(ctx context.Context) (net.Conn, error) {
		return net.Dial(testNetworkTCP, agentListener.Addr().String())
	}

	service := newProxyService(dialer, "")
	err := service.Connect(context.Background())
	require.NoError(t, err)

	cleanup := func() {
		service.Close()
		_ = agentServer.Shutdown(context.Background())
		agentListener.Close()
	}

	return service, cleanup
}
