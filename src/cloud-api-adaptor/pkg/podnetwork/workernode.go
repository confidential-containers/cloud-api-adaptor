// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

const DefaultTunnelType = "vxlan"

// netnsDir holds the named network namespace mount points on this node.
const netnsDir = "/run/netns"

type WorkerNode interface {
	Inspect(nsPath string) (*tunneler.Config, error)
	Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error
	Teardown(nsPath string, config *tunneler.Config) error
}

type workerNode struct {
	*tunneler.NetworkConfig
	tunneler tunneler.TunnelerConfigurator
}

// The pod index lives in this process only; NewWorkerNode seeds it past the
// VNIs still in use on the node so a restart does not reuse them.
var podIndexManager podIndex

type podIndex struct {
	mutex sync.Mutex
	index int
}

func (p *podIndex) Get() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	index := p.index
	p.index++
	return index
}

// SetMin makes the next Get return at least index; it never moves backwards.
func (p *podIndex) SetMin(index int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.index = max(p.index, index)
}

// nextPodIndex returns the index after the highest one whose vxlan device
// still exists in a namespace under dir, or 0 when there is none. The kernel
// keys a VNI on the underlay namespace the device was created in, so a pod
// that outlives a restart of this process keeps its VNI taken on the host.
// An entry that cannot be opened or listed is skipped, since a stale mount
// point left behind by a crashed teardown holds no VNI.
func nextPodIndex(dir string, minID, port int) (int, error) {
	if minID < 0 || minID > vxlan.MaxVXLANID {
		return 0, fmt.Errorf("vxlan minimum ID %d is not between 0 and %d", minID, vxlan.MaxVXLANID)
	}

	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to list network namespaces under %s: %w", dir, err)
	}

	next := 0
	for _, entry := range entries {
		nsPath := filepath.Join(dir, entry.Name())
		ns, err := netops.OpenNamespace(nsPath)
		if err != nil {
			logger.Printf("skipping %s: %v", nsPath, err)
			continue
		}
		links, err := ns.LinkList()
		if closeErr := ns.Close(); closeErr != nil {
			logger.Printf("failed to close network namespace %s: %v", nsPath, closeErr)
		}
		if err != nil {
			logger.Printf("skipping %s: %v", nsPath, err)
			continue
		}
		for _, link := range links {
			if link.Name() != vxlan.PodInterfaceName || link.Type() != "vxlan" {
				continue
			}
			device, err := link.GetDevice()
			if err != nil {
				return 0, fmt.Errorf("failed to inspect %s in %s: %w", link.Name(), nsPath, err)
			}
			v, ok := device.(*netops.VXLAN)
			if !ok || v.Port != port || v.ID < minID {
				continue
			}
			next = max(next, v.ID-minID+1)
		}
	}

	return next, nil
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

	if networkConfig.TunnelType == "vxlan" {
		next, err := nextPodIndex(netnsDir, networkConfig.VXLAN.MinID, networkConfig.VXLAN.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to find the vxlan VNIs in use: %w", err)
		}
		if next > 0 {
			logger.Printf("pod index starts at %d: VNI %d is still in use under %s", next, networkConfig.VXLAN.MinID+next-1, netnsDir)
		}
		podIndexManager.SetMin(next)
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
