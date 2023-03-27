//go:build ibmcloud_powervs

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	powervs "github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/ibmcloud-powervs"
)

func init() {
	cloudTable["ibmcloud-powervs"] = &powervs.Manager{}
}
