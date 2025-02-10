// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tunneler

import "strings"

type TunnelerConfigurator interface {
	Tunneler
	Configure(*NetworkConfig, *Config) error
}

type NetworkConfig struct {
	TunnelType          string
	HostInterface       string
	VXLAN               VXLANConfig
	ExternalNetViaPodVM bool
	PodSubnetCIDRs      SubnetCIDRs
}

type VXLANConfig struct {
	Port  int
	MinID int
}

type SubnetCIDRs []string

func (i *SubnetCIDRs) String() string {
	return strings.Join(*i, ", ")
}

func (i *SubnetCIDRs) Set(value string) error {
	parts := strings.Split(value, ",")

	for _, part := range parts {
		*i = append(*i, strings.TrimSpace(part))
	}
	return nil
}
