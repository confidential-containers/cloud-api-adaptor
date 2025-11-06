// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var (
	byomcfg     Config
	maxRangeIPs int // Maximum IPs in an IP range
)

type Manager struct{}

func init() {
	provider.AddCloudProvider("byom", &Manager{})
}

func (m *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.CustomTypeWithEnv(&byomcfg.VMPoolIPs, "vm-pool-ips", "", "VM_POOL_IPS", "Comma-separated list of IP addresses for pre-created VMs")
	reg.StringWithEnv(&byomcfg.SSHUserName, "ssh-username", "peerpod", "SSH_USERNAME", "SSH username for VM access")
	reg.StringWithEnv(&byomcfg.SSHPubKeyPath, "ssh-pub-key", "/root/.ssh/id_rsa.pub", "SSH_PUB_KEY_PATH", "SSH public key file path")
	reg.StringWithEnv(&byomcfg.SSHPrivKeyPath, "ssh-priv-key", "/root/.ssh/id_rsa", "SSH_PRIV_KEY_PATH", "SSH private key file path")
	reg.StringWithEnv(&byomcfg.PoolNamespace, "pool-namespace", "", "POOL_NAMESPACE", "Namespace for ConfigMap storage (default: auto-detect from running pod)")
	reg.StringWithEnv(&byomcfg.PoolConfigMapName, "pool-configmap-name", "byom-ip-pool-state", "POOL_CONFIGMAP_NAME", "ConfigMap name for state storage")
	reg.IntWithEnv(&maxRangeIPs, "max-range-ips", 100, "MAX_RANGE_IPS", "Maximum number of IPs allowed in a range")
	reg.IntWithEnv(&byomcfg.SSHTimeout, "ssh-timeout", 30, "SSH_TIMEOUT", "SSH connection timeout in seconds")
	reg.StringWithEnv(&byomcfg.SSHHostKeyAllowlistDir, "ssh-host-key-allowlist-dir", "", "SSH_HOST_KEY_ALLOWLIST_DIR", "Directory containing allowed SSH host key files (enables allowlist mode if set)")
}

func (m *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (m *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&byomcfg)
}

func (m *Manager) GetConfig() (config *Config) {
	return &byomcfg
}
