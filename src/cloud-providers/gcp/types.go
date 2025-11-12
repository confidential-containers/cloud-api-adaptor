// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type Config struct {
	GcpCredentials   string
	ProjectId        string
	Zone             string
	ImageName        string
	MachineType      string
	Network          string
	Subnetwork       string
	DiskType         string
	DisableCVM       bool
	ConfidentialType string
	RootVolumeSize   int
	Tags             provider.KeyValueFlag
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "GcpCredentials").(*Config)
}
