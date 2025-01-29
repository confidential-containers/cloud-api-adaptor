// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var gcpcfg Config

type Manager struct{}

func init() {
	provider.AddCloudProvider("gcp", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	flags.StringVar(&gcpcfg.GcpCredentials, "gcp-credentials", "", "Google Application Credentials, defaults to `GCP_CREDENTIALS`")
	flags.StringVar(&gcpcfg.GcpProjectId, "gcp-project-id", "", "GCP Project ID")
	flags.StringVar(&gcpcfg.GcpZone, "gcp-zone", "", "GCP Zone")
	flags.StringVar(&gcpcfg.ImageId, "imageid", "", "Pod VM image id that is available at GCP Images. Usually a name like 'podvm-image'")
	flags.StringVar(&gcpcfg.MachineType, "machine-type", "e2-medium", "Pod VM Machine type")
	flags.StringVar(&gcpcfg.GcpNetworkId, "gcp-network", "default", "GCP Network ID for the VMs")
	flags.BoolVar(&gcpcfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
	flags.StringVar(&gcpcfg.DiskType, "disk-type", "pd-standard", "Any GCP disk type (pd-standard, pd-ssd, pd-balanced or pd-extreme)")
}

func (_ *Manager) LoadEnv() {
	provider.DefaultToEnv(&gcpcfg.GcpCredentials, "GCP_CREDENTIALS", "")
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&gcpcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &gcpcfg
}
