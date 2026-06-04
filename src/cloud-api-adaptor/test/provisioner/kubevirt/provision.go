//go:build kubevirt

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["kubevirt"] = NewKubeVirtProvisioner
	pv.NewInstallChartFunctions["kubevirt"] = NewKubeVirtInstallChart
}
