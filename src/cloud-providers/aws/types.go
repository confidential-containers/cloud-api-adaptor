// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type securityGroupIds []string

func (i *securityGroupIds) String() string {
	return strings.Join(*i, ", ")
}

func (i *securityGroupIds) Set(value string) error {
	*i = append(*i, strings.Split(value, ",")...)
	return nil
}

type instanceTypes []string

func (i *instanceTypes) String() string {
	return strings.Join(*i, ", ")
}

func (i *instanceTypes) Set(value string) error {
	if len(value) == 0 {
		*i = make(instanceTypes, 0)
	} else {
		*i = append(*i, strings.Split(value, ",")...)
	}
	return nil
}

type Config struct {
	AccessKeyID          string
	SecretKey            string
	Region               string
	LoginProfile         string
	LaunchTemplateName   string
	ImageID              string
	InstanceType         string
	KeyName              string
	SubnetID             string
	SecurityGroupIds     securityGroupIds
	UseLaunchTemplate    bool
	InstanceTypes        instanceTypes
	InstanceTypeSpecList []provider.InstanceTypeSpec
	Tags                 provider.KeyValueFlag
	UsePublicIP          bool
	RootVolumeSize       int
	RootDeviceName       string
	DisableCVM           bool
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "AccessKeyId", "SecretKey").(*Config)
}
