//go:build docker

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["docker"] = NewDockerProvisioner
	pv.NewInstallOverlayFunctions["docker"] = NewDockerInstallOverlay
	pv.NewInstallChartFunctions["docker"] = NewDockerInstallChart
}
