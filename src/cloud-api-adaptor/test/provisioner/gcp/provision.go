//go:build gcp

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["gcp"] = NewGCPProvisioner
	pv.NewInstallOverlayFunctions["gcp"] = NewGCPInstallOverlay
	pv.NewInstallChartFunctions["gcp"] = NewGCPInstallChart
}
