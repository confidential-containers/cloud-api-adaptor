// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"log"
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

func getRoutes(ns *netops.NS) ([]*netops.Route, string, error) {

	routes, err := ns.GetRoutes()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path, err)
	}

	logger.Printf("routes on netns %s", ns.Path)
	for _, r := range routes {
		var dst, gw, dev string
		if r.Dst != nil {
			dst = r.Dst.String()
		} else {
			dst = "default"
		}
		if r.GW != nil {
			gw = "via " + r.GW.String()
		}
		if r.Dev != "" {
			dev = "dev " + r.Dev
		}
		logger.Printf("    %s %s %s", dst, gw, dev)
	}

	for _, r := range routes {
		if r.Dst == nil || r.Dst.IP == nil {
			return routes, r.Dev, nil
		}
		if r.Dst.IP.Equal(net.IPv4zero) && r.Dst.Mask != nil {
			ones, bits := r.Dst.Mask.Size()
			if bits != 0 && ones == 0 {
				return routes, r.Dev, nil
			}
		}
	}

	return nil, "", fmt.Errorf("failed to identify destination interface of default gateway on network namespace %q", ns.Path)
}
