// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

// Package tlsconfig reads the cluster TLS profile that the operator injects
// into peer pods components.
package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
)

// Profile is a parsed TLS profile.
type Profile struct {
	MinVersion   uint16
	CipherSuites []uint16
}

// Parse validates a minimum TLS version and cipher suite names. It returns nil
// when both are empty, so callers keep Go's defaults.
func Parse(minVersion string, cipherSuites []string) (*Profile, error) {
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

	p := &Profile{MinVersion: version}

	if len(cipherSuites) > 0 {
		ids, err := cliflag.TLSCipherSuites(cipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid cipherSuites: %w; valid names: %v", err, cliflag.PreferredTLSCipherNames())
		}
		p.CipherSuites = ids
	}

	return p, nil
}

// OptionsFromEnv builds TLS options from the TLS_MIN_VERSION and
// TLS_CIPHER_SUITES environment variables. It returns no options when both are
// unset.
func OptionsFromEnv() ([]func(*tls.Config), error) {
	var cipherSuites []string
	if cs := os.Getenv("TLS_CIPHER_SUITES"); cs != "" {
		cipherSuites = strings.Split(cs, ",")
	}

	profile, err := Parse(os.Getenv("TLS_MIN_VERSION"), cipherSuites)
	if err != nil || profile == nil {
		return nil, err
	}

	return []func(*tls.Config){func(c *tls.Config) {
		c.MinVersion = profile.MinVersion
		c.CipherSuites = profile.CipherSuites
	}}, nil
}
