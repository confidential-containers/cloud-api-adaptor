// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
)

var awscfg Config

type Manager struct{}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&awscfg.AccessKeyId, "aws-access-key-id", "", "Access Key ID, defaults to `AWS_ACCESS_KEY_ID`")
	flags.StringVar(&awscfg.SecretKey, "aws-secret-key", "", "Secret Key, defaults to `AWS_SECRET_ACCESS_KEY`")
	flags.StringVar(&awscfg.Region, "aws-region", "", "Region")
	flags.StringVar(&awscfg.LoginProfile, "aws-profile", "", "AWS Login Profile")
	flags.StringVar(&awscfg.LaunchTemplateName, "aws-lt-name", "kata", "AWS Launch Template Name")
	flags.BoolVar(&awscfg.UseLaunchTemplate, "use-lt", false, "Use EC2 Launch Template for the Pod VMs")
	flags.StringVar(&awscfg.ImageId, "imageid", "", "Pod VM ami id")
	flags.StringVar(&awscfg.InstanceType, "instance-type", "t3.small", "Pod VM instance type")
	flags.Var(&awscfg.SecurityGroupIds, "securitygroupids", "Security Group Ids to be used for the Pod VM, comma separated")
	flags.StringVar(&awscfg.KeyName, "keyname", "", "SSH Keypair name to be used with the Pod VM")
	flags.StringVar(&awscfg.SubnetId, "subnetid", "", "Subnet ID to be used for the Pod VMs")
	// Add a List parameter to indicate differet type of instance types to be used for the Pod VMs
	flags.Var(&awscfg.InstanceTypes, "instance-types", "Instance types to be used for the Pod VMs, comma separated")
	// Add a key value list parameter to indicate custom tags to be used for the Pod VMs
	flags.Var(&awscfg.Tags, "tags", "Custom tags (key=value pairs) to be used for the Pod VMs, comma separated")
	flags.BoolVar(&awscfg.UsePublicIP, "use-public-ip", false, "Use Public IP for connecting to the kata-agent inside the Pod VM")
	// Add a parameter to indicate the root volume size for the Pod VMs
	// Default is 30GiBs for free tier. Hence use it as default
	flags.IntVar(&awscfg.RootVolumeSize, "root-volume-size", 30, "Root volume size (in GiB) for the Pod VMs")
	// Setting disable-cvm to true as there are still some rough edges with CVMs.
	// Once the issues are fixed, we can set it to false by default
	flags.BoolVar(&awscfg.DisableCVM, "disable-cvm", true, "Use non-CVMs for peer pods")
	// Add a flag to disable cloud config and use userdata via metadata service
	flags.BoolVar(&awscfg.DisableCloudConfig, "disable-cloud-config", false, "Disable cloud config and use userdata via metadata service")

}

func (_ *Manager) LoadEnv() {
	cloud.DefaultToEnv(&awscfg.AccessKeyId, "AWS_ACCESS_KEY_ID", "")
	cloud.DefaultToEnv(&awscfg.SecretKey, "AWS_SECRET_ACCESS_KEY", "")

}

func (_ *Manager) NewProvider() (cloud.Provider, error) {
	return NewProvider(&awscfg)
}
