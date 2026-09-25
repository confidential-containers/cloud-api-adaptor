// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// KbsManager abstracts interaction with a Key Broker Service (Trustee),
// allowing the CoCo tests to run against different KBS deployments (for
// example a test-framework-deployed KBS driven through the kbs-client admin
// API, or a pre-installed operator-managed KBS).
type KbsManager interface {
	// GetKbsEndpoint discovers and caches the KBS endpoint.
	GetKbsEndpoint(ctx context.Context, cfg *envconf.Config) (string, error)
	// GetCachedKbsEndpoint returns the previously discovered KBS endpoint.
	GetCachedKbsEndpoint() (string, error)
	// SetSecret stores a secret at the given KBS resource path.
	SetSecret(resourcePath string, secret []byte) error
	// SetImageDecryptionKey stores an image decryption key at the given key ID.
	SetImageDecryptionKey(keyID string, key []byte) error
	// EnableKbsCustomizedResourcePolicy sets the KBS resource release policy.
	EnableKbsCustomizedResourcePolicy(customizedOpaFile string) error
	// EnableKbsCustomizedAttestationPolicy sets the KBS attestation policy.
	EnableKbsCustomizedAttestationPolicy(customizedOpaFile string) error
	// Delete tears down the KBS deployment.
	Delete(ctx context.Context, cfg *envconf.Config) error
}

// Compile-time interface checks.
var (
	_ KbsManager = (*KeyBrokerService)(nil)
	_ KbsManager = (*operatorKbsManager)(nil)
)
