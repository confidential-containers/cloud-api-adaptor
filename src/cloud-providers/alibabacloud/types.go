// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0
package alibabacloud

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
	ImageID              string
	InstanceType         string
	KeyName              string
	VpcID                string
	VswitchID            string
	SecurityGroupIds     securityGroupIds
	InstanceTypes        instanceTypes
	InstanceTypeSpecList []provider.InstanceTypeSpec
	Tags                 provider.KeyValueFlag
	UsePublicIP          bool
	SystemDiskSize       int
	DisableCVM           bool
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "AccessKeyId", "SecretKey").(*Config)
}
