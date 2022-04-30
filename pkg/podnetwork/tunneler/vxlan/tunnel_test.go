// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tuntest"
)

func TestRouting(t *testing.T) {

	tuntest.RunTunnelTest(t, "vxlan", NewWorkerNodeTunneler, NewPodNodeTunneler, false)

}
