// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var awscfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("aws", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&awscfg.AccessKeyID, "aws-access-key-id", "", "Access Key ID, defaults to `AWS_ACCESS_KEY_ID`")
	flags.StringVar(&awscfg.SecretKey, "aws-secret-key", "", "Secret Key, defaults to `AWS_SECRET_ACCESS_KEY`")
	flags.StringVar(&awscfg.Region, "aws-region", "", "Region")
	flags.StringVar(&awscfg.LoginProfile, "aws-profile", "", "AWS Login Profile")
	flags.StringVar(&awscfg.LaunchTemplateName, "aws-lt-name", "kata", "AWS Launch Template Name")
	flags.BoolVar(&awscfg.UseLaunchTemplate, "use-lt", false, "Use EC2 Launch Template for the Pod VMs")
	flags.StringVar(&awscfg.ImageID, "imageid", "", "Pod VM ami id")
	flags.StringVar(&awscfg.InstanceType, "instance-type", "m6a.large", "Pod VM instance type")
	flags.Var(&awscfg.SecurityGroupIds, "securitygroupids", "Security Group Ids to be used for the Pod VM, comma separated")
	flags.StringVar(&awscfg.KeyName, "keyname", "", "SSH Keypair name to be used with the Pod VM")
	flags.StringVar(&awscfg.SubnetID, "subnetid", "", "Subnet ID to be used for the Pod VMs")
	// Add a List parameter to indicate differet type of instance types to be used for the Pod VMs
	flags.Var(&awscfg.InstanceTypes, "instance-types", "Instance types to be used for the Pod VMs, comma separated")
	// Add a key value list parameter to indicate custom tags to be used for the Pod VMs
	flags.Var(&awscfg.Tags, "tags", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
	flags.BoolVar(&awscfg.UsePublicIP, "use-public-ip", false, "Use Public IP for connecting to the kata-agent inside the Pod VM")
	// Add a parameter to indicate the root volume size for the Pod VMs
	// Default is 30GiBs for free tier. Hence use it as default
	flags.IntVar(&awscfg.RootVolumeSize, "root-volume-size", 30, "Root volume size (in GiB) for the Pod VMs")
	flags.BoolVar(&awscfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")

}

func (*Manager) LoadEnv() {
	provider.DefaultToEnv(&awscfg.AccessKeyID, "AWS_ACCESS_KEY_ID", "")
	provider.DefaultToEnv(&awscfg.SecretKey, "AWS_SECRET_ACCESS_KEY", "")
	provider.DefaultToEnv(&awscfg.InstanceType, "PODVM_INSTANCE_TYPE", "m6a.large")
}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&awscfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &awscfg
}
