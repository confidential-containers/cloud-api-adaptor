// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var byomcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("byom", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	flags.Var(&byomcfg.VMPoolIPs, "vm-pool-ips", "Comma-separated list of IP addresses for pre-created VMs")
	flags.StringVar(&byomcfg.SSHUserName, "ssh-username", "peerpod", "SSH username for VM access")
	flags.StringVar(&byomcfg.SSHPubKeyPath, "ssh-pub-key", "/root/.ssh/id_rsa.pub", "SSH public key file path")
	flags.StringVar(&byomcfg.SSHPrivKeyPath, "ssh-priv-key", "/root/.ssh/id_rsa", "SSH private key file path")
	flags.IntVar(&byomcfg.SSHTimeout, "ssh-timeout", 30, "SSH connection timeout in seconds")
	flags.BoolVar(&byomcfg.SSHInsecureIgnoreHostKey, "ssh-insecure-ignore-host-key", false, "Skip SSH host key verification (insecure, for debugging only)")

	// Pool management configuration
	flags.StringVar(&byomcfg.PoolNamespace, "pool-namespace", "", "Namespace for ConfigMap storage (default: auto-detect from running pod)")
	flags.StringVar(&byomcfg.PoolConfigMapName, "pool-configmap-name", "", "ConfigMap name for state storage (default: byom-ip-pool-state)")
}

func (_ *Manager) LoadEnv() {
	provider.DefaultToEnv(&byomcfg.SSHUserName, "BYOM_SSH_USERNAME", "peerpod")
	provider.DefaultToEnv(&byomcfg.SSHPubKeyPath, "BYOM_SSH_PUB_KEY_PATH", "/root/.ssh/id_rsa.pub")
	provider.DefaultToEnv(&byomcfg.SSHPrivKeyPath, "BYOM_SSH_PRIV_KEY_PATH", "/root/.ssh/id_rsa")

	// Pool management configuration
	provider.DefaultToEnv(&byomcfg.PoolNamespace, "BYOM_POOL_NAMESPACE", "")
	provider.DefaultToEnv(&byomcfg.PoolConfigMapName, "BYOM_POOL_CONFIGMAP_NAME", "")
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&byomcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &byomcfg
}
