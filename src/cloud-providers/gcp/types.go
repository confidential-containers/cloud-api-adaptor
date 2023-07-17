// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type Config struct {
	GcpCredentials string
	ProjectId      string
	Zone           string
	ImageName      string
	MachineType    string
	Network        string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "GcpCredentials").(*Config)
}
