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

	clientCertPEM, _, err := NewClientCertificate("test-client")
	require.NoError(t, err)

	tests := []struct {
		name        string
		setupConfig func() *TLSConfig
		wantErr     bool
		errContains string
		checkResult func(*testing.T, *tls.Config)
	}{
		{
			name: "default empty profile uses TLS 1.2 floor",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
			},
		},
		{
			name: "VersionTLS13 is applied",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM, MinTLSVersion: "VersionTLS13"}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
			},
		},
		{
			name: "valid cipher suites are applied",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM,
					CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, cfg.CipherSuites)
			},
		},
		{
			name: "invalid MinTLSVersion returns error",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM, MinTLSVersion: "VersionTLS11"}
			},
			wantErr:     true,
			errContains: "invalid TLS profile",
		},
		{
			name: "invalid cipher suite name returns error",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM,
					CipherSuites: []string{"BOGUS_CIPHER"}}
			},
			wantErr:     true,
			errContains: "invalid TLS profile",
		},
		{
			name: "Option B floor: empty version cannot go below TLS 1.2",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CAData: clientCertPEM, CertData: certPEM, KeyData: keyPEM}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12))
			},
		},
		{
			name: "MinTLSVersion alone (no CA/cert) returns non-nil config",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{MinTLSVersion: "VersionTLS13"}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
			},
		},
		{
			name: "CipherSuites alone (no CA/cert) returns non-nil config",
			setupConfig: func() *TLSConfig {
				return &TLSConfig{CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}}
			},
			checkResult: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, cfg.CipherSuites)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := GetTLSConfigFor(tt.setupConfig())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.checkResult != nil {
				tt.checkResult(t, cfg)
			}
		})
	}
}

func TestGetTLSConfigForTLS13Enforcement(t *testing.T) {
	ca, err := NewCAService("test-ca")
	require.NoError(t, err)

	certPEM, keyPEM, err := ca.Issue("test-server")
	require.NoError(t, err)

	clientCertPEM, clientKeyPEM, err := NewClientCertificate("test-client")
	require.NoError(t, err)

	caCertPEM := ca.RootCertificate()

	serverTLS, err := GetTLSConfigFor(&TLSConfig{
		CAData:        clientCertPEM,
		CertData:      certPEM,
		KeyData:       keyPEM,
		MinTLSVersion: "VersionTLS13",
	})
	require.NoError(t, err)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	require.NoError(t, err)
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

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
}
