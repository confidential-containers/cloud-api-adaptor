// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"fmt"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
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

func (t *podNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {

	nodeAddr := config.WorkerNodeIP

	if !nodeAddr.IsValid() {
		return fmt.Errorf("WorkerNodeIP is not specified: %#v", config.WorkerNodeIP)
	}

	podAddr := config.PodIP
	if !podAddr.IsValid() {
		return fmt.Errorf("PodIP is not specified: %#v", config.PodIP)
	}

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get host network namespace: %w", err)
	}
	defer hostNS.Close()

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a pod network namespace: %s: %w", nsPath, err)
	}
	defer podNS.Close()

	vxlanDevice := &netops.VXLAN{
		Group: nodeAddr.Addr(),
		ID:    config.VXLANID,
		Port:  config.VXLANPort,
	}
	vxlan, err := hostNS.LinkAdd(podVxlanInterface, vxlanDevice)
	if err != nil {
		return fmt.Errorf("failed to add vxlan interface %s: %w", podVxlanInterface, err)
	}

	if err := vxlan.SetNamespace(podNS); err != nil {
		return fmt.Errorf("failed to move vxlan interface %s to netns %s: %w", podVxlanInterface, podNS.Path(), err)
	}

	if err := vxlan.SetHardwareAddr(config.PodHwAddr); err != nil {
		return fmt.Errorf("failed to set pod HW address %s on %s: %w", config.PodHwAddr, podVxlanInterface, err)
	}

	mtu := int(config.MTU)
	if mtu > maxMTU {
		mtu = maxMTU
	}
	if err := vxlan.SetMTU(mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d on %s: %w", podVxlanInterface, mtu, nsPath, err)
	}

	if err := vxlan.AddAddr(podAddr); err != nil {
		return fmt.Errorf("failed to add pod IP %s to %s on %s: %w", podAddr, podVxlanInterface, nsPath, err)
	}

	if err := vxlan.SetUp(); err != nil {
		return err
	}

	// We need to process routes without gateway address first. Processing routes with a gateway causes an error if the gateway is not reachable.
	// Calico sets up routes with this pattern.
	// https://github.com/projectcalico/cni-plugin/blob/7495c0279c34faac315b82c1838bca638e23dbbe/pkg/dataplane/linux/dataplane_linux.go#L158-L167

	var first, second []*tunneler.Route
	for _, route := range config.Routes {
		if !route.GW.IsValid() {
			first = append(first, route)
		} else {
			second = append(second, route)
		}
	}
	routes := append(first, second...)

	for _, route := range routes {
		if err := podNS.RouteAdd(&netops.Route{Destination: route.Dst, Gateway: route.GW, Device: podVxlanInterface}); err != nil {
			return fmt.Errorf("failed to add a route to %s via %s on pod network namespace %s: %w", route.Dst, route.GW, nsPath, err)
		}
	}

	return nil
}

func (t *podNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}
