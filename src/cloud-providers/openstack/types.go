// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type networkIds []string

func (i *networkIds) String() string {
	return strings.Join(*i, ", ")
}

func (i *networkIds) Set(value string) error {
	if value != "" {
		*i = append(*i, strings.Split(value, ",")...)
	}
	return nil
}

type securityGroups []string

func (i *securityGroups) String() string {
	return strings.Join(*i, ", ")
}

func (i *securityGroups) Set(value string) error {
	if value != "" {
		*i = append(*i, strings.Split(value, ",")...)
	}
	return nil
}

type Config struct {
	IdentityEndpoint    string
	Username            string
	TenantName          string
	Password            string
	DomainName          string
	Region              string
	ServerPrefix        string
	ImageID             string
	FlavorID            string
	NetworkIDs          networkIds
	SecurityGroups      securityGroups
	FloatingIpNetworkID string
}

// Redact sensitive information from the config
func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "Username", "Password", "TenantName").(*Config)
}
