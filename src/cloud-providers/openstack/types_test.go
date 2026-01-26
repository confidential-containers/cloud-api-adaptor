// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"testing"
)

func TestRedact(t *testing.T) {
	// Prepare Config values for testing
	cfg := Config{
		IdentityEndpoint:    "https://identity.test-openstack/v3",
		Username:            "test-user",
		TenantName:          "test-tenant",
		Password:            "test-password",
		DomainName:          "test-domain",
		Region:              "test-region",
		ServerPrefix:        "test-vm-name",
		ImageID:             "test-image-id",
		FlavorID:            "test-flavor-id",
		NetworkIDs:          []string{"net1"},
		SecurityGroups:      []string{"sg1"},
		FloatingIpNetworkID: "floating-net",
	}

	redactedCfg := cfg.Redact()
	t.Logf("Config: %v", redactedCfg)

	// Check if values are masked
	maskingCfg := "**********"

	if redactedCfg.Username != maskingCfg {
		t.Errorf("Username not redacted: %s", redactedCfg.Username)
	}
	if redactedCfg.Password != maskingCfg {
		t.Errorf("Password not redacted: %s", redactedCfg.Password)
	}
	if redactedCfg.TenantName != maskingCfg {
		t.Errorf("TenantName not redacted: %s", redactedCfg.TenantName)
	}
}
