//go:build vsphere

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudmgr

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/vsphere"
)

func init() {
	cloudTable["vsphere"] = &vsphere.Manager{}
}
