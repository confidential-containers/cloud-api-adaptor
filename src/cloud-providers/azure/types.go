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
	SSHUserName          string
	SSHPubKeyPath        string
	SSHPrivKeyPath       string
	SSHPubKey            string
	SSHPrivKey           string
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
	EnableSftp       bool
	// New VM Pool configuration
	VMPoolType          string
	VMPoolPodRegex      string
	VMPoolInstanceTypes instanceSizes
	VMPoolIPs           []string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ClientId", "TenantId", "ClientSecret").(*Config)
}
