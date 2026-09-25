// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	// RevertResources restores the KBS to its pre-test state, undoing the
	// resources a test provisioned. Backends whose KBS is torn down wholesale at
	// the end of the run (see Delete) may implement this as a no-op.
	RevertResources() error
	// Delete tears down a framework-deployed KBS at environment teardown. It is a
	// no-op for a pre-installed, externally-managed KBS.
	Delete(ctx context.Context, cfg *envconf.Config) error
}

// Ensure the kbs-client backed implementation satisfies the interface.
var _ KbsManager = (*KeyBrokerService)(nil)

// DefaultKbsManagement is the backend used when KBS_MANAGEMENT is unset: the
// Trustee the test framework deploys itself.
const DefaultKbsManagement = "kbs-client"

// NewKbsManagerFunc builds a KbsManager for an already-selected backend.
type NewKbsManagerFunc func(ctx context.Context, cfg *envconf.Config) (KbsManager, error)

// NewKbsManagerFunctions maps a KBS_MANAGEMENT value to its backend
// constructor. Backends register themselves from init(), so an out-of-tree
// implementation only needs its package linked into the test binary.
var NewKbsManagerFunctions = make(map[string]NewKbsManagerFunc)

// RegisteredKbsManagers returns the registered backend names, sorted.
func RegisteredKbsManagers() []string {
	names := make([]string, 0, len(NewKbsManagerFunctions))
	for name := range NewKbsManagerFunctions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LookupKbsManager returns the constructor registered under name, without
// constructing anything.
func LookupKbsManager(name string) (NewKbsManagerFunc, error) {
	newKbsManager, ok := NewKbsManagerFunctions[name]
	if !ok {
		return nil, fmt.Errorf("no KBS manager registered for %q, registered backends are: %s",
			name, strings.Join(RegisteredKbsManagers(), ", "))
	}

	return newKbsManager, nil
}

// GetKbsManager returns the KBS backend registered under name.
func GetKbsManager(ctx context.Context, cfg *envconf.Config, name string) (KbsManager, error) {
	newKbsManager, err := LookupKbsManager(name)
	if err != nil {
		return nil, err
	}

	return newKbsManager(ctx, cfg)
}
