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
	flags.StringVar(&azurecfg.ClientId, "clientid", "", "Client Id, defaults to `AZURE_CLIENT_ID`")
	flags.StringVar(&azurecfg.ClientSecret, "secret", "", "Client Secret, defaults to `AZURE_CLIENT_SECRET`")
	flags.StringVar(&azurecfg.TenantId, "tenantid", "", "Tenant Id, defaults to `AZURE_TENANT_ID`")
	flags.StringVar(&azurecfg.ResourceGroupName, "resourcegroup", "", "Resource Group")
	flags.StringVar(&azurecfg.Zone, "zone", "", "Zone")
	flags.StringVar(&azurecfg.Region, "region", "", "Region")
	flags.StringVar(&azurecfg.SubnetId, "subnetid", "", "Network Subnet Id")
	flags.StringVar(&azurecfg.SecurityGroupId, "securitygroupid", "", "Security Group Id")
	flags.StringVar(&azurecfg.Size, "instance-size", "Standard_DC2as_v5", "Instance size")
	flags.StringVar(&azurecfg.ImageId, "imageid", "", "Image Id")
	flags.StringVar(&azurecfg.SubscriptionId, "subscriptionid", "", "Subscription ID")
	flags.StringVar(&azurecfg.SSHUserName, "ssh-username", "peerpod", "SSH User Name")
	flags.StringVar(&azurecfg.SSHPubKeyPath, "ssh-pub-key", "/root/.ssh/id_rsa.pub", "SSH public key file path")
	flags.StringVar(&azurecfg.SSHPrivKeyPath, "ssh-priv-key", "/root/.ssh/id_rsa", "SSH private key file path (required for SFTP if not auto-generated)")
	flags.BoolVar(&azurecfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
	// Add a List parameter to indicate different types of instance sizes to be used for the Pod VMs
	flags.Var(&azurecfg.InstanceSizes, "instance-sizes", "Instance sizes to be used for the Pod VMs, comma separated")
	// Add a key value list parameter to indicate custom tags to be used for the Pod VMs
	flags.Var(&azurecfg.Tags, "tags", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
	flags.BoolVar(&azurecfg.EnableSecureBoot, "enable-secure-boot", false, "Enable secure boot for the VMs")
	flags.BoolVar(&azurecfg.UsePublicIP, "use-public-ip", false, "Assign public IP to the PoD VM and use to connect to kata-agent")
	flags.IntVar(&azurecfg.RootVolumeSize, "root-volume-size", 0, "Root volume size in GB. Default is 0, which implies the default image disk size")
	flags.BoolVar(&azurecfg.EnableSftp, "enable-sftp", false, "Enable SFTP-based user-data transfer")
	// VM Pool configuration
	flags.StringVar(&azurecfg.VMPoolType, "vm-pool-type", "disabled", "VM pool type: disabled, global, podregex, instancetypes")
	flags.StringVar(&azurecfg.VMPoolPodRegex, "vm-pool-pod-regex", "", "Regex pattern for pod names (for podregex mode)")
	flags.Var(&azurecfg.VMPoolInstanceTypes, "vm-pool-instance-types", "Instance types to use VM pool for (for instancetypes mode), comma separated")
	flags.Var((*preCreatedIPsFlag)(&azurecfg.VMPoolIPs), "vm-pool-ips", "Comma-separated list of IP addresses for pre-created VMs")
}

func (_ *Manager) LoadEnv() {
	provider.DefaultToEnv(&azurecfg.ClientId, "AZURE_CLIENT_ID", "")
	provider.DefaultToEnv(&azurecfg.ClientSecret, "AZURE_CLIENT_SECRET", "")
	provider.DefaultToEnv(&azurecfg.TenantId, "AZURE_TENANT_ID", "")
	provider.DefaultToEnv(&azurecfg.SubscriptionId, "AZURE_SUBSCRIPTION_ID", "")
	provider.DefaultToEnv(&azurecfg.Region, "AZURE_REGION", "")
	provider.DefaultToEnv(&azurecfg.ResourceGroupName, "AZURE_RESOURCE_GROUP", "")
	provider.DefaultToEnv(&azurecfg.Size, "AZURE_INSTANCE_SIZE", "Standard_DC2as_v5")
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&azurecfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &azurecfg
}
