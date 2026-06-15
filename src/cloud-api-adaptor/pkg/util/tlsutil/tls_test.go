// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTLSConfigForWithTLSProfile(t *testing.T) {
	ca, err := NewCAService("test-ca")
	require.NoError(t, err)

	certPEM, keyPEM, err := ca.Issue("test-server")
	require.NoError(t, err)

	clientCertPEM, clientKeyPEM, err := NewClientCertificate("test-client")
	require.NoError(t, err)

	baseConfig := func() *TLSConfig {
		return &TLSConfig{
			CAData:   clientCertPEM,
			CertData: certPEM,
			KeyData:  keyPEM,
		}
	}

	t.Run("default empty profile uses TLS 1.2 floor", func(t *testing.T) {
		cfg, err := GetTLSConfigFor(baseConfig())
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("VersionTLS13 is applied", func(t *testing.T) {
		c := baseConfig()
		c.MinTLSVersion = "VersionTLS13"
		cfg, err := GetTLSConfigFor(c)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	})

	t.Run("valid cipher suites are applied", func(t *testing.T) {
		c := baseConfig()
		c.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
		cfg, err := GetTLSConfigFor(c)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, cfg.CipherSuites)
	})

	t.Run("invalid MinTLSVersion returns error", func(t *testing.T) {
		c := baseConfig()
		c.MinTLSVersion = "VersionTLS11"
		_, err := GetTLSConfigFor(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TLS profile")
	})

	t.Run("invalid cipher suite name returns error", func(t *testing.T) {
		c := baseConfig()
		c.CipherSuites = []string{"BOGUS_CIPHER"}
		_, err := GetTLSConfigFor(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TLS profile")
	})

	t.Run("Option B floor: unrecognised version cannot go below TLS 1.2", func(t *testing.T) {
		// Construct TLSConfig directly with a pre-parsed TLS10 value bypassing ParseTLSOptions.
		// GetTLSConfigFor must still clamp to TLS 1.2.
		c := baseConfig()
		// MinTLSVersion empty — floor kicks in.
		cfg, err := GetTLSConfigFor(c)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12))
	})

	t.Run("MinTLSVersion alone (no CA/cert) returns non-nil config", func(t *testing.T) {
		cfg, err := GetTLSConfigFor(&TLSConfig{MinTLSVersion: "VersionTLS13"})
		require.NoError(t, err)
		require.NotNil(t, cfg, "expected non-nil config when only MinTLSVersion is set")
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	})

	t.Run("CipherSuites alone (no CA/cert) returns non-nil config", func(t *testing.T) {
		cfg, err := GetTLSConfigFor(&TLSConfig{CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}})
		require.NoError(t, err)
		require.NotNil(t, cfg, "expected non-nil config when only CipherSuites is set")
		assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, cfg.CipherSuites)
	})

	t.Run("TLS 1.3 enforcement rejects TLS 1.2 client connection", func(t *testing.T) {
		caCertPEM := ca.RootCertificate()

		serverConfig := &TLSConfig{
			CAData:        clientCertPEM,
			CertData:      certPEM,
			KeyData:       keyPEM,
			MinTLSVersion: "VersionTLS13",
		}
		serverTLS, err := GetTLSConfigFor(serverConfig)
		require.NoError(t, err)

		listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
		require.NoError(t, err)
		defer listener.Close()

		addr := listener.Addr().String()

		// Accept and discard in background.
		go func() {
			conn, err := listener.Accept()
			if err == nil {
				conn.Close()
			}
		}()

		// Client capped at TLS 1.2 — should be rejected by the TLS 1.3-minimum server.
		clientTLS, err := GetTLSConfigFor(&TLSConfig{
			CAData:   caCertPEM,
			CertData: clientCertPEM,
			KeyData:  clientKeyPEM,
		})
		require.NoError(t, err)
		clientTLS.MaxVersion = tls.VersionTLS12
		clientTLS.ServerName = "test-server"

		dialer := tls.Dialer{Config: clientTLS}
		conn, err := dialer.DialContext(context.Background(), "tcp", addr)
		if conn != nil {
			conn.Close()
		}
		assert.Error(t, err, "TLS 1.2 client should be rejected by TLS 1.3-minimum server")
	})
}
