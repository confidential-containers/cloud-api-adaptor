// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"log"
	"math"

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

	routes, err := ns.RouteList(&netops.Route{Destination: netops.DefaultPrefix})
	if err != nil {
		return "", fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path(), err)
	}

	var priority = math.MaxInt
	var dev string

	for _, r := range routes {
		if r.Destination.Bits() == 0 && r.Priority < priority {
			dev = r.Device
		}
	}

	if dev == "" {
		return "", fmt.Errorf("failed to identify destination interface of default gateway on network namespace %q", ns.Path())
	}

	return dev, nil
}
