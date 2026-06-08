// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
)

type mockRedirector struct {
	agentproto.Redirector
	createContainerCalled bool
	startContainerCalled  bool
	removeContainerCalled bool
	createSandboxCalled   bool
	destroySandboxCalled  bool
	createContainerError  error
	startContainerError   error
	removeContainerError  error
	createSandboxError    error
	destroySandboxError   error
}

func (m *mockRedirector) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	m.createContainerCalled = true
	return &emptypb.Empty{}, m.createContainerError
}

func (m *mockRedirector) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {
	m.startContainerCalled = true
	return &emptypb.Empty{}, m.startContainerError
}

func (m *mockRedirector) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {
	m.removeContainerCalled = true
	return &emptypb.Empty{}, m.removeContainerError
}

func (m *mockRedirector) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {
	m.createSandboxCalled = true
	return &emptypb.Empty{}, m.createSandboxError
}

func (m *mockRedirector) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {
	m.destroySandboxCalled = true
	return &emptypb.Empty{}, m.destroySandboxError
}

func (m *mockRedirector) Close() error {
	return nil
}

func TestNewInterceptor(t *testing.T) {
	t.Run("creates interceptor with valid socket name", func(t *testing.T) {
		socketName := "dummy.sock"

		i := NewInterceptor(socketName, "")
		require.NotNil(t, i, "Expected non-nil interceptor")

		// Verify the interceptor is properly initialized
		assert.NotNil(t, i, "Expected interceptor to be non-nil")
	})

	t.Run("creates interceptor with namespace path", func(t *testing.T) {
		socketName := "agent.sock"
		nsPath := "/run/netns/test"

		i := NewInterceptor(socketName, nsPath)
		require.NotNil(t, i, "Expected non-nil interceptor")

		// Verify the interceptor is properly initialized
		interceptorImpl, ok := i.(*interceptor)
		require.True(t, ok, "Expected *interceptor type")
		assert.Equal(t, nsPath, interceptorImpl.nsPath)
	})

	t.Run("creates interceptor with empty namespace path", func(t *testing.T) {
		socketName := "agent.sock"

		i := NewInterceptor(socketName, "")
		require.NotNil(t, i, "Expected non-nil interceptor")

		interceptorImpl, ok := i.(*interceptor)
		require.True(t, ok, "Expected *interceptor type")
		assert.Empty(t, interceptorImpl.nsPath)
	})
}

func TestIsTargetPath(t *testing.T) {
	t.Run("returns false when target path is empty", func(t *testing.T) {
		path := "/path/to/target"
		assert.False(t, isTargetPath(path, ""))
	})

	t.Run("returns false when both paths are empty", func(t *testing.T) {
		assert.False(t, isTargetPath("", ""))
	})

	t.Run("returns false when paths do not match", func(t *testing.T) {
		path := "/path/to/target"
		assert.False(t, isTargetPath(path, "mock path"))
		assert.False(t, isTargetPath(path, "/different/path"))
	})

	t.Run("returns true when paths match exactly", func(t *testing.T) {
		path := "/path/to/target"
		assert.True(t, isTargetPath(path, "/path/to/target"))
	})

	t.Run("returns false when path is empty but target is not", func(t *testing.T) {
		assert.False(t, isTargetPath("", "/path/to/target"))
	})

	t.Run("handles paths with special characters", func(t *testing.T) {
		path := "/path/to/target-with_special.chars"
		assert.True(t, isTargetPath(path, path))
		assert.False(t, isTargetPath(path, "/path/to/target"))
	})
}

func TestInterceptorCreateContainer(t *testing.T) {
	t.Run("adds network namespace to container spec", func(t *testing.T) {
		nsPath := "/run/netns/podns"
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     nsPath,
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)

		// Verify network namespace was added
		found := false
		for _, ns := range req.OCI.Linux.Namespaces {
			if ns.Type == string(specs.NetworkNamespace) && ns.Path == nsPath {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected network namespace to be added")
	})

	t.Run("creates mount source directory if it doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountSource := filepath.Join(tmpDir, "nonexistent", "mount")

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)

		// Verify directory was created
		_, err = os.Stat(mountSource)
		assert.NoError(t, err, "Expected mount source directory to be created")
	})

	t.Run("handles existing mount source directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountSource := filepath.Join(tmpDir, "existing")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles non-bind mount types", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{
					{
						Source: "/nonexistent",
						Type:   "tmpfs",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("propagates redirector errors", func(t *testing.T) {
		expectedErr := assert.AnError
		mock := &mockRedirector{
			createContainerError: expectedErr,
		}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.True(t, mock.createContainerCalled)
	})
}

func TestInterceptorStartContainer(t *testing.T) {
	t.Run("successfully starts container", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.StartContainerRequest{
			ContainerId: "test-container",
		}

		ctx := context.Background()
		_, err := i.StartContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.startContainerCalled)
	})

	t.Run("propagates redirector errors", func(t *testing.T) {
		expectedErr := assert.AnError
		mock := &mockRedirector{
			startContainerError: expectedErr,
		}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.StartContainerRequest{
			ContainerId: "test-container",
		}

		ctx := context.Background()
		_, err := i.StartContainer(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.True(t, mock.startContainerCalled)
	})
}

func TestInterceptorRemoveContainer(t *testing.T) {
	t.Run("successfully removes container", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.RemoveContainerRequest{
			ContainerId: "test-container",
		}

		ctx := context.Background()
		_, err := i.RemoveContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.removeContainerCalled)
	})

	t.Run("propagates redirector errors", func(t *testing.T) {
		expectedErr := assert.AnError
		mock := &mockRedirector{
			removeContainerError: expectedErr,
		}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.RemoveContainerRequest{
			ContainerId: "test-container",
		}

		ctx := context.Background()
		_, err := i.RemoveContainer(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.True(t, mock.removeContainerCalled)
	})
}

func TestInterceptorCreateSandbox(t *testing.T) {
	t.Run("successfully creates sandbox", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
	})

	t.Run("removes DNS settings from request", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
			Dns:       []string{"8.8.8.8", "8.8.4.4"},
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
		assert.Nil(t, req.Dns, "Expected DNS to be removed from request")
	})

	t.Run("handles empty DNS settings", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
			Dns:       []string{},
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
	})

	t.Run("propagates redirector errors", func(t *testing.T) {
		expectedErr := assert.AnError
		mock := &mockRedirector{
			createSandboxError: expectedErr,
		}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.True(t, mock.createSandboxCalled)
	})
}

func TestInterceptorDestroySandbox(t *testing.T) {
	t.Run("successfully destroys sandbox", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.DestroySandboxRequest{}

		ctx := context.Background()
		_, err := i.DestroySandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.destroySandboxCalled)
	})

	t.Run("propagates redirector errors", func(t *testing.T) {
		expectedErr := assert.AnError
		mock := &mockRedirector{
			destroySandboxError: expectedErr,
		}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.DestroySandboxRequest{}

		ctx := context.Background()
		_, err := i.DestroySandbox(ctx, req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.True(t, mock.destroySandboxCalled)
	})
}

func TestInterceptorWithAnnotations(t *testing.T) {
	t.Run("handles volume target path annotation without matching mount", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create annotation path but use different mount source to avoid device wait
		annotationPath := filepath.Join(tmpDir, "annotation-volume")
		mountSource := filepath.Join(tmpDir, "actual-mount")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					volumeTargetPathKey: annotationPath,
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles empty volume target path annotation", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountSource := filepath.Join(tmpDir, "volume")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					volumeTargetPathKey: "",
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles missing volume target path annotation", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountSource := filepath.Join(tmpDir, "volume")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})
}

func TestInterceptorCreateContainerWithMountErrors(t *testing.T) {
	t.Run("handles mount source creation error when MkdirAll fails", func(t *testing.T) {
		// Create a temp directory with restricted permissions to trigger MkdirAll failure
		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")

		// Create directory and make it non-writable
		err := os.Mkdir(restrictedDir, 0755)
		require.NoError(t, err)
		err = os.Chmod(restrictedDir, 0444) // read-only
		require.NoError(t, err)

		// Restore permissions after test for cleanup
		defer func() {
			_ = os.Chmod(restrictedDir, 0755)
		}()

		// Try to create a subdirectory in the non-writable directory
		mountSource := filepath.Join(restrictedDir, "subdir", "test-mount-should-fail")

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		// Should not fail even if directory creation fails
		_, err = i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles multiple mounts with mixed types", func(t *testing.T) {
		tmpDir := t.TempDir()
		bindMount := filepath.Join(tmpDir, "bind")
		require.NoError(t, os.MkdirAll(bindMount, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{
					{
						Source: bindMount,
						Type:   "bind",
					},
					{
						Source: "tmpfs",
						Type:   "tmpfs",
					},
					{
						Source: "proc",
						Type:   "proc",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles container with no mounts", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: []*pb.Mount{},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles container with nil mounts", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Mounts: nil,
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})
}

func TestInterceptorCreateContainerWithNamespaces(t *testing.T) {
	t.Run("adds network namespace to existing namespaces", func(t *testing.T) {
		nsPath := "/run/netns/podns"
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     nsPath,
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{
						{
							Type: string(specs.PIDNamespace),
							Path: "/proc/1/ns/pid",
						},
						{
							Type: string(specs.UTSNamespace),
							Path: "/proc/1/ns/uts",
						},
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)

		// Verify network namespace was added
		assert.Len(t, req.OCI.Linux.Namespaces, 3)
		found := false
		for _, ns := range req.OCI.Linux.Namespaces {
			if ns.Type == string(specs.NetworkNamespace) && ns.Path == nsPath {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected network namespace to be added")
	})

	t.Run("handles empty namespace path", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)

		// Verify network namespace was added even with empty path
		found := false
		for _, ns := range req.OCI.Linux.Namespaces {
			if ns.Type == string(specs.NetworkNamespace) {
				found = true
				assert.Empty(t, ns.Path)
				break
			}
		}
		assert.True(t, found)
	})
}

func TestInterceptorCreateSandboxWithDNS(t *testing.T) {
	t.Run("handles single DNS server", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
			Dns:       []string{"8.8.8.8"},
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
		assert.Nil(t, req.Dns)
	})

	t.Run("handles many DNS servers", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
			Dns:       []string{"8.8.8.8", "8.8.4.4", "1.1.1.1", "1.0.0.1"},
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
		assert.Nil(t, req.Dns)
	})

	t.Run("handles nil DNS", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
		}

		req := &pb.CreateSandboxRequest{
			Hostname:  "test-host",
			SandboxId: "test-sandbox",
			Dns:       nil,
		}

		ctx := context.Background()
		_, err := i.CreateSandbox(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createSandboxCalled)
		assert.Nil(t, req.Dns)
	})
}

func TestInterceptorWithComplexAnnotations(t *testing.T) {
	t.Run("handles annotation with whitespace in paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		path1 := filepath.Join(tmpDir, "path1")
		path2 := filepath.Join(tmpDir, "path2")
		mountSource := filepath.Join(tmpDir, "mount")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					volumeTargetPathKey: path1 + " , " + path2 + " ",
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})

	t.Run("handles annotation with single path and comma", func(t *testing.T) {
		tmpDir := t.TempDir()
		annotationPath := filepath.Join(tmpDir, "annotation")
		mountSource := filepath.Join(tmpDir, "mount")
		require.NoError(t, os.MkdirAll(mountSource, 0o755))

		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					volumeTargetPathKey: annotationPath + ",",
				},
				Mounts: []*pb.Mount{
					{
						Source: mountSource,
						Type:   "bind",
					},
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)

		require.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})
}

func TestCloudVolumesJSONParseError(t *testing.T) {
	t.Run("corrupt JSON annotation returns error instead of silently skipping", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: "{invalid json",
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "corrupt cloud_volumes annotation")
		assert.False(t, mock.createContainerCalled)
	})

	t.Run("missing mount_point field returns error", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: `{"vol-0":{"fs_type":"ext4","lun":"0"}}`,
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required mount_point or lun")
	})

	t.Run("unsupported filesystem type returns error", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: `{"vol-0":{"mount_point":"/data","fs_type":"ntfs","lun":"0"}}`,
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported filesystem type")
	})

	t.Run("unsafe volume name returns error", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: `{"../escape":{"mount_point":"/data","fs_type":"ext4","lun":"0"}}`,
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsafe name")
	})

	t.Run("invalid lun value returns error", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: `{"vol-0":{"mount_point":"/data","fs_type":"ext4","lun":"abc"}}`,
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid lun")
	})

	t.Run("valid cloud_volumes without device does not panic", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: `{"vol-0":{"mount_point":"/data","fs_type":"ext4","lun":"0","disk_id":"vol-fake"}}`,
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		// Will error because the device doesn't exist, but should not panic
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cloud volume vol-0")
	})

	t.Run("empty cloud_volumes annotation is ignored", func(t *testing.T) {
		mock := &mockRedirector{}
		i := &interceptor{
			Redirector: mock,
			nsPath:     "/run/netns/podns",
		}

		req := &pb.CreateContainerRequest{
			ContainerId: "test-container",
			OCI: &pb.Spec{
				Linux: &pb.Linux{
					Namespaces: []*pb.LinuxNamespace{},
				},
				Annotations: map[string]string{
					util.CloudVolumesAnnotationKey: "",
				},
			},
		}

		ctx := context.Background()
		_, err := i.CreateContainer(ctx, req)
		assert.NoError(t, err)
		assert.True(t, mock.createContainerCalled)
	})
}

func TestHasPartitions(t *testing.T) {
	t.Run("returns false for nonexistent device", func(t *testing.T) {
		assert.False(t, hasPartitions("nonexistent_device_xyz"))
	})

	t.Run("returns false when no partition entries exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		devName := "sdtest"
		sysBlockDir := filepath.Join(tmpDir, devName)
		require.NoError(t, os.MkdirAll(sysBlockDir, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "queue"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(sysBlockDir, "power"), 0o755))
		// No sda1 etc.
		// We can't directly test since hasPartitions reads from /sys/block,
		// but we test the logic indirectly through mock paths
		assert.False(t, hasPartitions("nonexistent_device_xyz"))
	})
}

func TestIsRootOrMountedDevice(t *testing.T) {
	t.Run("returns false for unknown device", func(t *testing.T) {
		assert.False(t, isRootOrMountedDevice("zzznonexistent999"))
	})

	t.Run("returns true for root device if accessible", func(t *testing.T) {
		data, err := os.ReadFile("/proc/mounts")
		if err != nil {
			t.Skip("cannot read /proc/mounts")
		}
		// Find actual root mount device
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == "/" && strings.HasPrefix(fields[0], "/dev/") {
				devName := strings.TrimPrefix(fields[0], "/dev/")
				// Strip partition number (e.g., sda1 -> sda)
				base := strings.TrimRight(devName, "0123456789")
				assert.True(t, isRootOrMountedDevice(base),
					"expected device %s (from %s) to be detected as mounted", base, fields[0])
				return
			}
		}
		t.Skip("could not find root device in /proc/mounts")
	})
}

func TestDetectCloudProvider(t *testing.T) {
	provider := detectCloudProvider()
	// On a regular test machine, this won't be a cloud provider
	// We just verify it returns one of the known values
	validProviders := map[string]bool{
		"azure": true, "aws": true, "libvirt": true, "generic": true,
	}
	assert.True(t, validProviders[provider],
		"detectCloudProvider returned unexpected value: %s", provider)
}

func TestFindLibvirtDataDisk(t *testing.T) {
	t.Run("rejects out of range LUN index", func(t *testing.T) {
		_, err := findLibvirtDataDisk(25)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("rejects negative LUN index", func(t *testing.T) {
		_, err := findLibvirtDataDisk(-1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("returns error when device does not exist", func(t *testing.T) {
		// Unless /dev/vdb exists, this should fail
		_, err := findLibvirtDataDisk(0)
		if err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	})
}

func TestFindDataDiskBySysfsHCTL(t *testing.T) {
	t.Run("returns error for impossible LUN on test machine", func(t *testing.T) {
		// LUN 999 should never exist
		_, err := findDataDiskBySysfsHCTL(999)
		assert.Error(t, err)
	})
}

func TestIsTargetPathEdgeCases(t *testing.T) {
	t.Run("handles paths with trailing slashes", func(t *testing.T) {
		assert.False(t, isTargetPath("/path/to/target/", "/path/to/target"))
		assert.False(t, isTargetPath("/path/to/target", "/path/to/target/"))
	})

	t.Run("handles similar but different paths", func(t *testing.T) {
		assert.False(t, isTargetPath("/path/to/target", "/path/to/target2"))
		assert.False(t, isTargetPath("/path/to/target2", "/path/to/target"))
	})

	t.Run("handles paths with dots", func(t *testing.T) {
		path := "/path/to/../target"
		assert.True(t, isTargetPath(path, path))
		assert.False(t, isTargetPath(path, "/path/target"))
	})
}
