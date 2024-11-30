// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tunneler

type TunnelerConfigurator interface {
	Tunneler
	Configure(*NetworkConfig, *Config) error
}

type NetworkConfig struct {
	TunnelType    string
	HostInterface string
	VXLAN         VXLANConfig
}

type VXLANConfig struct {
	Port  int
	MinID int
}
