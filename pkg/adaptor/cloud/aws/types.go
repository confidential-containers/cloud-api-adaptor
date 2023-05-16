// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
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
	AccessKeyId          string
	SecretKey            string
	Region               string
	LoginProfile         string
	LaunchTemplateName   string
	ImageId              string
	InstanceType         string
	KeyName              string
	SubnetId             string
	SecurityGroupIds     securityGroupIds
	UseLaunchTemplate    bool
	InstanceTypes        instanceTypes
	InstanceTypeSpecList []cloud.InstanceTypeSpec
	Tags                 cloud.KeyValueFlag
	UsePublicIP          bool
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "AccessKeyId", "SecretKey").(*Config)
}
