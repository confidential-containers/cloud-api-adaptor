// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// EcloudProps holds the global provisioner state for the e2e tests.
var EcloudProps = &EcloudProvisioner{}

// EcloudProvisioner implements the CloudProvisioner interface for ecloud.
// It assumes the Kubernetes cluster and network resources (subnet, security
// groups) already exist; it only wires the configuration through to the CAA
// deployment.
type EcloudProvisioner struct {
	AccessKeyID       string
	SecretKey         string
	Region            string
	ZoneID            string
	ImageID           string
	InstanceType      string
	NetworkID         string
	SecurityGroupIDs  string
	KeyName           string
	UsePublicIP       string
	PublicBandwidthSz string
	SystemDiskSize    string
	SystemDiskType    string
	Tags              string
	CaaImage          string
	ContainerRuntime  string
	DisableCVM        string
	PauseImage        string
	TunnelType        string
	VxlanPort         string
	Initdata          string
}

// EcloudInstallChart implements the InstallChart interface.
type EcloudInstallChart struct {
	Helm *pv.Helm
}

func credentialsFromEnv() (string, string) {
	return os.Getenv("ECLOUD_ACCESS_KEY_ID"), os.Getenv("ECLOUD_ACCESS_KEY_SECRET")
}

// NewEcloudProvisioner instantiates the ecloud provisioner.
func NewEcloudProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	accessKeyID, secretKey := credentialsFromEnv()
	if accessKeyID == "" || secretKey == "" {
		return nil, fmt.Errorf("ecloud credentials not found in environment (ECLOUD_ACCESS_KEY_ID/ECLOUD_ACCESS_KEY_SECRET)")
	}

	region := properties["region"]
	if region == "" {
		region = os.Getenv("REGION")
	}
	if region == "" {
		return nil, fmt.Errorf("region is required (set REGION env or 'region' property)")
	}

	EcloudProps = &EcloudProvisioner{
		AccessKeyID:       accessKeyID,
		SecretKey:         secretKey,
		Region:            region,
		ZoneID:            properties["zone_id"],
		ImageID:           properties["podvm_image_id"],
		InstanceType:      properties["podvm_instance_type"],
		NetworkID:         properties["network_id"],
		SecurityGroupIDs:  properties["security_group_ids"],
		KeyName:           properties["keyname"],
		UsePublicIP:       properties["use_public_ip"],
		PublicBandwidthSz: properties["public_bandwidth_size"],
		SystemDiskSize:    properties["system_disk_size"],
		SystemDiskType:    properties["system_disk_type"],
		Tags:              properties["tags"],
		CaaImage:          properties["CAA_IMAGE"],
		ContainerRuntime:  properties["container_runtime"],
		DisableCVM:        properties["disablecvm"],
		PauseImage:        properties["pause_image"],
		TunnelType:        properties["tunnel_type"],
		VxlanPort:         properties["vxlan_port"],
		Initdata:          properties["INITDATA"],
	}

	return EcloudProps, nil
}

// CreateVPC verifies that the required network resources exist. ecloud e2e
// relies on pre-created subnet/security groups, so this only checks the
// configuration is present.
func (e *EcloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	if e.NetworkID == "" {
		return fmt.Errorf("network_id is required (pre-created subnet ID)")
	}
	log.Infof("Using pre-created network %s in region %s", e.NetworkID, e.Region)
	return nil
}

// CreateCluster assumes an existing Kubernetes cluster pointed to by the
// current kubeconfig.
func (e *EcloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	kubeconfigPath := kconf.ResolveKubeConfigFile()
	if kubeconfigPath == "" {
		return fmt.Errorf("unable to find a kubeconfig file")
	}
	*cfg = *envconf.NewWithKubeConfig(kubeconfigPath)
	log.Infof("Using existing cluster (kubeconfig: %s)", kubeconfigPath)
	return nil
}

// DeleteCluster is a no-op: the cluster is managed externally.
func (e *EcloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

// DeleteVPC is a no-op: network resources are managed externally.
func (e *EcloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

// GetProperties returns the properties used to configure the CAA deployment.
func (e *EcloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"CAA_IMAGE":            e.CaaImage,
		"CONTAINER_RUNTIME":    e.ContainerRuntime,
		"disablecvm":           e.DisableCVM,
		"pause_image":          e.PauseImage,
		"podvm_image_id":       e.ImageID,
		"podvm_instance_type":  e.InstanceType,
		"network_id":           e.NetworkID,
		"security_group_ids":   e.SecurityGroupIDs,
		"keyname":              e.KeyName,
		"region":               e.Region,
		"zone_id":              e.ZoneID,
		"use_public_ip":        e.UsePublicIP,
		"public_bandwidth_size": e.PublicBandwidthSz,
		"system_disk_size":     e.SystemDiskSize,
		"system_disk_type":     e.SystemDiskType,
		"tags":                 e.Tags,
		"access_key_id":        e.AccessKeyID,
		"secret_access_key":    e.SecretKey,
		"tunnel_type":          e.TunnelType,
		"vxlan_port":           e.VxlanPort,
		"INITDATA":             e.Initdata,
	}
}

// UploadPodvm is a no-op: the podvm image must be pre-uploaded and its ID
// passed via the podvm_image_id property.
func (e *EcloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	if e.ImageID == "" {
		return fmt.Errorf("podvm_image_id is required; ecloud provisioner does not support automatic image upload")
	}
	log.Infof("Using pre-uploaded podvm image %s", e.ImageID)
	return nil
}

// NewEcloudInstallChart creates the InstallChart for ecloud.
func NewEcloudInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &EcloudInstallChart{
		Helm: helm,
	}, nil
}

// GetHelm returns the underlying Helm instance.
func (e *EcloudInstallChart) GetHelm() *pv.Helm { return e.Helm }

// Install installs the chart.
func (e *EcloudInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return e.Helm.Install(ctx, cfg)
}

// Uninstall uninstalls the chart.
func (e *EcloudInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return e.Helm.Uninstall(ctx, cfg)
}

// Configure maps the provisioner properties to Helm chart values.
func (e *EcloudInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	if properties["CAA_IMAGE"] != "" {
		img := strings.Split(properties["CAA_IMAGE"], ":")
		e.Helm.OverrideValues["image.name"] = img[0]
		if len(img) == 2 {
			e.Helm.OverrideValues["image.tag"] = img[1]
		}
	}

	if properties["CONTAINER_RUNTIME"] == "crio" {
		e.Helm.OverrideValues["kata-deploy.snapshotter.setup"] = ""
	}

	// Map internal property keys to Helm chart provider values.
	mapProps := map[string]string{
		"disablecvm":           "DISABLECVM",
		"pause_image":          "PAUSE_IMAGE",
		"podvm_image_id":       "IMAGEID",
		"podvm_instance_type":  "PODVM_INSTANCE_TYPE",
		"network_id":           "ECLOUD_NETWORK_ID",
		"security_group_ids":   "SECURITY_GROUP_IDS",
		"keyname":              "KEYNAME",
		"region":               "REGION",
		"zone_id":              "ECLOUD_ZONE_ID",
		"use_public_ip":        "USE_PUBLIC_IP",
		"public_bandwidth_size": "ECLOUD_PUBLIC_BANDWIDTH_SIZE",
		"system_disk_size":     "SYSTEM_DISK_SIZE",
		"system_disk_type":     "ECLOUD_SYSTEM_DISK_TYPE",
		"tags":                 "TAGS",
		"tunnel_type":          "TUNNEL_TYPE",
		"vxlan_port":           "VXLAN_PORT",
		"INITDATA":             "INITDATA",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			e.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	if properties["access_key_id"] != "" {
		e.Helm.OverrideProviderSecrets["ECLOUD_ACCESS_KEY_ID"] = properties["access_key_id"]
	}
	if properties["secret_access_key"] != "" {
		e.Helm.OverrideProviderSecrets["ECLOUD_ACCESS_KEY_SECRET"] = properties["secret_access_key"]
	}

	return nil
}
