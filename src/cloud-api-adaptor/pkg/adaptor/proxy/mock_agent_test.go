// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto/testutil"
)

type mockAgentService struct {
	*testutil.MockAgentServiceClient
	*testutil.MockHealthServiceClient
}

func newMockAgentService() *mockAgentService {
	return &mockAgentService{
		MockAgentServiceClient:  &testutil.MockAgentServiceClient{},
		MockHealthServiceClient: &testutil.MockHealthServiceClient{},
	}
}

// errorReturningMockAgent is a mock that returns errors for testing error handling
type errorReturningMockAgent struct {
	*mockAgentService
}

func (m *errorReturningMockAgent) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	return nil, errors.New("mock agent error")
}

// setupMockAgent is a shared helper function to create and start a mock agent server
// It returns the server and listener for use in tests
func setupMockAgent(t *testing.T) (*ttrpc.Server, net.Listener) {
	agentServer, err := ttrpc.NewServer()
	require.NoError(t, err, "failed to create ttrpc server")

	mockAgent := newMockAgentService()
	pb.RegisterAgentServiceService(agentServer, mockAgent)
	pb.RegisterHealthService(agentServer, mockAgent)

	agentListener, err := net.Listen("tcp", testListenAddressProxy)
	require.NoError(t, err, "failed to create listener")

	go func() {
		_ = agentServer.Serve(context.Background(), agentListener)
	}()

	return agentServer, agentListener
}
