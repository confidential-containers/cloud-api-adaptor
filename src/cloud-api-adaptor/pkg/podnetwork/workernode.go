// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

const DefaultTunnelType = "vxlan"

type WorkerNode interface {
	Inspect(nsPath string) (*tunneler.Config, error)
	Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error
	Teardown(nsPath string, config *tunneler.Config) error
}

type workerNode struct {
	*tunneler.NetworkConfig
	tunneler tunneler.TunnelerConfigurator
}

// TODO: Pod index is reset when this process restarts.
// We need to manage a persistent unique index number for each pod VM
var podIndexManager podIndex

type podIndex struct {
	index int
	mutex sync.Mutex
}

func (p *podIndex) Get() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	index := p.index
	p.index++
	return index
}

func NewWorkerNode(networkConfig *tunneler.NetworkConfig) (WorkerNode, error) {

	t, err := tunneler.WorkerNodeTunneler(networkConfig.TunnelType)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunneler: %w", err)
	}

	tun, ok := t.(tunneler.TunnelerConfigurator)
	if !ok {
		return nil, fmt.Errorf("internal error: Configure is not defined: %T", t)
	}

	wn := &workerNode{
		NetworkConfig: networkConfig,
		tunneler:      tun,
	}

	return wn, nil
}

func (n *workerNode) Inspect(nsPath string) (*tunneler.Config, error) {

	config := &tunneler.Config{
		TunnelType:          n.TunnelType,
		Index:               podIndexManager.Get(),
		ExternalNetViaPodVM: n.ExternalNetViaPodVM,
	}

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	hostPrimaryInterface, _, err := findPrimaryInterface(hostNS)
	if err != nil {
		return nil, fmt.Errorf("failed to identify the host primary interface: %w", err)
	}

	hostInterface := n.HostInterface
	if hostInterface == "" {
		hostInterface = hostPrimaryInterface
	} else if hostInterface != hostPrimaryInterface {
		config.Dedicated = true
	}

	hostLink, err := hostNS.LinkFind(hostInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to find host interface %q on netns %s: %w", hostInterface, hostNS.Path(), err)
	}

	addrs, err := hostLink.GetAddr()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP address on %s (netns: %s): %w", hostInterface, hostNS.Path(), err)
	}
	if len(addrs) != 1 {
		logger.Printf("more than one IP address (%v) assigned on %s (netns: %s)", addrs, hostInterface, hostNS.Path())
	}
	// Use the first IP as the workerNodeIP
	// TBD: Might be faster to retrieve using K8s downward API
	config.WorkerNodeIP = addrs[0]

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
	}
	defer func() {
		if err := podNS.Close(); err != nil {
			logger.Printf("failed to close a network namespace: %q", podNS.Path())
		}
	}()

	routes, err := podNS.RouteList()
	if err != nil {
		return nil, err
	}

	podInterface, gatewayAddr, err := findPrimaryInterface(podNS)
	if err != nil {
		return nil, err
	}

	logger.Printf("routes on netns %s", nsPath)
	for _, r := range routes {
		var dst, gw, dev string
		if r.Destination.IsValid() {
			dst = r.Destination.String()
		} else {
			dst = "default"
		}
		if r.Gateway.IsValid() {
			gw = "via " + r.Gateway.String()
		}
		if r.Device != "" {
			dev = "dev " + r.Device
		}

		logger.Printf("    %s %s %s", dst, gw, dev)
	}

	podLink, err := podNS.LinkFind(podInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to find pod interface %q on netns %s): %w", podInterface, podNS.Path(), err)
	}

	podIP, err := getPodIP(podLink)
	if err != nil {
		return nil, err
	}

	config.PodIP = podIP
	config.PodHwAddr, err = podLink.GetHardwareAddr()
	if err != nil {
		logger.Printf("failed to get Mac address of the Pod interface")
		return nil, fmt.Errorf("failed to get Mac address for Pod interface %s: %w", podInterface, err)
	}

	config.InterfaceName = podInterface

	mtu, err := podLink.GetMTU()
	if err != nil {
		return nil, fmt.Errorf("failed to get MTU size of %s: %w", podInterface, err)
	}
	config.MTU = mtu

	neighbors, err := podNS.NeighborList(&netops.Neighbor{Dev: podInterface, State: netops.NeighborStatePermanent})
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		r := &tunneler.Route{
			Dst:      route.Destination,
			Dev:      route.Device,
			GW:       route.Gateway,
			Protocol: route.Protocol,
			Scope:    route.Scope,
		}
		config.Routes = append(config.Routes, r)
	}

	// Add route for the subnet CIDRs in the new namespace
	if n.PodSubnetCIDRs != nil {
		for _, cidr := range n.PodSubnetCIDRs {
			prefix, err := netip.ParsePrefix(cidr)
			if err != nil {
				logger.Printf("failed to parse CIDR %q: %s", cidr, err)
				continue
			}
			route := &tunneler.Route{
				Dst: prefix,
				GW:  gatewayAddr,
				Dev: podInterface,
			}
			config.Routes = append(config.Routes, route)
		}
	}

	for _, neighbor := range neighbors {
		n := &tunneler.Neighbor{
			IP:           neighbor.IP,
			Dev:          neighbor.Dev,
			HardwareAddr: neighbor.HardwareAddr,
			State:        neighbor.State,
		}
		config.Neighbors = append(config.Neighbors, n)
	}

	if err := n.tunneler.Configure(n.NetworkConfig, config); err != nil {
		return nil, err
	}

	return config, nil
}

func (n *workerNode) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {

	if err := n.tunneler.Setup(nsPath, podNodeIPs, config); err != nil {
		return fmt.Errorf("failed to set up tunnel %q: %w", config.TunnelType, err)
	}

	return nil
}

func (n *workerNode) Teardown(nsPath string, config *tunneler.Config) error {

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	hostInterface := n.HostInterface
	if hostInterface == "" {
		hostPrimaryInterface, _, err := findPrimaryInterface(hostNS)
		if err != nil {
			return fmt.Errorf("failed to identify the host primary interface: %w", err)
		}
		hostInterface = hostPrimaryInterface
	}

	if err := n.tunneler.Teardown(nsPath, hostInterface, config); err != nil {
		return fmt.Errorf("failed to tear down tunnel %q: %w", config.TunnelType, err)
	}

	return nil
}

func getPodIP(podLink netops.Link) (netip.Prefix, error) {

	prefixes, err := podLink.GetAddr()
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("failed to get IP address on %s of netns %s: %w", podLink.Name(), podLink.Namespace().Path(), err)
	}

	var ips []netip.Prefix
	for _, prefix := range prefixes {
		if prefix.IsValid() && prefix.Addr().Is4() {
			ips = append(ips, prefix)
		}
	}
	if len(ips) < 1 {
		return netip.Prefix{}, fmt.Errorf("no IPv4 address found on %s of netns %s", podLink.Name(), podLink.Namespace().Path())
	}
	if len(ips) > 1 {
		return netip.Prefix{}, fmt.Errorf("more than one IPv4 addresses found on %s of netns %s", podLink.Name(), podLink.Namespace().Path())
	}
	return ips[0], nil
}
