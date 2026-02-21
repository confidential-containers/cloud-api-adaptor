//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["ibmcloud"] = NewIBMCloudProvisioner
	pv.NewInstallOverlayFunctions["ibmcloud"] = NewIBMCloudInstallOverlay
	pv.NewInstallChartFunctions["ibmcloud"] = NewIBMCloudInstallChart
}
