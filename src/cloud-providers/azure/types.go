// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
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
	SubscriptionID       string
	ClientID             string
	ClientSecret         string
	TenantID             string
	ResourceGroupName    string
	Zone                 string
	Region               string
	SubnetID             string
	SecurityGroupName    string
	SecurityGroupID      string
	Size                 string
	ImageID              string
	SSHKeyPath           string
	SSHUserName          string
	DisableCVM           bool
	InstanceSizes        instanceSizes
	InstanceSizeSpecList []provider.InstanceTypeSpec
	Tags                 provider.KeyValueFlag
	DisableCloudConfig   bool
	// Disabled by default, we want to do measured boot.
	// Secure boot brings no additional security.
	EnableSecureBoot bool
	UsePublicIP      bool
	RootVolumeSize   int
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ClientId", "TenantId", "ClientSecret").(*Config)
}
