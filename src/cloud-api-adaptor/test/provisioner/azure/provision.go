//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["azure"] = NewAzureCloudProvisioner
	pv.NewInstallOverlayFunctions["azure"] = NewAzureInstallOverlay
	pv.NewInstallChartFunctions["azure"] = NewAzureInstallChart
}
