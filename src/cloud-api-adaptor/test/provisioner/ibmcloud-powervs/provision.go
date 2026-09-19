//go:build ibmcloud_powervs

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["ibmcloud_powervs"] = NewIBMCloudPowerVSProvisioner
	pv.NewInstallChartFunctions["ibmcloud_powervs"] = NewIBMCloudPowerVSInstallChart
}
