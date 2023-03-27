// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import "github.com/confidential-containers/cloud-api-adaptor/pkg/util"

type Config struct {
	ApiKey            string
	Zone              string
	ServiceInstanceID string
	NetworkID         string
	ImageID           string
	SSHKey            string
	Memory            float64
	Processors        float64
	ProcessorType     string
	SystemType        string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ApiKey").(*Config)
}
