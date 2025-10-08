// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"flag"
	"log"
	"os"

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
	flags.Var(&byomcfg.VMPoolIPs, "vm-pool-ips", "Comma-separated list of IP addresses for pre-created VMs")
	flags.IntVar(&maxRangeIPs, "max-range-ips", 100, "Maximum number of IPs allowed in a range")
	flags.StringVar(&byomcfg.SSHUserName, "ssh-username", "peerpod", "SSH username for VM access")
	flags.StringVar(&byomcfg.SSHPubKeyPath, "ssh-pub-key", "/root/.ssh/id_rsa.pub", "SSH public key file path")
	flags.StringVar(&byomcfg.SSHPrivKeyPath, "ssh-priv-key", "/root/.ssh/id_rsa", "SSH private key file path")
	flags.IntVar(&byomcfg.SSHTimeout, "ssh-timeout", 30, "SSH connection timeout in seconds")
	flags.StringVar(&byomcfg.SSHHostKeyAllowlistDir, "ssh-host-key-allowlist-dir", "", "Directory containing allowed SSH host key files (enables allowlist mode if set)")

	// Pool management configuration
	flags.StringVar(&byomcfg.PoolNamespace, "pool-namespace", "", "Namespace for ConfigMap storage (default: auto-detect from running pod)")
	flags.StringVar(&byomcfg.PoolConfigMapName, "pool-configmap-name", "byom-ip-pool-state", "ConfigMap name for state storage")
}

func (m *Manager) LoadEnv() {
	// VM Pool IPs (custom handling since it's not a string type)
	if vmPoolIPsEnv := os.Getenv("VM_POOL_IPS"); vmPoolIPsEnv != "" {
		if err := byomcfg.VMPoolIPs.Set(vmPoolIPsEnv); err != nil {
			log.Printf("Warning: failed to parse VM_POOL_IPS environment variable: %v", err)
		}
	}

	provider.DefaultToEnv(&byomcfg.SSHUserName, "SSH_USERNAME", "peerpod")
	provider.DefaultToEnv(&byomcfg.SSHPubKeyPath, "SSH_PUB_KEY_PATH", "/root/.ssh/id_rsa.pub")
	provider.DefaultToEnv(&byomcfg.SSHPrivKeyPath, "SSH_PRIV_KEY_PATH", "/root/.ssh/id_rsa")

	// Pool management configuration
	provider.DefaultToEnv(&byomcfg.PoolNamespace, "POOL_NAMESPACE", "")
	provider.DefaultToEnv(&byomcfg.PoolConfigMapName, "POOL_CONFIGMAP_NAME", "byom-ip-pool-state")
}

func (m *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&byomcfg)
}

func (m *Manager) GetConfig() (config *Config) {
	return &byomcfg
}
