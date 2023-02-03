// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"strings"

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

type Config struct {
	AccessKeyId        string
	SecretKey          string
	Region             string
	LoginProfile       string
	LaunchTemplateName string
	ImageId            string
	InstanceType       string
	SecurityGroupIds   securityGroupIds
	KeyName            string
	SubnetId           string
	UseLaunchTemplate  bool
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "AccessKeyId", "SecretKey").(*Config)
}
