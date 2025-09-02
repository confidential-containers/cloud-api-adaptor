// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"

type Config struct {
	ApiKey            string
	Zone              string
	ServiceInstanceID string
	NetworkID         string
	ImageId           string
	SSHKey            string
	Memory            float64
	Processors        float64
	ProcessorType     string
	CAPublicKeyPath   string
	SystemType        string
	UsePublicIP       bool
	EnableSftp        bool
	pubKey            string
	privKey           string
	CloudUserName     string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ApiKey").(*Config)
}
