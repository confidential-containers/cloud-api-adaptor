// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var openstackcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("openstack", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	reg.StringWithEnv(&openstackcfg.ServerPrefix, "server-prefix", "", "OPENSTACK_SERVER_PREFIX", "server-prefix")
	reg.StringWithEnv(&openstackcfg.ImageID, "imageID", "", "OPENSTACK_IMAGE_ID", "openstack-image-id")
	reg.StringWithEnv(&openstackcfg.FlavorID, "flavorID", "", "OPENSTACK_FLAVOR_ID", "openstack-flavor-id")
	reg.CustomTypeWithEnv(&openstackcfg.NetworkIDs, "networkID", "", "OPENSTACK_NETWORK_ID", "openstack-network-id")
	reg.CustomTypeWithEnv(&openstackcfg.SecurityGroups, "security-group", "", "OPENSTACK_SECURITY_GROUP", "openstack-security-group")
	reg.StringWithEnv(&openstackcfg.FloatingIpNetworkID, "floating-ip-networkID", "", "OPENSTACK_FLOATING_IP_NETWORK_ID", "openstack-floating-ip-network-id")

	reg.StringWithEnv(&openstackcfg.Username, "openstack-username", "", "OPENSTACK_USERNAME", "openstack-username")
	reg.StringWithEnv(&openstackcfg.Password, "openstack-password", "", "OPENSTACK_PASSWORD", "openstack-password")
	reg.StringWithEnv(&openstackcfg.Region, "openstack-region", "", "OPENSTACK_REGION", "openstack-region")
	reg.StringWithEnv(&openstackcfg.TenantName, "openstack-tenant-name", "", "OPENSTACK_TENANT_NAME", "openstack-tenant-name")
	reg.StringWithEnv(&openstackcfg.DomainName, "openstack-domain-name", "", "OPENSTACK_DOMAIN_NAME", "openstack-domain-name")
	reg.StringWithEnv(&openstackcfg.IdentityEndpoint, "openstack-identity-endpoint", "", "OPENSTACK_IDENTITY_ENDPOINT", "openstack-identity-endpoint")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&openstackcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &openstackcfg
}
