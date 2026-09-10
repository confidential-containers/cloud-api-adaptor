// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"math"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tuntest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVXLAN(t *testing.T) {

	tuntest.RunTunnelTest(t, "vxlan", NewWorkerNodeTunneler, NewPodNodeTunneler, false)

}

func TestConfigure(t *testing.T) {
	tun := &workerNodeTunneler{}

	t.Run("derives the VNI from the minimum ID and the pod index", func(t *testing.T) {
		config := &tunneler.Config{Index: 3}
		require.NoError(t, tun.Configure(&tunneler.NetworkConfig{VXLAN: tunneler.VXLANConfig{Port: DefaultVXLANPort, MinID: DefaultVXLANMinID}}, config))
		assert.Equal(t, DefaultVXLANMinID+3, config.VXLANID)
		assert.Equal(t, DefaultVXLANPort, config.VXLANPort)
	})

	t.Run("rejects a pod index outside the remaining range", func(t *testing.T) {
		for _, index := range []int{-1, 1, math.MaxInt} {
			config := &tunneler.Config{Index: index}
			assert.Error(t, tun.Configure(&tunneler.NetworkConfig{VXLAN: tunneler.VXLANConfig{MinID: MaxVXLANID}}, config), "index=%d", index)
		}
	})

	t.Run("rejects a minimum ID outside the field", func(t *testing.T) {
		for _, minID := range []int{-1, MaxVXLANID + 1} {
			assert.Error(t, tun.Configure(&tunneler.NetworkConfig{VXLAN: tunneler.VXLANConfig{MinID: minID}}, &tunneler.Config{}), "minID=%d", minID)
		}
	})
}
