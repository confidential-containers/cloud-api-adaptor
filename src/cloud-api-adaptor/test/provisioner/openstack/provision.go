//go:build openstack

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["openstack"] = NewOpenStackProvisioner
	pv.NewInstallChartFunctions["openstack"] = NewOpenStackInstallChart
}
