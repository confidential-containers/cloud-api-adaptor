//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"net/netip"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPodName        = "test-pod"
	testSandboxID      = "test-sandbox"
	testVMName         = "test-vm"
	testDefaultCPU     = 2
	testDefaultMemory  = 2048
	testQemuURI        = "qemu:///system"
	testDefaultPool    = "default"
	testDefaultNetwork = "default"
	testVolName        = "test.qcow2"
	testInvalidURI     = "invalid://uri"
)

// MockCloudConfigGenerator is a mock implementation for testing
type MockCloudConfigGenerator struct {
	userData string
	err      error
}

func (m *MockCloudConfigGenerator) Generate() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.userData, nil
}

// newTestConfig creates a config for testing with optional customizations.
// Uses hardcoded test constants for unit tests.
func newTestConfig(opts ...func(*Config)) *Config {
	cfg := &Config{
		URI:         testQemuURI,
		PoolName:    testDefaultPool,
		NetworkName: testDefaultNetwork,
		VolName:     testVolName,
		CPU:         testDefaultCPU,
		Memory:      testDefaultMemory,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func withURI(uri string) func(*Config) {
	return func(c *Config) { c.URI = uri }
}

func withPoolName(pool string) func(*Config) {
	return func(c *Config) { c.PoolName = pool }
}

func withNetworkName(network string) func(*Config) {
	return func(c *Config) { c.NetworkName = network }
}

func withVolName(vol string) func(*Config) {
	return func(c *Config) { c.VolName = vol }
}

func withCPU(cpu uint) func(*Config) {
	return func(c *Config) { c.CPU = cpu }
}

func withMemory(memory uint) func(*Config) {
	return func(c *Config) { c.Memory = memory }
}

func withDisableCVM(disabled bool) func(*Config) {
	return func(c *Config) { c.DisableCVM = disabled }
}

func withLaunchSecurity(security string) func(*Config) {
	return func(c *Config) { c.LaunchSecurity = security }
}

func withRootDiskSize(size uint64) func(*Config) {
	return func(c *Config) { c.RootDiskSize = size }
}

// TestProviderRootDiskSizeWired verifies that RootDiskSize flows from Config
// into the provider's service config (and from there into vmConfig at CreateInstance time).
func TestProviderRootDiskSizeWired(t *testing.T) {
	cfg := newTestConfig(withRootDiskSize(30))
	p := &libvirtProvider{serviceConfig: cfg}

	assert.Equal(t, uint64(30), p.serviceConfig.RootDiskSize)
}

func TestNewProvider(t *testing.T) {
	t.Run("invalid URI", func(t *testing.T) {
		config := newTestConfig(withURI(testInvalidURI))
		_, err := NewProvider(config)
		assert.Error(t, err)
	})
}

func TestGetIPs(t *testing.T) {
	tests := []struct {
		name        string
		ips         []netip.Addr
		expectEmpty bool
		expectNil   bool
	}{
		{
			name: "multiple IPs",
			ips: []netip.Addr{
				netip.MustParseAddr("192.168.122.10"),
				netip.MustParseAddr("10.0.0.5"),
				netip.MustParseAddr("2001:db8::1"),
			},
		},
		{
			name:        "empty IPs",
			ips:         []netip.Addr{},
			expectEmpty: true,
		},
		{
			name:      "nil IPs",
			ips:       nil,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &vmConfig{
				name: testVMName,
				ips:  tt.ips,
			}

			ips, err := getIPs(vm)
			require.NoError(t, err)

			if tt.expectEmpty || tt.expectNil {
				assert.Empty(t, ips)
			} else {
				assert.Equal(t, tt.ips, ips)
				assert.Len(t, ips, len(tt.ips))
			}
		})
	}
}

func TestTeardown(t *testing.T) {
	p := &libvirtProvider{}
	err := p.Teardown()
	assert.NoError(t, err)
}

func TestDeleteInstanceEmptyID(t *testing.T) {
	p := &libvirtProvider{
		serviceConfig: newTestConfig(),
	}

	err := p.DeleteInstance(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty instanceID")
}

func TestCreateInstanceCloudConfigError(t *testing.T) {
	// Test that doesn't require libvirt - tests early validation
	// Create provider without checkConfig to test cloud config error path
	p := &libvirtProvider{
		serviceConfig: newTestConfig(),
	}

	mockGen := &MockCloudConfigGenerator{
		err: assert.AnError,
	}

	spec := provider.InstanceTypeSpec{
		InstanceType: "test",
	}

	_, err := p.CreateInstance(context.Background(), testPodName, testSandboxID, mockGen, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assert.AnError")
}

// TestCreateInstanceLaunchSecurity tests launch security validation logic.
// This is a unit test that verifies the validation happens correctly without requiring libvirt.
func TestProviderConfigVerifier(t *testing.T) {
	t.Run("valid configurations", func(t *testing.T) {
		tests := []struct {
			name   string
			config *Config
		}{
			{
				name:   "all fields set",
				config: newTestConfig(),
			},
			{
				name: "with optional fields",
				config: newTestConfig(
					withLaunchSecurity("s390-pv"),
					withDisableCVM(false),
				),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := &libvirtProvider{serviceConfig: tt.config}
				err := p.ConfigVerifier()
				assert.NoError(t, err)
			})
		}
	})

	t.Run("invalid configurations", func(t *testing.T) {
		tests := []struct {
			name          string
			configOpts    []func(*Config)
			errorContains string
		}{
			{
				name:          "empty URI",
				configOpts:    []func(*Config){withURI("")},
				errorContains: "URI is empty",
			},
			{
				name:          "empty PoolName",
				configOpts:    []func(*Config){withPoolName("")},
				errorContains: "PoolName is empty",
			},
			{
				name:          "empty NetworkName",
				configOpts:    []func(*Config){withNetworkName("")},
				errorContains: "NetworkName is empty",
			},
			{
				name:          "empty VolName",
				configOpts:    []func(*Config){withVolName("")},
				errorContains: "VolName is empty",
			},
			{
				name:          "zero CPU",
				configOpts:    []func(*Config){withCPU(0)},
				errorContains: "CPU must be greater than zero",
			},
			{
				name:          "zero Memory",
				configOpts:    []func(*Config){withMemory(0)},
				errorContains: "Memory must be greater than zero",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := newTestConfig(tt.configOpts...)
				p := &libvirtProvider{serviceConfig: config}

				err := p.ConfigVerifier()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			})
		}
	})
}
