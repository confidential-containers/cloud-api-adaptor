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
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&gcpcfg.GcpCredentials, "gcp-credentials", "", "GCP_CREDENTIALS", "Google Application Credentials")
	reg.StringWithEnv(&gcpcfg.ProjectId, "gcp-project-id", "", "GCP_PROJECT_ID", "GCP Project ID")
	reg.StringWithEnv(&gcpcfg.Zone, "zone", "", "GCP_ZONE", "Zone")
	reg.StringWithEnv(&gcpcfg.ImageName, "image-name", "", "PODVM_IMAGE_NAME", "Pod VM image name")
	reg.StringWithEnv(&gcpcfg.MachineType, "machine-type", "e2-medium", "GCP_MACHINE_TYPE", "Pod VM instance type")
	reg.StringWithEnv(&gcpcfg.Network, "network", "", "GCP_NETWORK", "Network ID to be used for the Pod VMs")
	reg.StringWithEnv(&gcpcfg.Subnetwork, "subnetwork", "", "GCP_SUBNETWORK", "Subnetwork ID to be used for the Pod VMs (required for custom subnet mode networks)")
	reg.StringWithEnv(&gcpcfg.DiskType, "disk-type", "pd-standard", "GCP_DISK_TYPE", "Any GCP disk type (pd-standard, pd-ssd, pd-balanced or pd-extreme)")
	reg.BoolWithEnv(&gcpcfg.DisableCVM, "disable-cvm", false, "DISABLECVM", "Use non-CVMs for peer pods")
	reg.StringWithEnv(&gcpcfg.ConfidentialType, "confidential-type", "", "GCP_CONFIDENTIAL_TYPE", "Used when DisableCVM=false. i.e: TDX, SEV or SEV_SNP. Check if the machine type is compatible.")
	reg.IntWithEnv(&gcpcfg.RootVolumeSize, "root-volume-size", 10, "ROOT_VOLUME_SIZE", "Root volume size (in GiB) for the Pod VMs")
	reg.BoolWithEnv(&gcpcfg.UsePublicIP, "use-public-ip", false, "USE_PUBLIC_IP", "Use Public IP for connecting to the kata-agent inside the Pod VM")

	// Custom flag types (comma-separated lists)
	reg.CustomTypeWithEnv(&gcpcfg.Tags, "tags", "", "TAGS", "List of tags to be added to the Pod VMs. Tags must already exist in the GCP project. Format: key1=value1,key2=value2")
}

func (_ *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&gcpcfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &gcpcfg
}
