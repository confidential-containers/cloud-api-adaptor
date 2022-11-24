// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"net"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

const DefaultTunnelType = "vxlan"

type WorkerNode interface {
	Inspect(nsPath string) (*tunneler.Config, error)
	Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error
	Teardown(nsPath string, config *tunneler.Config) error
}

type workerNode struct {
	tunnelType    string
	hostInterface string
	vxlanPort     int
	vxlanMinID    int
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

func NewWorkerNode(tunnelType, hostInterface string, vxlanPort, vxlanMinID int) WorkerNode {

	return &workerNode{
		tunnelType:    tunnelType,
		hostInterface: hostInterface,
		vxlanPort:     vxlanPort,
		vxlanMinID:    vxlanMinID,
	}
}

func (n *workerNode) Inspect(nsPath string) (*tunneler.Config, error) {

	config := &tunneler.Config{
		TunnelType: n.tunnelType,
		Index:      podIndexManager.Get(),
	}

	hostNS, err := netops.GetNS()
	if err != nil {
		return nil, fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	_, hostPrimaryInterface, err := getRoutes(hostNS)
	if err != nil {
		return nil, fmt.Errorf("failed to identify the host primary interface: %w", err)
	}

	hostInterface := n.hostInterface
	if hostInterface == "" {
		hostInterface = hostPrimaryInterface
	} else if hostInterface != hostPrimaryInterface {
		config.Dedicated = true
	}

	addrs, err := hostNS.GetIPNet(hostInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP address on %s (netns: %s): %w", hostInterface, hostNS.Path, err)
	}
	if len(addrs) != 1 {
		logger.Printf("more than one IP address (%v) assigned on %s (netns: %s)", addrs, hostInterface, hostNS.Path)
	}
	// Use the first IP as the workerNodeIP
	// TBD: Might be faster to retrieve using K8s downward API
	config.WorkerNodeIP = addrs[0].String()

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
	}
	defer func() {
		if err := podNS.Close(); err != nil {
			logger.Printf("failed to close a network namespace: %q", podNS.Path)
		}
	}()

	routes, podInterface, err := getRoutes(podNS)
	if err != nil {
		return nil, err
	}

	podIP, err := getPodIP(podNS, podInterface)
	if err != nil {
		return nil, err
	}

	config.PodIP = podIP
	config.PodHwAddr, err = podNS.GetHardwareAddr(podInterface)
	if err != nil {
		logger.Printf("failed to get Mac address of the Pod interface")
		return nil, fmt.Errorf("failed to get Mac address for Pod interface %s: %w", podInterface, err)
	}

	config.InterfaceName = podInterface

	mtu, err := podNS.GetMTU(podInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get MTU size of %s: %w", podInterface, err)
	}
	config.MTU = mtu

	for _, route := range routes {
		r := &tunneler.Route{
			Dev: route.Dev,
		}
		if route.Dst != nil {
			r.Dst = route.Dst.String()
		}
		if route.GW != nil {
			r.GW = route.GW.String()
		}
		config.Routes = append(config.Routes, r)
	}

	if n.tunnelType == "vxlan" {
		config.VXLANPort = n.vxlanPort
		config.VXLANID = n.vxlanMinID + config.Index
	}

	return config, nil
}

func (n *workerNode) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {

	tun, err := tunneler.WorkerNodeTunneler(n.tunnelType)
	if err != nil {
		return fmt.Errorf("failed to get tunneler: %w", err)
	}

	if err := tun.Setup(nsPath, podNodeIPs, config); err != nil {
		return fmt.Errorf("failed to set up tunnel %q: %w", config.TunnelType, err)
	}

	return nil
}

func (n *workerNode) Teardown(nsPath string, config *tunneler.Config) error {

	tun, err := tunneler.WorkerNodeTunneler(n.tunnelType)
	if err != nil {
		return fmt.Errorf("failed to get tunneler: %w", err)
	}

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	hostInterface := n.hostInterface
	if hostInterface == "" {
		_, hostPrimaryInterface, err := getRoutes(hostNS)
		if err != nil {
			return fmt.Errorf("failed to identify the host primary interface: %w", err)
		}
		hostInterface = hostPrimaryInterface
	}

	if err := tun.Teardown(nsPath, hostInterface, config); err != nil {
		return fmt.Errorf("failed to tear down tunnel %q: %w", config.TunnelType, err)
	}

	return nil
}

func getPodIP(podNS *netops.NS, podInterface string) (string, error) {

	ipNets, err := podNS.GetIPNet(podInterface)
	if err != nil {
		return "", fmt.Errorf("failed to get IP address on %s of netns %s: %w", podInterface, podNS.Path, err)
	}

	var ips []string
	for _, ipNet := range ipNets {
		if ipNet.IP.To4() != nil {
			ips = append(ips, ipNet.String())
		}
	}
	if len(ips) < 1 {
		return "", fmt.Errorf("no IPv4 address found on %s of netns %s", podInterface, podNS.Path)
	}
	if len(ips) > 1 {
		return "", fmt.Errorf("more than one IPv4 addresses found on %s of netns %s", podInterface, podNS.Path)
	}
	return ips[0], nil
}
