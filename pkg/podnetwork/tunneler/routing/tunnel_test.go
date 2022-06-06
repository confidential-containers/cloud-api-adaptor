// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"testing"

	testutils "github.com/confidential-containers/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tuntest"
)

func TestRouting(t *testing.T) {
	// TODO: enable this test once https://github.com/confidential-containers/cloud-api-adaptor/issues/52 is fixed
	testutils.SkipTestIfRunningInCI(t)

	tuntest.RunTunnelTest(t, "routing", NewWorkerNodeTunneler, NewPodNodeTunneler, true)

}
