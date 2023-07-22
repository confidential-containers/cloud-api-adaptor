// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"testing"

	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestNewInterceptor(t *testing.T) {

	socketName := "dummy.sock"

	i := NewInterceptor(socketName, "")
	if i == nil {
		t.Fatal("Expect non nil, got nil")
	}
}

func TestIsTargetPath(t *testing.T) {
	path := "/path/to/target"

	assert.False(t, isTargetPath(path, ""))
	assert.False(t, isTargetPath("", ""))
	assert.False(t, isTargetPath(path, "mock path"))
	assert.True(t, isTargetPath(path, "/path/to/target"))
}

func TestProcessOCISpec(t *testing.T) {
	i := &interceptor{
		nsPath: "/var/run/netns/test",
	}

	spec := &pb.Spec{
		Linux: &pb.Linux{
			Namespaces: []pb.LinuxNamespace{
				{
					Type: string(specs.NetworkNamespace),
					Path: "/var/run/netns/default",
				},
			},
		},
	}

	i.processOCISpec(spec)

	// Check that the network namespace path was set correctly
	assert.Equal(t, i.nsPath, spec.Linux.Namespaces[1].Path)

	// Check that the attester socket was added as a mount
	assert.Equal(t, "/run/confidential-containers/attester.sock", spec.Mounts[0].Destination)
	assert.Equal(t, "bind", spec.Mounts[0].Type)
	assert.Equal(t, []string{"bind"}, spec.Mounts[0].Options)
}
