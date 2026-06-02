// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"fmt"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
)

type tlsProfile struct {
	MinVersion   uint16
	CipherSuites []uint16
}

func parseTLSOptions(minVersion string, cipherSuites []string) (*tlsProfile, error) {
	minVersion = strings.TrimSpace(minVersion)

	var cleaned []string
	for _, s := range cipherSuites {
		if s = strings.TrimSpace(s); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	cipherSuites = cleaned

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

	p := &tlsProfile{MinVersion: version}

	if len(cipherSuites) > 0 {
		ids, err := cliflag.TLSCipherSuites(cipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid cipherSuites: %w; valid names: %v", err, cliflag.PreferredTLSCipherNames())
		}
		p.CipherSuites = ids
	}

	return p, nil
}
