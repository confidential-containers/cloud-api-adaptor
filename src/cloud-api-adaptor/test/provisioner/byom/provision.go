//go:build byom

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	pv.NewProvisionerFunctions["byom"] = NewByomProvisioner
	pv.NewInstallOverlayFunctions["byom"] = NewByomInstallOverlay
}
