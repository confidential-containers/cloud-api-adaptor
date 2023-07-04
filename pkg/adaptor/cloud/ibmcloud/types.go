// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
)

type instanceProfiles []string

func (i *instanceProfiles) String() string {
	return strings.Join(*i, ", ")
}

func (i *instanceProfiles) Set(value string) error {
	if len(value) == 0 {
		*i = make(instanceProfiles, 0)
	} else {
		*i = append(*i, strings.Split(value, ",")...)
	}
	return nil
}

type Config struct {
	ApiKey                   string
	IAMProfileID             string
	CRTokenFileName          string
	IamServiceURL            string
	VpcServiceURL            string
	ResourceGroupID          string
	ProfileName              string
	ZoneName                 string
	ImageID                  string
	PrimarySubnetID          string
	PrimarySecurityGroupID   string
	SecondarySubnetID        string
	SecondarySecurityGroupID string
	KeyID                    string
	VpcID                    string
	InstanceProfiles         instanceProfiles
	InstanceProfileSpecList  []cloud.InstanceTypeSpec
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ApiKey").(*Config)
}
