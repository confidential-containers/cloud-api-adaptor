// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var azurecfg Config

type Manager struct{}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	flags.StringVar(&azurecfg.ClientId, "clientid", "", "Client Id, defaults to `AZURE_CLIENT_ID`")
	flags.StringVar(&azurecfg.ClientSecret, "secret", "", "Client Secret, defaults to `AZURE_CLIENT_SECRET`")
	flags.StringVar(&azurecfg.TenantId, "tenantid", "", "Tenant Id, defaults to `AZURE_TENANT_ID`")
	flags.StringVar(&azurecfg.ResourceGroupName, "resourcegroup", "", "Resource Group")
	flags.StringVar(&azurecfg.Zone, "zone", "", "Zone")
	flags.StringVar(&azurecfg.Region, "region", "", "Region")
	flags.StringVar(&azurecfg.SubnetId, "subnetid", "", "Network Subnet Id")
	flags.StringVar(&azurecfg.SecurityGroupId, "securitygroupid", "", "Security Group Id")
	flags.StringVar(&azurecfg.Size, "instance-size", "", "Instance size")
	flags.StringVar(&azurecfg.ImageId, "imageid", "", "Image Id")
	flags.StringVar(&azurecfg.SubscriptionId, "subscriptionid", "", "Subscription ID")
	flags.StringVar(&azurecfg.SSHKeyPath, "ssh-key-path", "$HOME/.ssh/id_rsa.pub", "Path to SSH public key")
	flags.StringVar(&azurecfg.SSHUserName, "ssh-username", "peerpod", "SSH User Name")
	flags.BoolVar(&azurecfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
}

func (_ *Manager) LoadEnv() {
	cloud.DefaultToEnv(&azurecfg.ClientId, "AZURE_CLIENT_ID", "")
	cloud.DefaultToEnv(&azurecfg.ClientSecret, "AZURE_CLIENT_SECRET", "")
	cloud.DefaultToEnv(&azurecfg.TenantId, "AZURE_TENANT_ID", "")
	cloud.DefaultToEnv(&azurecfg.SubscriptionId, "AZURE_SUBSCRIPTION_ID", "")
	cloud.DefaultToEnv(&azurecfg.Region, "AZURE_REGION", "")
	cloud.DefaultToEnv(&azurecfg.ResourceGroupName, "AZURE_RESOURCE_GROUP", "")
}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&azurecfg)
}
