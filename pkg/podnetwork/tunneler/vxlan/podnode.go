// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"fmt"
	"net"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/vishvananda/netlink"
)

const (
	podVxlanInterface = "vxlan0"
	maxMTU            = 1450
)

type podNodeTunneler struct {
}

func NewPodNodeTunneler() tunneler.Tunneler {
	return &podNodeTunneler{}
}

func (t *podNodeTunneler) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {

	nodeIP, _, err := net.ParseCIDR(config.WorkerNodeIP)
	if err != nil {
		return fmt.Errorf("failed to parse node IP %s: %w", config.WorkerNodeIP, err)
	}

	podIP, podIPNet, err := net.ParseCIDR(config.PodIP)
	if err != nil {
		return fmt.Errorf("failed to parse pod IP %s: %w", config.PodIP, err)
	}
	podIPNet.IP = podIP

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to get host network namespace: %w", err)
	}
	defer hostNS.Close()

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a pod network namespace: %s: %w", nsPath, err)
	}
	defer podNS.Close()

	vxlanLink := &netlink.Vxlan{
		Group:   nodeIP,
		VxlanId: config.VXLANID,
		Port:    config.VXLANPort,
	}
	if err := hostNS.LinkAdd(podVxlanInterface, vxlanLink); err != nil {
		return fmt.Errorf("failed to add vxlan interface %s: %w", podVxlanInterface, err)
	}

	if err := hostNS.LinkSetNS(podVxlanInterface, podNS); err != nil {
		return fmt.Errorf("failed to move vxlan interface %s to netns %s: %w", podVxlanInterface, podNS.Path, err)
	}

	if err := podNS.SetHardwareAddr(podVxlanInterface, config.PodHwAddr); err != nil {
		return fmt.Errorf("failed to set pod Mac %s on %s: %w", config.PodHwAddr, podVxlanInterface, err)
	}

	mtu := int(config.MTU)
	if mtu > maxMTU {
		mtu = maxMTU
	}
	if err := podNS.SetMTU(podVxlanInterface, mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d on %s: %w", podVxlanInterface, mtu, nsPath, err)
	}

	if err := podNS.AddrAdd(podVxlanInterface, podIPNet); err != nil {
		return fmt.Errorf("failed to add pod IP %s to %s on %s: %w", podIPNet, podVxlanInterface, nsPath, err)
	}

	if err := podNS.LinkSetUp(podVxlanInterface); err != nil {
		return err
	}

	// We need to process routes without gateway address first. Processing routes with a gateway causes an error if the gateway is not reachable.
	// Calico sets up routes with this pattern.
	// https://github.com/projectcalico/cni-plugin/blob/7495c0279c34faac315b82c1838bca638e23dbbe/pkg/dataplane/linux/dataplane_linux.go#L158-L167

	var first, second []*tunneler.Route
	for _, route := range config.Routes {
		if route.GW == "" {
			first = append(first, route)
		} else {
			second = append(second, route)
		}
	}
	routes := append(first, second...)

	for _, route := range routes {
		var dst *net.IPNet
		if route.Dst != "" {
			var err error
			_, dst, err = net.ParseCIDR(route.Dst)
			if err != nil {
				return fmt.Errorf("failed to add route destination %s: %w", route.Dst, err)
			}
		}
		var gw net.IP
		if route.GW != "" {
			gw = net.ParseIP(route.GW)
			if gw == nil {
				return fmt.Errorf("failed to parse GW IP: %s", route.GW)
			}
		}

		if err := podNS.RouteAdd(0, dst, gw, podVxlanInterface); err != nil {
			return fmt.Errorf("failed to add a route to %s via %s on pod network namespace %s: %w", dst, gw, nsPath, err)
		}
	}

	return nil
}

func (t *podNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}
