//go:build ecloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ecloud

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["ecloud"] = NewEcloudProvisioner
	pv.NewInstallChartFunctions["ecloud"] = NewEcloudInstallChart
}
