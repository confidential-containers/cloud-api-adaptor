// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"flag"
	"time"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var openstackcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("openstack", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	reg.StringWithEnv(&openstackcfg.ServerPrefix, "server-prefix", "", "OPENSTACK_SERVER_PREFIX", "server-prefix")
	reg.StringWithEnv(&openstackcfg.ImageID, "imageID", "", "OPENSTACK_IMAGE_ID", "openstack-image-id", provider.Required())
	reg.StringWithEnv(&openstackcfg.FlavorID, "flavorID", "", "OPENSTACK_FLAVOR_ID", "openstack-flavor-id", provider.Required())
	reg.CustomTypeWithEnv(&openstackcfg.NetworkIDs, "networkID", "", "OPENSTACK_NETWORK_ID", "openstack-network-id", provider.Required())
	reg.CustomTypeWithEnv(&openstackcfg.SecurityGroups, "security-group", "", "OPENSTACK_SECURITY_GROUP", "openstack-security-group", provider.Required())
	reg.StringWithEnv(&openstackcfg.FloatingIPNetworkID, "floating-ip-networkID", "", "OPENSTACK_FLOATING_IP_NETWORK_ID", "openstack-floating-ip-network-id")

	reg.StringWithEnv(&openstackcfg.Username, "openstack-username", "", "OPENSTACK_USERNAME", "openstack-username", provider.Secret())
	reg.StringWithEnv(&openstackcfg.Password, "openstack-password", "", "OPENSTACK_PASSWORD", "openstack-password", provider.Secret())
	reg.StringWithEnv(&openstackcfg.Region, "openstack-region", "", "OPENSTACK_REGION", "openstack-region", provider.Required())
	reg.StringWithEnv(&openstackcfg.TenantName, "openstack-tenant-name", "", "OPENSTACK_TENANT_NAME", "openstack-tenant-name", provider.Secret())
	reg.StringWithEnv(&openstackcfg.DomainName, "openstack-domain-name", "", "OPENSTACK_DOMAIN_NAME", "openstack-domain-name", provider.Required())
	reg.StringWithEnv(&openstackcfg.IdentityEndpoint, "openstack-identity-endpoint", "", "OPENSTACK_IDENTITY_ENDPOINT", "openstack-identity-endpoint", provider.Required())

	reg.IntWithEnv(&openstackcfg.VMStatusMaxRetries, "openstack-vm-status-max-retries", 60, "OPENSTACK_VM_STATUS_MAX_RETRIES", "Maximum number of retries for status checks until the VM becomes ACTIVE")
	reg.DurationWithEnv(&openstackcfg.VMStatusRetryInterval, "openstack-vm-status-retry-interval", 3*time.Second, "OPENSTACK_VM_STATUS_RETRY_INTERVAL", "Interval between status check retries until the VM becomes ACTIVE")
}

func (*Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&openstackcfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &openstackcfg
}
