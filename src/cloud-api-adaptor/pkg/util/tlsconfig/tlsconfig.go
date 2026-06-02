// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsconfig

import (
	"crypto/tls"
	"fmt"

	cliflag "k8s.io/component-base/cli/flag"
)

// TLS holds parsed TLS profile options as uint16 values ready for use in tls.Config.
type TLS struct {
	MinVersion   uint16
	CipherSuites []uint16
}

// ParseTLSOptions parses TLS version and cipher suite name strings into a TLS struct.
// Returns nil, nil when both inputs are empty.
// Rejects TLS 1.0 and 1.1.
// Rejects cipher suites when minVersion is VersionTLS13, since Go's crypto/tls
// does not allow configuring TLS 1.3 cipher suites.
func ParseTLSOptions(minVersion string, cipherSuites []string) (*TLS, error) {
	if minVersion == "" && len(cipherSuites) == 0 {
		return nil, nil
	}

	if minVersion == "VersionTLS10" || minVersion == "VersionTLS11" {
		return nil, fmt.Errorf("invalid minVersion %q: TLS 1.0 and 1.1 are not supported, use VersionTLS12 or VersionTLS13", minVersion)
	}

	version, err := cliflag.TLSVersion(minVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid minVersion %q: %w", minVersion, err)
	}

	if version == tls.VersionTLS13 && len(cipherSuites) > 0 {
		return nil, fmt.Errorf("cipherSuites may not be specified when minVersion is VersionTLS13: Go's crypto/tls does not allow configuring TLS 1.3 cipher suites")
	}

	t := &TLS{MinVersion: version}

	if len(cipherSuites) > 0 {
		ids, err := cliflag.TLSCipherSuites(cipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid cipherSuites: %w; valid names: %v", err, cliflag.PreferredTLSCipherNames())
		}
		t.CipherSuites = ids
	}

	return t, nil
}
