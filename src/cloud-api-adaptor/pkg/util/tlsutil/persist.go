// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// tlsMaterial is the JSON schema used to persist TLS material on disk.
type tlsMaterial struct {
	CACertPEM     []byte `json:"ca_cert_pem"`
	CAKeyPEM      []byte `json:"ca_key_pem"`
	ClientCertPEM []byte `json:"client_cert_pem"`
	ClientKeyPEM  []byte `json:"client_key_pem"`
}

// LoadOrCreateTLSMaterial loads persisted TLS material from path, or generates
// fresh material and persists it when none exists. The returned CAService can
// issue per-pod-VM server certificates.
func LoadOrCreateTLSMaterial(path string) (CAService, []byte, []byte, error) {
	// Try load existing
	if data, readErr := os.ReadFile(path); readErr == nil {
		var mat tlsMaterial
		if jsonErr := json.Unmarshal(data, &mat); jsonErr == nil &&
			len(mat.CACertPEM) > 0 && len(mat.CAKeyPEM) > 0 &&
			len(mat.ClientCertPEM) > 0 && len(mat.ClientKeyPEM) > 0 {
			svc := &caService{
				orgName: "agent-protocol-forwarder",
				certPEM: mat.CACertPEM,
				keyPEM:  mat.CAKeyPEM,
			}
			return svc, mat.ClientCertPEM, mat.ClientKeyPEM, nil
		}
	}

	// Generate new CA
	svc, err := NewCAService("agent-protocol-forwarder")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating CA: %w", err)
	}

	// Generate new client certificate
	clientCertPEM, clientKeyPEM, err := NewClientCertificate("cloud-api-adaptor")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating client certificate: %w", err)
	}

	// Persist - access internal fields via type assertion
	inner := svc.(*caService)
	mat := tlsMaterial{
		CACertPEM:     inner.certPEM,
		CAKeyPEM:      inner.keyPEM,
		ClientCertPEM: clientCertPEM,
		ClientKeyPEM:  clientKeyPEM,
	}
	data, err := json.Marshal(mat)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling TLS material: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, nil, fmt.Errorf("creating TLS material directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, nil, nil, fmt.Errorf("writing TLS material: %w", err)
	}

	return svc, clientCertPEM, clientKeyPEM, nil
}
