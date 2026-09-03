//go:build ibmcloud_powervs

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"context"
	"fmt"
	"path/filepath"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func NewIBMCloudPowerVSInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, "ibmcloud-powervs", debug)
	if err != nil {
		return nil, err
	}

	return &IBMCloudPowerVSInstallChart{
		Helm: helm,
	}, nil
}

type IBMCloudPowerVSInstallChart struct {
	Helm *pv.Helm
}

func (ic *IBMCloudPowerVSInstallChart) GetHelm() *pv.Helm {
	return ic.Helm
}

func (ic *IBMCloudPowerVSInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return ic.Helm.Install(ctx, cfg)
}

func (ic *IBMCloudPowerVSInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return ic.Helm.Uninstall(ctx, cfg)
}

func (ic *IBMCloudPowerVSInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	const chartProvider = "ibmcloud-powervs"

	// Set IBM Cloud API key as a provider secret.
	if key := properties["IBMCLOUD_API_KEY"]; key != "" {
		ic.Helm.OverrideValues[fmt.Sprintf("providerSecrets.%s.IBMCLOUD_API_KEY", chartProvider)] = key
	}

	// Set PowerVS-specific provider config values from properties along with the ones required for e2e.
	providerKeys := []string{
		"IBMCLOUD_ACCOUNT_ID",
		"POWERVS_ZONE",
		"POWERVS_SERVICE_INSTANCE_ID",
		"POWERVS_IMAGE_ID",
		"POWERVS_NETWORK_ID",
		"POWERVS_SSH_KEY_NAME",
		"POWERVS_MEMORY",
		"POWERVS_PROCESSORS",
		"POWERVS_PROCESSOR_TYPE",
		"POWERVS_SYSTEM_TYPE",
		"CONTAINER_RUNTIME",
		"PROXY_TIMEOUT",
		"USE_PUBLIC_IP",
		"FORWARDER_PORT",
	}
	for _, k := range providerKeys {
		if v := properties[k]; v != "" {
			ic.Helm.OverrideValues[fmt.Sprintf("providerConfigs.%s.%s", chartProvider, k)] = v
		}
	}

	return nil
}
