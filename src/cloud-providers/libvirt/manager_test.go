// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetLibvirtConfig() {
	libvirtcfg = Config{
		URI:            defaultURI,
		PoolName:       defaultPoolName,
		NetworkName:    defaultNetworkName,
		DataDir:        defaultDataDir,
		VolName:        defaultVolName,
		LaunchSecurity: defaultLaunchSecurity,
		Firmware:       defaultFirmware,
		CPU:            2,
		Memory:         8192,
		DisableCVM:     true,
		RootDiskSize:   defaultRootDiskSize,
	}
}

// setupManagerTest creates a fresh Manager with flags for testing
func setupManagerTest(t *testing.T) (*Manager, *flag.FlagSet) {
	t.Helper()
	resetLibvirtConfig()
	manager := &Manager{}
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	manager.ParseCmd(flags)
	return manager, flags
}

func TestManagerParseCmdWithEnvVars(t *testing.T) {
	// Set environment variables
	t.Setenv("LIBVIRT_URI", "qemu+ssh://testhost/system")
	t.Setenv("LIBVIRT_POOL", "env-pool")
	t.Setenv("LIBVIRT_NET", "env-network")
	t.Setenv("LIBVIRT_VOL_NAME", "env-vol.qcow2")
	t.Setenv("LIBVIRT_LAUNCH_SECURITY", "sev")
	t.Setenv("LIBVIRT_EFI_FIRMWARE", "/env/firmware.fd")
	t.Setenv("LIBVIRT_CPU", "8")
	t.Setenv("LIBVIRT_MEMORY", "16384")
	t.Setenv("DISABLECVM", "false")
	t.Setenv("LIBVIRT_ROOT_DISK_SIZE", "20")

	_, _ = setupManagerTest(t)

	// Verify environment variables were applied
	assert.Equal(t, "qemu+ssh://testhost/system", libvirtcfg.URI)
	assert.Equal(t, "env-pool", libvirtcfg.PoolName)
	assert.Equal(t, "env-network", libvirtcfg.NetworkName)
	assert.Equal(t, "env-vol.qcow2", libvirtcfg.VolName)
	assert.Equal(t, "sev", libvirtcfg.LaunchSecurity)
	assert.Equal(t, "/env/firmware.fd", libvirtcfg.Firmware)
	assert.Equal(t, uint(8), libvirtcfg.CPU)
	assert.Equal(t, uint(16384), libvirtcfg.Memory)
	assert.False(t, libvirtcfg.DisableCVM)
	assert.Equal(t, uint64(20), libvirtcfg.RootDiskSize)
}

func TestManagerParseCmdFlagOverridesEnv(t *testing.T) {
	// Set environment variable
	t.Setenv("LIBVIRT_URI", "qemu+ssh://envhost/system")
	t.Setenv("LIBVIRT_CPU", "8")

	_, flags := setupManagerTest(t)

	// Verify env var was applied initially
	assert.Equal(t, "qemu+ssh://envhost/system", libvirtcfg.URI)
	assert.Equal(t, uint(8), libvirtcfg.CPU)

	// Override with flag
	err := flags.Set("uri", "qemu:///system")
	require.NoError(t, err)
	err = flags.Set("cpu", "4")
	require.NoError(t, err)

	// Verify flag overrides env var
	assert.Equal(t, "qemu:///system", libvirtcfg.URI)
	assert.Equal(t, uint(4), libvirtcfg.CPU)
}

func TestManagerParseCmdInvalidValues(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		invalidValue string
		description  string
	}{
		{
			name:         "invalid CPU value",
			flagName:     "cpu",
			invalidValue: "invalid",
			description:  "non-numeric CPU value should be rejected",
		},
		{
			name:         "invalid memory value",
			flagName:     "memory",
			invalidValue: "not-a-number",
			description:  "non-numeric memory value should be rejected",
		},
		{
			name:         "invalid bool value",
			flagName:     "disable-cvm",
			invalidValue: "maybe",
			description:  "invalid boolean value should be rejected",
		},
		{
			name:         "negative CPU value",
			flagName:     "cpu",
			invalidValue: "-1",
			description:  "negative CPU value should be rejected",
		},
		{
			name:         "negative memory value",
			flagName:     "memory",
			invalidValue: "-1",
			description:  "negative memory value should be rejected",
		},
		{
			name:         "invalid root-disk-size value",
			flagName:     "root-disk-size",
			invalidValue: "notanumber",
			description:  "non-numeric root-disk-size should be rejected",
		},
		{
			name:         "negative root-disk-size value",
			flagName:     "root-disk-size",
			invalidValue: "-5",
			description:  "negative root-disk-size should be rejected",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, flags := setupManagerTest(t)

			err := flags.Set(tc.flagName, tc.invalidValue)
			assert.Error(t, err, tc.description)
		})
	}
}

func TestManagerParseCmdEmptyStringValues(t *testing.T) {
	_, flags := setupManagerTest(t)

	// Set empty string values (should be allowed)
	err := flags.Set("uri", "")
	require.NoError(t, err)
	err = flags.Set("pool-name", "")
	require.NoError(t, err)
	err = flags.Set("launch-security", "")
	require.NoError(t, err)

	// Verify empty strings are set
	assert.Equal(t, "", libvirtcfg.URI)
	assert.Equal(t, "", libvirtcfg.PoolName)
	assert.Equal(t, "", libvirtcfg.LaunchSecurity)
}

func TestManagerLoadEnv(t *testing.T) {
	manager, _ := setupManagerTest(t)
	// LoadEnv should do nothing (it's a no-op)
	manager.LoadEnv()
	// No assertion needed, just verify it doesn't panic
}

func TestManagerGetConfig(t *testing.T) {
	manager, _ := setupManagerTest(t)
	config := manager.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, &libvirtcfg, config)
}

func TestManagerNewProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*testing.T, *flag.FlagSet)
		expectError bool
		description string
	}{
		{
			name: "valid config",
			setupConfig: func(t *testing.T, flags *flag.FlagSet) {
				resetLibvirtConfig()
				checkConfig(t)
				require.NoError(t, flags.Set("uri", testCfg.URI))
				require.NoError(t, flags.Set("pool-name", testCfg.PoolName))
				require.NoError(t, flags.Set("network-name", testCfg.NetworkName))
				require.NoError(t, flags.Set("vol-name", testCfg.VolName))
			},
			expectError: false,
			description: "should succeed with valid config",
		},
		{
			name: "invalid URI",
			setupConfig: func(t *testing.T, flags *flag.FlagSet) {
				require.NoError(t, flags.Set("uri", "invalid://uri"))
			},
			expectError: true,
			description: "should fail with invalid URI",
		},
		{
			name: "empty URI",
			setupConfig: func(t *testing.T, flags *flag.FlagSet) {
				require.NoError(t, flags.Set("uri", ""))
			},
			expectError: true,
			description: "should fail with empty URI",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manager, flags := setupManagerTest(t)
			tc.setupConfig(t, flags)

			provider, err := manager.NewProvider()
			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				require.NoError(t, err, tc.description)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	// Test all constants in a single assertion block
	assert.Equal(t, "qemu+ssh://root@192.168.122.1/system?no_verify=1", defaultURI)
	assert.Equal(t, "default", defaultPoolName)
	assert.Equal(t, "default", defaultNetworkName)
	assert.Equal(t, "/var/lib/libvirt/images", defaultDataDir)
	assert.Equal(t, "podvm-base.qcow2", defaultVolName)
	assert.Equal(t, "", defaultLaunchSecurity)
	assert.Equal(t, "/usr/share/OVMF/OVMF_CODE_4M.fd", defaultFirmware)
	assert.Equal(t, "2", defaultCPU)
	assert.Equal(t, "8192", defaultMemory)
	assert.Equal(t, uint64(10), defaultRootDiskSize)
}

func TestManagerParseCmdFlags(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		flagValue     string
		expectedValue interface{}
		getActual     func() interface{}
	}{
		{
			name:          "uri flag",
			flagName:      "uri",
			flagValue:     "qemu:///system",
			expectedValue: "qemu:///system",
			getActual:     func() interface{} { return libvirtcfg.URI },
		},
		{
			name:          "pool-name flag",
			flagName:      "pool-name",
			flagValue:     "test-pool",
			expectedValue: "test-pool",
			getActual:     func() interface{} { return libvirtcfg.PoolName },
		},
		{
			name:          "network-name flag",
			flagName:      "network-name",
			flagValue:     "test-network",
			expectedValue: "test-network",
			getActual:     func() interface{} { return libvirtcfg.NetworkName },
		},
		{
			name:          "vol-name flag",
			flagName:      "vol-name",
			flagValue:     "test-vol.qcow2",
			expectedValue: "test-vol.qcow2",
			getActual:     func() interface{} { return libvirtcfg.VolName },
		},
		{
			name:          "launch-security flag",
			flagName:      "launch-security",
			flagValue:     "s390-pv",
			expectedValue: "s390-pv",
			getActual:     func() interface{} { return libvirtcfg.LaunchSecurity },
		},
		{
			name:          "firmware flag",
			flagName:      "firmware",
			flagValue:     "/custom/path/to/firmware.fd",
			expectedValue: "/custom/path/to/firmware.fd",
			getActual:     func() interface{} { return libvirtcfg.Firmware },
		},
		{
			name:          "data-dir flag",
			flagName:      "data-dir",
			flagValue:     "/custom/data/dir",
			expectedValue: "/custom/data/dir",
			getActual:     func() interface{} { return libvirtcfg.DataDir },
		},
		{
			name:          "cpu flag",
			flagName:      "cpu",
			flagValue:     "4",
			expectedValue: uint(4),
			getActual:     func() interface{} { return libvirtcfg.CPU },
		},
		{
			name:          "memory flag",
			flagName:      "memory",
			flagValue:     "4096",
			expectedValue: uint(4096),
			getActual:     func() interface{} { return libvirtcfg.Memory },
		},
		{
			name:          "disable-cvm flag",
			flagName:      "disable-cvm",
			flagValue:     "false",
			expectedValue: false,
			getActual:     func() interface{} { return libvirtcfg.DisableCVM },
		},
		{
			name:          "root-disk-size flag",
			flagName:      "root-disk-size",
			flagValue:     "20",
			expectedValue: uint64(20),
			getActual:     func() interface{} { return libvirtcfg.RootDiskSize },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, flags := setupManagerTest(t)

			err := flags.Set(tc.flagName, tc.flagValue)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedValue, tc.getActual())
		})
	}
}

// TestManagerRootDiskSizeInvalidEnv is an integration-level guard that exercises the
// full ParseCmd path (setupManagerTest → ParseCmd → libvirtcfg global) with an invalid
// env var.  TestUint64WithEnv covers the util helper in isolation; this test verifies
// that the wiring in manager.go also degrades gracefully.
func TestManagerRootDiskSizeInvalidEnv(t *testing.T) {
	t.Setenv("LIBVIRT_ROOT_DISK_SIZE", "notanumber")
	_, _ = setupManagerTest(t)
	assert.Equal(t, defaultRootDiskSize, libvirtcfg.RootDiskSize,
		"invalid LIBVIRT_ROOT_DISK_SIZE env var must silently fall back to default (%d GiB)", defaultRootDiskSize)
}
