// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	//"google.golang.org/api/option"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"

	//"google.golang.org/api/compute/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var GCPProps = &GCPProvisioner{}

type GCPProvisioner struct {
	GkeCluster       *GKECluster
	GcpVPC           *GCPVPC
	GcpOverlay       *GCPInstallOverlay
	CaaImage         string
	PodvmImageName   string
	PodvmMachineType string
}

func NewGCPProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	// TODO:
	// ctx := context.Background()

	// Replace credentials with expanded version to be used from now on
	credPath, err := expandUser(properties["credentials"])
	if err != nil {
		return nil, err
	}
	properties["credentials"] = credPath

	gkeCluster, err := NewGKECluster(properties)
	if err != nil {
		return nil, err
	}

	gcpVPC, err := NewGCPVPC(properties)
	if err != nil {
		return nil, err
	}

	println("Properties received by NewGCPProvisioner")
	for k, v := range properties {
		println(k, "=", v)
	}

	GCPProps = &GCPProvisioner{
		GkeCluster:       gkeCluster,
		GcpVPC:           gcpVPC,
		CaaImage:         properties["caa_image"],
		PodvmImageName:   properties["podvm_image_name"],
		PodvmMachineType: properties["podvm_machine_type"],
	}
	return GCPProps, nil
}

func (p *GCPProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	err := p.GkeCluster.CreateCluster(ctx)
	if err != nil {
		return err
	}

	kubeconfigPath, err := p.GkeCluster.GetKubeconfigFile(ctx)
	if err != nil {
		return err
	}
	*cfg = *envconf.NewWithKubeConfig(kubeconfigPath)

	return nil
}

func (p *GCPProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return p.GcpVPC.CreateVPC(ctx, cfg)
}

func (p *GCPProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return p.GkeCluster.DeleteCluster(ctx)
}

func (p *GCPProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return p.GcpVPC.DeleteVPC(ctx, cfg)
}

func (p *GCPProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		// GkeCluster properties
		"project_id":           p.GkeCluster.ProjectID,
		"credentials":          p.GkeCluster.credentials,
		"cluster_name":         p.GkeCluster.clusterName,
		"cluster_version":      p.GkeCluster.clusterVersion,
		"cluster_machine_type": p.GkeCluster.clusterMachineType,
		"zone":                 p.GkeCluster.Zone,
		"node_count":           fmt.Sprint(p.GkeCluster.nodeCount),

		// GkeVpc
		"vpc_name": p.GcpVPC.vpcName,

		// Overlay Parameters
		"caa_image":          p.CaaImage,
		"podvm_machine_type": p.PodvmMachineType, // GCP_MACHINE_TYPE
		"podvm_image_name":   p.PodvmImageName,   // PODVM_IMAGE_NAME
	}
}

func (p *GCPProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

// GCPInstallChart implements the InstallChart interface
type GCPInstallChart struct {
	Helm *pv.Helm
}

func NewGCPInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &GCPInstallChart{
		Helm: helm,
	}, nil
}

func (g *GCPInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return g.Helm.Install(ctx, cfg)
}

func (g *GCPInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return g.Helm.Uninstall(ctx, cfg)
}

func (g *GCPInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Handle CAA image - split on ":" like kustomization does
	if GCPProps.CaaImage != "" {
		parts := strings.Split(GCPProps.CaaImage, ":")
		if len(parts) >= 1 && parts[0] != "" {
			log.Infof("Configuring helm: CAA image %q", parts[0])
			g.Helm.OverrideValues["image.name"] = parts[0]
		}
		if len(parts) >= 2 && parts[1] != "" {
			log.Infof("Configuring helm: CAA image tag %q", parts[1])
			g.Helm.OverrideValues["image.tag"] = parts[1]
		}
	}

	// Map properties to Helm chart providerConfigs
	// List matches the keys in install/charts/peerpods/providers/gcp.yaml
	providerConfigKeys := map[string]string{
		"podvm_image_name":   "PODVM_IMAGE_NAME",
		"podvm_machine_type": "GCP_MACHINE_TYPE",
		"project_id":         "GCP_PROJECT_ID",
		"zone":               "GCP_ZONE",
		"vpc_name":           "GCP_NETWORK",
	}

	for k, v := range providerConfigKeys {
		if properties[k] != "" {
			g.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	// Handle GCP credentials - read from file and set as secret
	credsPath := properties["credentials"]
	if credsPath != "" {
		credData, err := os.ReadFile(credsPath)
		if err != nil {
			return fmt.Errorf("failed to read GCP credentials file %q: %w", credsPath, err)
		}
		log.Info("Configuring helm: GCP credentials")
		g.Helm.OverrideProviderSecrets["GCP_CREDENTIALS"] = string(credData)
	}

	return nil
}
