// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"net"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants for proxy_test.go
const (
	// Socket and path constants
	testSocketPathDummy   = "/run/dummy.sock"
	testSocketPathTest    = "/run/test.sock"
	testSocketPathInvalid = "/proc/invalid/test.sock"
	testSocketFileName    = "test.sock"

	// Server and network constants
	testServerName         = "test-server"
	testServerNamePodVM    = "podvm"
	testListenAddressProxy = "127.0.0.1:0"
	testInvalidAddress     = "0.0.0.0:0"
	testUnreachableAddress = "127.0.0.1:1"
	testUnreachablePort    = "127.0.0.1:9999"
	testSchemeGRPC         = "grpc"
	testNetworkUnix        = "unix"

	// Container and annotation constants
	testContainerIDProxy     = "123"
	testAnnotationKeyProxy   = "aaa"
	testAnnotationValueProxy = "111"

	// Image constants
	testPauseImageLatest = "pause:latest"

	// Timeout constants
	testTimeout1Second      = 1 * time.Second
	testTimeout2Second      = 2 * time.Second
	testTimeout5SecondProxy = 5 * time.Second
	testTimeout250          = 250 * time.Millisecond

	// TLS and certificate constants
	testCAFilePath   = "/path/to/ca.pem"
	testCertData     = "test-cert-data"
	testMockRootCert = "mock-root-cert"
	testMockCert     = "mock-cert"
	testMockKey      = "mock-key"
)

func TestNewAgentProxy(t *testing.T) {

	socketPath := testSocketPathDummy

	proxy := NewAgentProxy(testServerNamePodVM, socketPath, "", nil, nil, 0)
	p, ok := proxy.(*agentProxy)
	if !ok {
		t.Fatalf("expect %T, got %T", &agentProxy{}, proxy)
	}
	if e, a := socketPath, p.socketPath; e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}
}

func TestStartStop(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, testSocketFileName)

	// Setup mock agent server using shared helper
	agentServer, agentListener := setupMockAgent(t)
	defer func() { _ = agentServer.Shutdown(context.Background()) }()
	defer agentListener.Close()

	serverURL := &url.URL{
		Scheme: testSchemeGRPC,
		Host:   agentListener.Addr().String(),
	}

	proxy := NewAgentProxy(testServerNamePodVM, socketPath, "", nil, nil, testTimeout5SecondProxy)
	p, ok := proxy.(*agentProxy)
	if !ok {
		t.Fatalf("expect %T, got %T", &agentProxy{}, proxy)
	}

	proxyErrCh := make(chan error)
	go func() {
		defer close(proxyErrCh)
		if err := proxy.Start(context.Background(), serverURL); err != nil {
			proxyErrCh <- err
		}
	}()
	defer func() {
		require.NoError(t, p.Shutdown(), "expect no error during shutdown")
	}()

	select {
	case err := <-proxyErrCh:
		require.NoError(t, err, "expect no error from proxy")
	case <-proxy.Ready():
	}

	conn, err := net.Dial(testNetworkUnix, socketPath)
	require.NoError(t, err, "expect no error dialing unix socket")

	ttrpcClient := ttrpc.NewClient(conn)

	client := struct {
		pb.AgentServiceService
		pb.HealthService
	}{
		AgentServiceService: pb.NewAgentServiceClient(ttrpcClient),
		HealthService:       pb.NewHealthClient(ttrpcClient),
	}

	{
		res, err := client.CreateContainer(context.Background(), &pb.CreateContainerRequest{ContainerId: testContainerIDProxy, OCI: &pb.Spec{Annotations: map[string]string{testAnnotationKeyProxy: testAnnotationValueProxy}}})
		assert.NoError(t, err, "expect no error creating container")
		assert.NotNil(t, res, "expect non nil response")
	}

	select {
	case err := <-proxyErrCh:
		assert.NoError(t, err, "expect no error from proxy channel")
	default:
	}
}

func TestDialerSuccess(t *testing.T) {
	p := &agentProxy{
		proxyTimeout: testTimeout5SecondProxy,
	}

	for {
		listener, err := net.Listen(testNetworkTCP, testListenAddressProxy)
		require.NoError(t, err, "expect no error creating listener")

		address := listener.Addr().String()

		require.NoError(t, listener.Close(), "expect no error closing listener")

		listenerErrCh := make(chan error)
		go func() {
			defer close(listenerErrCh)

			time.Sleep(testTimeout250)

			var err error
			// Open the same port
			listener, err = net.Listen(testNetworkTCP, address)
			if err != nil {
				listenerErrCh <- err
			}
		}()

		conn, err := p.dial(context.Background(), address)
		if err == nil {
			listener.Close()
			break
		}
		defer conn.Close()

		if e := <-listenerErrCh; e != nil {
			// A rare case occurs. Retry the test.
			t.Logf("%v", e)
			continue
		}

		listener.Close()
		assert.NoError(t, err, "expect no error dialing")
		break
	}
}

func TestDialerFailure(t *testing.T) {
	p := &agentProxy{
		proxyTimeout: testTimeout5SecondProxy,
	}

	address := testInvalidAddress
	conn, err := p.dial(context.Background(), address)
	assert.ErrorContains(t, err, "failed to establish agent proxy connection")
	if err == nil {
		conn.Close()
	}
}

// Test CAService method
func TestCAService(t *testing.T) {
	t.Run("CAService returns nil when not set", func(t *testing.T) {
		proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", nil, nil, 0)
		p := proxy.(*agentProxy)
		assert.Nil(t, p.CAService())
	})

	t.Run("CAService returns service when set", func(t *testing.T) {
		mockCAService := &mockCAService{}
		proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", nil, mockCAService, 0)
		p := proxy.(*agentProxy)
		assert.Equal(t, mockCAService, p.CAService())
	})
}

// Test ClientCA method
func TestClientCA(t *testing.T) {
	t.Run("ClientCA returns nil when tlsConfig is nil", func(t *testing.T) {
		proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", nil, nil, 0)
		p := proxy.(*agentProxy)
		assert.Nil(t, p.ClientCA())
	})

	t.Run("ClientCA returns nil when CAFile is set", func(t *testing.T) {
		tlsConfig := &tlsutil.TLSConfig{
			CAFile: testCAFilePath,
		}
		proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", tlsConfig, nil, 0)
		p := proxy.(*agentProxy)
		assert.Nil(t, p.ClientCA())
	})

	t.Run("ClientCA returns CertData when CAFile is empty", func(t *testing.T) {
		certData := []byte(testCertData)
		tlsConfig := &tlsutil.TLSConfig{
			CertData: certData,
		}
		proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", tlsConfig, nil, 0)
		p := proxy.(*agentProxy)
		result := p.ClientCA()
		assert.Equal(t, string(certData), string(result))
	})
}

// Test Ready channel
func TestReady(t *testing.T) {
	proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", nil, nil, 0)
	readyCh := proxy.Ready()
	assert.NotNil(t, readyCh)
}

// Test multiple Shutdown calls
func TestMultipleShutdown(t *testing.T) {
	proxy := NewAgentProxy(testServerNamePodVM, testSocketPathTest, "", nil, nil, 0)
	p := proxy.(*agentProxy)

	// First shutdown
	assert.NoError(t, p.Shutdown())

	// Second shutdown should also succeed (stopOnce ensures it's safe)
	assert.NoError(t, p.Shutdown())
}

// Test Start with invalid socket path
func TestStartInvalidSocketPath(t *testing.T) {
	// Use a path that cannot be created
	socketPath := testSocketPathInvalid
	proxy := NewAgentProxy(testServerNamePodVM, socketPath, "", nil, nil, testTimeout5SecondProxy)

	serverURL := &url.URL{
		Scheme: testSchemeGRPC,
		Host:   testUnreachablePort,
	}

	err := proxy.Start(context.Background(), serverURL)
	assert.ErrorContains(t, err, "failed to create parent directories")
}

// Test Start with connection failure
func TestStartConnectionFailure(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, testSocketFileName)

	proxy := NewAgentProxy(testServerNamePodVM, socketPath, "", nil, nil, testTimeout1Second)

	// Use an address that will fail to connect
	serverURL := &url.URL{
		Scheme: testSchemeGRPC,
		Host:   testUnreachableAddress, // Port 1 is typically not accessible
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout2Second)
	defer cancel()

	err := proxy.Start(ctx, serverURL)
	assert.Error(t, err)
}

// Test NewFactory
func TestNewFactory(t *testing.T) {
	t.Run("NewFactory with nil TLS config", func(t *testing.T) {
		proxyFactory := NewFactory(testPauseImageLatest, nil, testTimeout5SecondProxy)
		assert.NotNil(t, proxyFactory)

		// Just verify it's not nil and can create proxies
		proxy := proxyFactory.New(testServerName, testSocketPathTest)
		assert.NotNil(t, proxy)
	})

	t.Run("Factory.New creates AgentProxy", func(t *testing.T) {
		proxyFactory := NewFactory(testPauseImageLatest, nil, testTimeout5SecondProxy)
		proxy := proxyFactory.New(testServerName, testSocketPathTest)

		assert.NotNil(t, proxy)

		p, ok := proxy.(*agentProxy)
		assert.True(t, ok, "expected *agentProxy, got %T", proxy)

		assert.Equal(t, testServerName, p.serverName)
		assert.Equal(t, testSocketPathTest, p.socketPath)
		assert.Equal(t, testPauseImageLatest, p.pauseImage)
		assert.Equal(t, testTimeout5SecondProxy, p.proxyTimeout)
	})
}

// Mock types for testing
type mockCAService struct{}

func (m *mockCAService) RootCertificate() []byte {
	return []byte(testMockRootCert)
}

func (m *mockCAService) Issue(name string) (certPEM, keyPEM []byte, err error) {
	return []byte(testMockCert), []byte(testMockKey), nil
}
