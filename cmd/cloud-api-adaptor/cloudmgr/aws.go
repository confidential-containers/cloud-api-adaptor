//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/aws"
)

func init() {
	cloudTable["aws"] = &awsMgr{}
}

var awscfg aws.Config

type awsMgr struct{}

func (_ *awsMgr) ParseCmd(flags *flag.FlagSet) {

	flags.StringVar(&awscfg.AccessKeyId, "aws-access-key-id", "", "Access Key ID, defaults to `AWS_ACCESS_KEY_ID`")
	flags.StringVar(&awscfg.SecretKey, "aws-secret-key", "", "Secret Key, defaults to `AWS_SECRET_ACCESS_KEY`")
	flags.StringVar(&awscfg.Region, "aws-region", "", "Region")
	flags.StringVar(&awscfg.LoginProfile, "aws-profile", "test", "AWS Login Profile")
	flags.StringVar(&awscfg.LaunchTemplateName, "aws-lt-name", "kata", "AWS Launch Template Name")
	flags.BoolVar(&awscfg.UseLaunchTemplate, "use-lt", false, "Use EC2 Launch Template for the Pod VMs")
	flags.StringVar(&awscfg.ImageId, "imageid", "", "Pod VM ami id")
	flags.StringVar(&awscfg.InstanceType, "instance-type", "t3.small", "Pod VM instance type")
	flags.Var(&awscfg.SecurityGroupIds, "securitygroupids", "Security Group Ids to be used for the Pod VM, comma separated")
	flags.StringVar(&awscfg.KeyName, "keyname", "", "SSH Keypair name to be used with the Pod VM")
	flags.StringVar(&awscfg.SubnetId, "subnetid", "", "Subnet ID to be used for the Pod VMs")

}

func (_ *awsMgr) LoadEnv() {
	defaultToEnv(&awscfg.AccessKeyId, "AWS_ACCESS_KEY_ID")
	defaultToEnv(&awscfg.SecretKey, "AWS_SECRET_ACCESS_KEY")

}

func (_ *awsMgr) NewProvider() (cloud.Provider, error) {
	return aws.NewProvider(&awscfg)
}
