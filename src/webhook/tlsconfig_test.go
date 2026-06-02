// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTLSOptions(t *testing.T) {
	tests := []struct {
		name           string
		minVersion     string
		cipherSuites   []string
		wantNil        bool
		wantMinVersion uint16
		wantCipherLen  int
		wantErr        bool
		errContains    string
	}{
		{
			name:    "empty inputs returns nil",
			wantNil: true,
		},
		{
			name:           "VersionTLS12 parses correctly",
			minVersion:     "VersionTLS12",
			wantMinVersion: tls.VersionTLS12,
		},
		{
			name:           "VersionTLS13 parses correctly",
			minVersion:     "VersionTLS13",
			wantMinVersion: tls.VersionTLS13,
		},
		{
			name:        "VersionTLS10 is rejected",
			minVersion:  "VersionTLS10",
			wantErr:     true,
			errContains: "TLS 1.0 and 1.1 are not supported",
		},
		{
			name:        "VersionTLS11 is rejected",
			minVersion:  "VersionTLS11",
			wantErr:     true,
			errContains: "TLS 1.0 and 1.1 are not supported",
		},
		{
			name:        "unknown version string returns error",
			minVersion:  "VersionTLS99",
			wantErr:     true,
			errContains: "invalid minVersion",
		},
		{
			name:           "valid cipher suites parsed correctly",
			minVersion:     "VersionTLS12",
			cipherSuites:   []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			wantMinVersion: tls.VersionTLS12,
			wantCipherLen:  2,
		},
		{
			name:         "unknown cipher suite name returns error",
			minVersion:   "VersionTLS12",
			cipherSuites: []string{"INVALID_CIPHER_SUITE"},
			wantErr:      true,
			errContains:  "invalid cipherSuites",
		},
		{
			name:         "cipher suites with VersionTLS13 returns error",
			minVersion:   "VersionTLS13",
			cipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			wantErr:      true,
			errContains:  "may not be specified when minVersion is VersionTLS13",
		},
		{
			name:           "empty version with cipher suites uses TLS12 default",
			cipherSuites:   []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			wantMinVersion: tls.VersionTLS12,
			wantCipherLen:  1,
		},
		{
			name:           "cipher suite names with leading/trailing spaces are trimmed",
			minVersion:     "VersionTLS12",
			cipherSuites:   []string{" TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 ", " TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			wantMinVersion: tls.VersionTLS12,
			wantCipherLen:  2,
		},
		{
			name:           "empty cipher suite entries are dropped",
			minVersion:     "VersionTLS12",
			cipherSuites:   []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "", " "},
			wantMinVersion: tls.VersionTLS12,
			wantCipherLen:  1,
		},
		{
			name:           "minVersion with surrounding whitespace is trimmed",
			minVersion:     "  VersionTLS13  ",
			wantMinVersion: tls.VersionTLS13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTLSOptions(tt.minVersion, tt.cipherSuites)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)

			if tt.wantMinVersion != 0 {
				assert.Equal(t, tt.wantMinVersion, result.MinVersion)
			}

			if tt.wantCipherLen > 0 {
				assert.Len(t, result.CipherSuites, tt.wantCipherLen)
			} else if tt.cipherSuites == nil {
				assert.Empty(t, result.CipherSuites)
			}
		})
	}
}
