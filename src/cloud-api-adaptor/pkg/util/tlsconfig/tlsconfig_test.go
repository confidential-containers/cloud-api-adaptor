// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTLSOptions(t *testing.T) {
	t.Run("empty inputs returns nil", func(t *testing.T) {
		result, err := ParseTLSOptions("", nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("VersionTLS12 parses correctly", func(t *testing.T) {
		result, err := ParseTLSOptions("VersionTLS12", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint16(tls.VersionTLS12), result.MinVersion)
		assert.Empty(t, result.CipherSuites)
	})

	t.Run("VersionTLS13 parses correctly", func(t *testing.T) {
		result, err := ParseTLSOptions("VersionTLS13", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint16(tls.VersionTLS13), result.MinVersion)
	})

	t.Run("VersionTLS10 is rejected", func(t *testing.T) {
		_, err := ParseTLSOptions("VersionTLS10", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS 1.0 and 1.1 are not supported")
	})

	t.Run("VersionTLS11 is rejected", func(t *testing.T) {
		_, err := ParseTLSOptions("VersionTLS11", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS 1.0 and 1.1 are not supported")
	})

	t.Run("unknown version string returns error", func(t *testing.T) {
		_, err := ParseTLSOptions("VersionTLS99", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid minVersion")
	})

	t.Run("valid cipher suites parsed correctly", func(t *testing.T) {
		suites := []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"}
		result, err := ParseTLSOptions("VersionTLS12", suites)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.CipherSuites, 2)
		assert.Equal(t, uint16(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256), result.CipherSuites[0])
		assert.Equal(t, uint16(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384), result.CipherSuites[1])
	})

	t.Run("unknown cipher suite name returns error", func(t *testing.T) {
		_, err := ParseTLSOptions("VersionTLS12", []string{"INVALID_CIPHER_SUITE"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cipherSuites")
	})

	t.Run("cipher suites with VersionTLS13 returns error", func(t *testing.T) {
		_, err := ParseTLSOptions("VersionTLS13", []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "may not be specified when minVersion is VersionTLS13")
	})

	t.Run("empty version with cipher suites uses TLS12 default", func(t *testing.T) {
		suites := []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
		result, err := ParseTLSOptions("", suites)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint16(tls.VersionTLS12), result.MinVersion)
		assert.Len(t, result.CipherSuites, 1)
	})
}
