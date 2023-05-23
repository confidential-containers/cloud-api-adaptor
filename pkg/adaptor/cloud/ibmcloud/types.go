// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import "github.com/confidential-containers/cloud-api-adaptor/pkg/util"

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
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ApiKey").(*Config)
}
