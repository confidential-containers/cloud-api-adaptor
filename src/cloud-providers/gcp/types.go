// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type machineTypes []string

func (m *machineTypes) String() string {
	return strings.Join(*m, ", ")
}

func (m *machineTypes) Set(value string) error {
	if len(value) == 0 {
		*m = make(machineTypes, 0)
	} else {
		*m = append(*m, strings.Split(value, ",")...)
	}
	return nil
}

type Config struct {
	GcpCredentials      string
	ProjectId           string
	Zone                string
	ImageName           string
	MachineType         string
	Network             string
	Subnetwork          string
	DiskType            string
	DisableCVM          bool
	ConfidentialType    string
	RootVolumeSize      int
	Tags                provider.KeyValueFlag
	UsePublicIP         bool
	MachineTypes        machineTypes
	MachineTypeSpecList []provider.InstanceTypeSpec
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "GcpCredentials").(*Config)
}
