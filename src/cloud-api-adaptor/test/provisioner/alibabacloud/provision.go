//go:build alibabacloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["alibabacloud"] = NewAlibabaCloudProvisioner
	pv.NewInstallChartFunctions["alibabacloud"] = NewAlibabaCloudInstallChart
}
