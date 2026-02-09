// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"path/filepath"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// IBMCloudInstallChart implements the InstallChart interface
type IBMCloudInstallChart struct {
	Helm *pv.Helm
}

func NewIBMCloudInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &IBMCloudInstallChart{
		Helm: helm,
	}, nil
}

func (i *IBMCloudInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return i.Helm.Install(ctx, cfg)
}

func (i *IBMCloudInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return i.Helm.Uninstall(ctx, cfg)
}

func (i *IBMCloudInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Handle CAA image tag
	// IBMCloud uses CaaImageTag instead of CaaImage
	if IBMCloudProps.CaaImageTag != "" {
		log.Infof("Configuring helm: CAA image tag %q", IBMCloudProps.CaaImageTag)
		i.Helm.OverrideValues["image.tag"] = IBMCloudProps.CaaImageTag
	} else if isWorkerS390xFlavors() {
		// For s390x flavors, get the latest commit tag like kustomization does
		newTag := getCaaLatestCommitTag()
		if newTag != "" {
			log.Infof("Configuring helm: CAA image tag %q (latest commit for s390x)", newTag)
			i.Helm.OverrideValues["image.tag"] = newTag
		}
	}

	// Map properties to Helm chart providerConfigs
	// List matches the keys in install/charts/peerpods/providers/ibmcloud.yaml
	providerConfigKeys := []string{
		"CACERT_FILE",
		"CERT_FILE",
		"CERT_KEY",
		"CLOUD_CONFIG_VERIFY",
		"DISABLECVM",
		"ENABLE_SCRATCH_SPACE",
		"FORWARDER_PORT",
		"IBMCLOUD_CLUSTER_ID",
		"IBMCLOUD_DEDICATED_HOST_GROUP_IDS",
		"IBMCLOUD_DEDICATED_HOST_IDS",
		"IBMCLOUD_IAM_ENDPOINT",
		"IBMCLOUD_PODVM_IMAGE_ID",
		"IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST",
		"IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME",
		"IBMCLOUD_RESOURCE_GROUP_ID",
		"IBMCLOUD_SSH_KEY_ID",
		"IBMCLOUD_VPC_ENDPOINT",
		"IBMCLOUD_VPC_ID",
		"IBMCLOUD_VPC_SG_ID",
		"IBMCLOUD_VPC_SUBNET_ID",
		"IBMCLOUD_ZONE",
		"INITDATA",
		"PAUSE_IMAGE",
		"PEERPODS_LIMIT_PER_NODE",
		"PODS_DIR",
		"PROXY_TIMEOUT",
		"REMOTE_HYPERVISOR_ENDPOINT",
		"TAGS",
		"TLS_SKIP_VERIFY",
		"TUNNEL_TYPE",
		"VXLAN_PORT",
	}

	for _, key := range providerConfigKeys {
		if properties[key] != "" {
			i.Helm.OverrideProviderValues[key] = properties[key]
		}
	}

	// Map properties to Helm chart providerSecrets
	providerSecretKeys := []string{
		"IBMCLOUD_API_KEY",
		"IBMCLOUD_IAM_PROFILE_ID",
	}

	for _, key := range providerSecretKeys {
		if properties[key] != "" {
			i.Helm.OverrideProviderSecrets[key] = properties[key]
		}
	}

	if properties["CONTAINER_RUNTIME"] == "crio" {
		log.Print("Configuring helm: disable snapshotter setup")
		i.Helm.OverrideValues["kata-deploy.snapshotter.setup"] = ""
	}

	return nil
}
