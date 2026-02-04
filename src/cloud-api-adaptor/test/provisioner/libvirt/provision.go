//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["libvirt"] = NewLibvirtProvisioner
	pv.NewInstallOverlayFunctions["libvirt"] = NewLibvirtInstallOverlay
	pv.NewInstallChartFunctions["libvirt"] = NewLibvirtInstallChart
}
