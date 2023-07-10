// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
)

type instanceSizes []string

func (i *instanceSizes) String() string {
	return strings.Join(*i, ", ")
}

func (i *instanceSizes) Set(value string) error {
	if len(value) == 0 {
		*i = make(instanceSizes, 0)
	} else {
		*i = append(*i, strings.Split(value, ",")...)
	}
	return nil
}

type Config struct {
	SubscriptionId       string
	ClientId             string
	ClientSecret         string
	TenantId             string
	ResourceGroupName    string
	Zone                 string
	Region               string
	SubnetId             string
	SecurityGroupName    string
	SecurityGroupId      string
	Size                 string
	ImageId              string
	SSHKeyPath           string
	SSHUserName          string
	DisableCVM           bool
	InstanceSizes        instanceSizes
	InstanceSizeSpecList []cloud.InstanceTypeSpec
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ClientId", "TenantId", "ClientSecret").(*Config)
}
