// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"testing"

	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork/tuntest"
)

func TestRouting(t *testing.T) {

	tuntest.RunTunnelTest(t, "routing", NewWorkerNodeTunneler, NewPodNodeTunneler, true)

}
