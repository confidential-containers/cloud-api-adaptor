//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
)

func init() {
	// Add this implementation to the list of provisioners.
	pv.NewProvisionerFunctions["aws"] = NewAWSProvisioner
	pv.NewInstallOverlayFunctions["aws"] = NewAwsInstallOverlay
}
