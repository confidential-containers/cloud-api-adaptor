// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type Config struct {
	// Generic GCP
	GcpCredentials string
	GcpProjectId   string
	GcpZone        string
	// VPC Configuration
	GcpNetworkId string
	// CAA configuration
	ImageId     string
	MachineType string
	DisableCVM  bool
	DiskType    string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "GcpCredentials").(*Config)
}
