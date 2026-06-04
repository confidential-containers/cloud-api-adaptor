// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var kubevirtcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("kubevirt", &Manager{})
}

func (*Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	reg.StringWithEnv(&kubevirtcfg.serviceconfigfile, "serviceconfig", "", "SERVICECONFIG", "serviceconfig filepath")
}

func (*Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (*Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&kubevirtcfg)
}

func (*Manager) GetConfig() (config *Config) {
	return &kubevirtcfg
}
