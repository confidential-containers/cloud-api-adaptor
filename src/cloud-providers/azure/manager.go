// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var azurecfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("azure", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&azurecfg.ClientId, "clientid", "", "AZURE_CLIENT_ID", "Client Id", provider.Secret())
	reg.StringWithEnv(&azurecfg.ClientSecret, "secret", "", "AZURE_CLIENT_SECRET", "Client Secret", provider.Secret())
	reg.StringWithEnv(&azurecfg.TenantId, "tenantid", "", "AZURE_TENANT_ID", "Tenant Id", provider.Secret())
	reg.StringWithEnv(&azurecfg.SubscriptionId, "subscriptionid", "", "AZURE_SUBSCRIPTION_ID", "Subscription ID", provider.Required())
	reg.StringWithEnv(&azurecfg.Region, "region", "", "AZURE_REGION", "Region", provider.Required())
	reg.StringWithEnv(&azurecfg.ResourceGroupName, "resourcegroup", "", "AZURE_RESOURCE_GROUP", "Resource Group", provider.Required())
	reg.StringWithEnv(&azurecfg.Size, "instance-size", "Standard_DC2as_v5", "AZURE_INSTANCE_SIZE", "Instance size")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&azurecfg.Zone, "zone", "", "", "Zone")
	reg.StringWithEnv(&azurecfg.SubnetId, "subnetid", "", "AZURE_SUBNET_ID", "Network Subnet Id", provider.Required())
	reg.StringWithEnv(&azurecfg.SecurityGroupId, "securitygroupid", "", "AZURE_NSG_ID", "Security Group Id")
	reg.StringWithEnv(&azurecfg.ImageId, "imageid", "", "AZURE_IMAGE_ID", "Image Id", provider.Required())
	reg.StringWithEnv(&azurecfg.SSHKeyPath, "ssh-key-path", "", "", "Path to SSH public key")
	reg.StringWithEnv(&azurecfg.SSHUserName, "ssh-username", "peerpod", "SSH_USERNAME", "SSH User Name")
	reg.BoolWithEnv(&azurecfg.DisableCVM, "disable-cvm", false, "DISABLECVM", "Use non-CVMs for peer pods")
	reg.BoolWithEnv(&azurecfg.EnableSecureBoot, "enable-secure-boot", false, "ENABLE_SECURE_BOOT", "Enable secure boot for the VMs")
	reg.BoolWithEnv(&azurecfg.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Assign public IP to the PoD VM and use to connect to kata-agent")
	reg.IntWithEnv(&azurecfg.RootVolumeSize, "root-volume-size", 0, "ROOT_VOLUME_SIZE", "Root volume size in GB. Default is 0, which implies the default image disk size")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&azurecfg.InstanceSizes, "instance-sizes", "", "AZURE_INSTANCE_SIZES", "Instance sizes to be used for the Pod VMs, comma separated")
	reg.CustomTypeWithEnv(&azurecfg.Tags, "tags", "", "TAGS", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&azurecfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &azurecfg
}
