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
	flags.StringVar(&gcpcfg.ProjectId, "gcp-project-id", "", "GCP Project ID")
	flags.StringVar(&gcpcfg.Zone, "zone", "", "Zone")
	flags.StringVar(&gcpcfg.ImageName, "image-name", "", "Pod VM image name")
	flags.StringVar(&gcpcfg.MachineType, "machine-type", "e2-medium", "Pod VM instance type")
	flags.StringVar(&gcpcfg.Network, "network", "", "Network ID to be used for the Pod VMs")
	flags.StringVar(&gcpcfg.Subnetwork, "subnetwork", "", "Subnetwork ID to be used for the Pod VMs (required for custom subnet mode networks)")
	flags.StringVar(&gcpcfg.DiskType, "disk-type", "pd-standard", "Any GCP disk type (pd-standard, pd-ssd, pd-balanced or pd-extreme)")
	flags.BoolVar(&gcpcfg.DisableCVM, "disable-cvm", false, "Use non-CVMs for peer pods")
	flags.StringVar(&gcpcfg.ConfidentialType, "confidential-type", "", "Used when DisableCVM=false. i.e: TDX, SEV or SEV_SNP. Check if the machine type is compatible.")
	flags.IntVar(&gcpcfg.RootVolumeSize, "root-volume-size", 10, "Root volume size (in GiB) for the Pod VMs")
	flags.Var(&gcpcfg.Tags, "tags", "List of tags to be added to the Pod VMs. Tags must already exist in the GCP project. Format: key1=value1,key2=value2")
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
