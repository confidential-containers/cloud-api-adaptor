// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"log"
	"math"
	"net"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler/routing"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[podnetwork] ", log.LstdFlags|log.Lmsgprefix)

func init() {
	tunneler.Register("routing", routing.NewWorkerNodeTunneler, routing.NewPodNodeTunneler)
	tunneler.Register("vxlan", vxlan.NewWorkerNodeTunneler, vxlan.NewPodNodeTunneler)
}

// findPrimaryInterface identifies the primary interface on the given network namespace.
// An interface is considered to be primary if it is attached to the default route.
func findPrimaryInterface(ns netops.Namespace) (string, error) {

	dst := &net.IPNet{
		IP:   net.IPv4zero,
		Mask: make(net.IPMask, net.IPv4len),
	}
	routes, err := ns.RouteList(&netops.Route{Destination: dst})
	if err != nil {
		return "", fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path(), err)
	}

	var priority = math.MaxInt
	var dev string

	for _, r := range routes {
		var flag bool
		if r.Destination == nil {
			flag = true
		} else if ones, bits := r.Destination.Mask.Size(); ones == 0 && bits > 0 {
			flag = true
		}
		if flag && r.Priority < priority {
			dev = r.Device
		}
	}

	if dev == "" {
		return "", fmt.Errorf("failed to identify destination interface of default gateway on network namespace %q", ns.Path())
	}

	return dev, nil
}
