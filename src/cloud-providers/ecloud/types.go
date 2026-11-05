// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0
package ecloud

import (
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type securityGroupIDs []string

func (i *securityGroupIDs) String() string {
	return strings.Join(*i, ", ")
}

func (i *securityGroupIDs) Set(value string) error {
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
	Region               string // Region ID, e.g. cn-jiangsu-1
	ZoneID               string // Availability zone ID, e.g. cn-jiangsu-1a
	ImageID              string
	InstanceType         string // Flavor name, e.g. s1.large.2
	NetworkID            string // Private network (subnet) ID
	SecurityGroupIDs     securityGroupIDs
	KeyName              string
	UsePublicIP          bool
	PublicBandwidthSize  int
	SystemDiskSize       int
	SystemDiskType       string
	InstanceTypes        instanceTypes
	InstanceTypeSpecList []provider.InstanceTypeSpec
	Tags                 provider.KeyValueFlag
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "AccessKeyID", "SecretKey").(*Config)
}
