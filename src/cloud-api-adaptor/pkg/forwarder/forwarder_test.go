// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto/testutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
)

func dummyDialer(ctx context.Context) (net.Conn, error) {
	return testutil.NewMockConn(), nil
}

type mockPodNode struct {
	setupCalled    bool
	teardownCalled bool
	setupError     error
	teardownError  error
}

func (n *mockPodNode) Setup() error {
	n.setupCalled = true
	return n.setupError
}

func (n *mockPodNode) Teardown() error {
	n.teardownCalled = true
	return n.teardownError
}

func TestNewDaemon(t *testing.T) {
	t.Run("creates daemon with minimal config", func(t *testing.T) {
		config := &Config{}
		tlsConfig := &tlsutil.TLSConfig{}

		ret := NewDaemon(config, DefaultListenAddr, tlsConfig, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret, "Expected non-nil daemon")

		d, ok := ret.(*daemon)
		require.True(t, ok, "Expected *daemon type, got %T", ret)
		assert.NotNil(t, d.interceptor, "Expected non-nil interceptor")
		assert.NotNil(t, d.stopCh, "Expected non-nil stopCh")
		assert.NotNil(t, d.readyCh, "Expected non-nil readyCh")

		// Verify stopCh is open
		select {
		case <-d.stopCh:
			t.Fatal("Expected stopCh to be open")
		default:
		}
	})

	t.Run("creates daemon with TLS config", func(t *testing.T) {
		config := &Config{
			TLSServerCert: "cert-data",
			TLSServerKey:  "key-data",
			TLSClientCA:   "ca-data",
		}
		tlsConfig := &tlsutil.TLSConfig{}

		ret := NewDaemon(config, DefaultListenAddr, tlsConfig, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.NotNil(t, d.tlsConfig)
		assert.Equal(t, []byte("cert-data"), d.tlsConfig.CertData)
		assert.Equal(t, []byte("key-data"), d.tlsConfig.KeyData)
		assert.Equal(t, []byte("ca-data"), d.tlsConfig.CAData)
	})

	t.Run("creates daemon with custom listen address", func(t *testing.T) {
		config := &Config{}
		customAddr := "127.0.0.1:9999"

		ret := NewDaemon(config, customAddr, nil, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.Equal(t, customAddr, d.listenAddr)
	})

	t.Run("creates daemon with nil TLS config", func(t *testing.T) {
		config := &Config{}

		ret := NewDaemon(config, DefaultListenAddr, nil, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.Nil(t, d.tlsConfig)
	})

	t.Run("creates daemon with pod network config", func(t *testing.T) {
		config := &Config{
			PodNetwork: &tunneler.Config{
				ExternalNetViaPodVM: true,
			},
		}

		ret := NewDaemon(config, DefaultListenAddr, nil, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.True(t, d.externalNetViaPodVM)
	})

	t.Run("handles TLS config with existing cert auth", func(t *testing.T) {
		config := &Config{
			TLSServerCert: "cert-data",
			TLSServerKey:  "key-data",
		}
		tlsConfig := &tlsutil.TLSConfig{
			CertFile: "/path/to/cert",
			KeyFile:  "/path/to/key",
		}

		ret := NewDaemon(config, DefaultListenAddr, tlsConfig, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.NotNil(t, d.tlsConfig)
		// Should not overwrite existing cert auth
		assert.Empty(t, d.tlsConfig.CertData)
	})

	t.Run("handles TLS config with existing CA", func(t *testing.T) {
		config := &Config{
			TLSClientCA: "ca-data",
		}
		tlsConfig := &tlsutil.TLSConfig{
			CAFile: "/path/to/ca",
		}

		ret := NewDaemon(config, DefaultListenAddr, tlsConfig, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, ret)

		d, ok := ret.(*daemon)
		require.True(t, ok)
		assert.NotNil(t, d.tlsConfig)
		// Should not overwrite existing CA
		assert.Empty(t, d.tlsConfig.CAData)
	})
}

func TestDaemonStart(t *testing.T) {
	t.Run("starts and stops successfully", func(t *testing.T) {
		podNode := &mockPodNode{}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "127.0.0.1:0", // Use port 0 for automatic assignment
		}

		errCh := make(chan error)
		go func() {
			defer close(errCh)

			if err := d.Start(context.Background()); err != nil {
				errCh <- err
			}
		}()

		// Wait for daemon to be ready
		select {
		case <-d.readyCh:
			// Daemon is ready
		case err := <-errCh:
			t.Fatalf("Expected daemon to start, got error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon to be ready")
		}

		// Verify pod node setup was called
		assert.True(t, podNode.setupCalled, "Expected Setup to be called")

		// Shutdown the daemon
		err := d.Shutdown()
		require.NoError(t, err, "Expected no error on shutdown")

		// Wait for daemon to stop
		select {
		case err := <-errCh:
			assert.NoError(t, err, "Expected no error from Start")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon to stop")
		}

		// Verify pod node teardown was called
		assert.True(t, podNode.teardownCalled, "Expected Teardown to be called")
	})

	t.Run("handles pod node setup error", func(t *testing.T) {
		expectedErr := assert.AnError
		podNode := &mockPodNode{
			setupError: expectedErr,
		}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "127.0.0.1:0",
		}

		err := d.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set up pod network")
		assert.True(t, podNode.setupCalled)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		podNode := &mockPodNode{}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "127.0.0.1:0",
		}

		ctx, cancel := context.WithCancel(context.Background())

		errCh := make(chan error)
		go func() {
			defer close(errCh)
			if err := d.Start(ctx); err != nil {
				errCh <- err
			}
		}()

		// Wait for daemon to be ready
		select {
		case <-d.readyCh:
			// Daemon is ready
		case err := <-errCh:
			t.Fatalf("Expected daemon to start, got error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon to be ready")
		}

		// Cancel context
		cancel()

		// Wait for daemon to stop
		select {
		case err := <-errCh:
			assert.NoError(t, err, "Expected no error from Start")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon to stop")
		}

		assert.True(t, podNode.setupCalled)
		assert.True(t, podNode.teardownCalled)
	})

	t.Run("handles invalid listen address", func(t *testing.T) {
		podNode := &mockPodNode{}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "invalid:address:format",
		}

		err := d.Start(context.Background())
		assert.Error(t, err)
		assert.True(t, podNode.setupCalled)
		assert.True(t, podNode.teardownCalled)
	})
}

func TestDaemonShutdown(t *testing.T) {
	t.Run("closes stop channel", func(t *testing.T) {
		d := &daemon{
			stopCh: make(chan struct{}),
		}

		err := d.Shutdown()
		require.NoError(t, err)

		// Verify stopCh is closed
		select {
		case <-d.stopCh:
			// Channel is closed as expected
		default:
			t.Fatal("Expected stopCh to be closed")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		d := &daemon{
			stopCh: make(chan struct{}),
		}

		// First shutdown
		err := d.Shutdown()
		require.NoError(t, err)

		// Second shutdown should not panic
		err = d.Shutdown()
		require.NoError(t, err)

		// Verify stopCh is still closed
		select {
		case <-d.stopCh:
			// Channel is closed as expected
		default:
			t.Fatal("Expected stopCh to be closed")
		}
	})

	t.Run("can be called multiple times concurrently", func(t *testing.T) {
		d := &daemon{
			stopCh: make(chan struct{}),
		}

		done := make(chan struct{})
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- struct{}{} }()
				_ = d.Shutdown()
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify stopCh is closed
		select {
		case <-d.stopCh:
			// Channel is closed as expected
		default:
			t.Fatal("Expected stopCh to be closed")
		}
	})
}

func TestDaemonReady(t *testing.T) {
	t.Run("returns ready channel", func(t *testing.T) {
		readyCh := make(chan struct{})
		d := &daemon{
			readyCh: readyCh,
		}

		ch := d.Ready()
		assert.Equal(t, readyCh, ch)
	})

	t.Run("ready channel is initially open", func(t *testing.T) {
		d := &daemon{
			readyCh: make(chan struct{}),
		}

		select {
		case <-d.Ready():
			t.Fatal("Expected ready channel to be open")
		default:
			// Channel is open as expected
		}
	})

	t.Run("ready channel closes when daemon starts", func(t *testing.T) {
		podNode := &mockPodNode{}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "127.0.0.1:0",
		}

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- d.Start(context.Background())
		}()

		// Wait for ready channel to close
		select {
		case <-d.Ready():
			// Channel closed as expected
		case err := <-startErrCh:
			require.NoError(t, err, "daemon failed to start before becoming ready")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for ready channel to close")
		}

		require.NoError(t, d.Shutdown())

		// Wait for Start() to exit cleanly
		select {
		case err := <-startErrCh:
			require.NoError(t, err, "daemon failed to stop cleanly")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon Start() to exit after Shutdown()")
		}
	})
}

func TestDaemonAddr(t *testing.T) {
	t.Run("returns listen address after ready", func(t *testing.T) {
		podNode := &mockPodNode{}
		d := &daemon{
			interceptor: agentproto.NewRedirector(dummyDialer),
			podNode:     podNode,
			readyCh:     make(chan struct{}),
			stopCh:      make(chan struct{}),
			listenAddr:  "127.0.0.1:0",
		}

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- d.Start(context.Background())
		}()

		// Wait for daemon to be ready with timeout
		select {
		case <-d.Ready():
			// Daemon is ready as expected
		case err := <-startErrCh:
			require.NoError(t, err, "daemon failed to start before becoming ready")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon to become ready")
		}

		// Now call Addr() with timeout protection
		addrCh := make(chan string, 1)
		go func() {
			addrCh <- d.Addr()
		}()

		select {
		case addr := <-addrCh:
			assert.NotEmpty(t, addr)
			assert.Contains(t, addr, "127.0.0.1:")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Addr() to return")
		}

		require.NoError(t, d.Shutdown())

		// Wait for Start() to exit cleanly
		select {
		case err := <-startErrCh:
			require.NoError(t, err, "daemon failed to stop cleanly")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for daemon Start() to exit after Shutdown()")
		}
	})

	t.Run("blocks until daemon is ready", func(t *testing.T) {
		d := &daemon{
			readyCh:    make(chan struct{}),
			listenAddr: "127.0.0.1:8080",
		}

		addrCh := make(chan string)
		go func() {
			addrCh <- d.Addr()
		}()

		// Verify Addr() is blocking
		select {
		case <-addrCh:
			t.Fatal("Expected Addr() to block until ready")
		case <-time.After(100 * time.Millisecond):
			// Addr() is blocking as expected
		}

		// Close ready channel
		close(d.readyCh)

		// Now Addr() should return
		select {
		case addr := <-addrCh:
			assert.Equal(t, "127.0.0.1:8080", addr)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Addr() to return")
		}
	})
}

func TestDaemonConstants(t *testing.T) {
	t.Run("default constants are set correctly", func(t *testing.T) {
		assert.Equal(t, "0.0.0.0", DefaultListenHost)
		assert.Equal(t, "15150", DefaultListenPort)
		assert.Equal(t, "0.0.0.0:15150", DefaultListenAddr)
		assert.Equal(t, "/run/peerpod/apf.json", DefaultConfigPath)
		assert.Equal(t, "/run/peerpod/podnetwork.json", DefaultPodNetworkSpecPath)
		assert.Equal(t, "/run/kata-containers/agent.sock", DefaultKataAgentSocketPath)
		assert.Equal(t, "/run/netns/podns", DefaultPodNamespace)
		assert.Equal(t, "/agent", AgentURLPath)
	})
}

func TestConfigStructure(t *testing.T) {
	t.Run("config has expected fields", func(t *testing.T) {
		config := &Config{
			PodNamespace:  "test-namespace",
			PodName:       "test-pod",
			TLSServerKey:  "key",
			TLSServerCert: "cert",
			TLSClientCA:   "ca",
			PpPrivateKey:  []byte("priv-key"),
			WnPublicKey:   []byte("pub-key"),
		}

		assert.Equal(t, "test-namespace", config.PodNamespace)
		assert.Equal(t, "test-pod", config.PodName)
		assert.Equal(t, "key", config.TLSServerKey)
		assert.Equal(t, "cert", config.TLSServerCert)
		assert.Equal(t, "ca", config.TLSClientCA)
		assert.Equal(t, []byte("priv-key"), config.PpPrivateKey)
		assert.Equal(t, []byte("pub-key"), config.WnPublicKey)
	})

	t.Run("config can be empty", func(t *testing.T) {
		config := &Config{}

		assert.Empty(t, config.PodNamespace)
		assert.Empty(t, config.PodName)
		assert.Empty(t, config.TLSServerKey)
		assert.Empty(t, config.TLSServerCert)
		assert.Empty(t, config.TLSClientCA)
		assert.Nil(t, config.PpPrivateKey)
		assert.Nil(t, config.WnPublicKey)
	})
}

func TestDaemonInterface(t *testing.T) {
	t.Run("daemon implements Daemon interface", func(t *testing.T) {
		config := &Config{}
		d := NewDaemon(config, DefaultListenAddr, nil, agentproto.NewRedirector(dummyDialer), &mockPodNode{})

		// Verify the concrete implementation satisfies the Daemon interface
		var _ Daemon = (*daemon)(nil)

		// Verify all interface methods are available
		assert.NotNil(t, d.Ready())
		assert.NotPanics(t, func() { _ = d.Shutdown() })

		// Note: d.Addr() blocks until ready channel is closed, so we don't call it here
		// It's tested separately in TestDaemonAddr
	})
}

func TestDaemonWithTLSConfig(t *testing.T) {
	t.Run("daemon with TLS config has proper initialization", func(t *testing.T) {
		config := &Config{
			TLSServerCert: "cert-data",
			TLSServerKey:  "key-data",
			TLSClientCA:   "ca-data",
		}
		tlsConfig := &tlsutil.TLSConfig{}

		d := NewDaemon(config, DefaultListenAddr, tlsConfig, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, d)

		daemonImpl, ok := d.(*daemon)
		require.True(t, ok)

		assert.NotNil(t, daemonImpl.tlsConfig)
		assert.Equal(t, []byte("cert-data"), daemonImpl.tlsConfig.CertData)
		assert.Equal(t, []byte("key-data"), daemonImpl.tlsConfig.KeyData)
		assert.Equal(t, []byte("ca-data"), daemonImpl.tlsConfig.CAData)
	})

	t.Run("daemon without TLS config has nil tlsConfig", func(t *testing.T) {
		config := &Config{}

		d := NewDaemon(config, DefaultListenAddr, nil, agentproto.NewRedirector(dummyDialer), &mockPodNode{})
		require.NotNil(t, d)

		daemonImpl, ok := d.(*daemon)
		require.True(t, ok)

		assert.Nil(t, daemonImpl.tlsConfig)
	})
}
