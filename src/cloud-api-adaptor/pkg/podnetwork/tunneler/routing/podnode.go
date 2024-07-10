// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
	"golang.org/x/sys/unix"
)

type podNodeTunneler struct{}

func NewPodNodeTunneler() tunneler.Tunneler {
	return &podNodeTunneler{}
}

const (
	hostVEthName = "veth0"

	podTableID          = 45001
	sourceTableID       = 45002
	sourceTablePriority = 505
)

func (t *podNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {

	if !config.Dedicated {
		return errors.New("shared subnet is not supported")
	}

	if len(podNodeIPs) != 2 {
		return errors.New("secondary pod node IP is not available")
	}

	podVEthName := config.InterfaceName
	if podVEthName == "" {
		return errors.New("InterfaceName is not specified")
	}

	podNodeIP := podNodeIPs[1]

	podIP := config.PodIP
	nodeIP := config.WorkerNodeIP

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get host network namespace: %w", err)
	}
	defer hostNS.Close()

	hostLink, err := findLinkByAddr(hostNS, podNodeIP)
	if err != nil {
		return fmt.Errorf("failed to find an interface that has IP address %s on netns %s: %w", podNodeIP.String(), hostNS.Path(), err)
	}

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a pod network namespace: %s: %w", nsPath, err)
	}
	defer podNS.Close()

	if err := hostNS.RuleAdd(&netops.Rule{Priority: localTableNewPriority, Table: unix.RT_TABLE_LOCAL}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add local table at priority %d: %w", localTableNewPriority, err)
	}

	if err = hostNS.RuleDel(&netops.Rule{Priority: localTableOriginalPriority, Table: unix.RT_TABLE_LOCAL}); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete local table at priority %d: %w", localTableOriginalPriority, err)
	}

	hostVEth, err := hostNS.LinkAdd(hostVEthName, &netops.VEth{PeerName: podVEthName, PeerNamespace: podNS})
	if err != nil {
		return fmt.Errorf("failed to create a veth pair: %s and %s on %s: %w", hostVEthName, podVEthName, nsPath, err)
	}
	podVEth, err := podNS.LinkFind(podVEthName)
	if err != nil {
		return fmt.Errorf("failed to find veth %q on %s: %w", podVEthName, nsPath, err)
	}

	mtu := int(config.MTU)
	if err := podVEth.SetMTU(mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d on %s: %w", podVEthName, mtu, nsPath, err)
	}

	if err := podVEth.AddAddr(podIP); err != nil {
		return fmt.Errorf("failed to add pod IP %s to %s on %s: %w", podIP, podVEthName, nsPath, err)
	}

	if err := podVEth.SetUp(); err != nil {
		return fmt.Errorf("failed to set %s up on %s: %w", podVEthName, nsPath, err)
	}

	if err := hostVEth.SetUp(); err != nil {
		return fmt.Errorf("failed to set %s up on host network namespace: %w", hostVEthName, err)
	}

	var defaultRouteGateway netip.Addr

	for _, route := range config.Routes {
		if err := podNS.RouteAdd(&netops.Route{Destination: route.Dst, Gateway: route.GW, Device: podVEthName}); err != nil {
			return fmt.Errorf("failed to add a route to %s via %s on pod network namespace %s: %w", route.Dst, route.GW, nsPath, err)
		}

		if !route.Dst.IsValid() || route.Dst.Bits() == 0 {
			defaultRouteGateway = route.GW
		}
	}

	if !defaultRouteGateway.IsValid() {
		return errors.New("no default route gateway is specified")
	}

	if err := hostVEth.AddAddr(netip.PrefixFrom(defaultRouteGateway, defaultRouteGateway.BitLen())); err != nil {
		return fmt.Errorf("failed to add GW IP %s to %s on host network namespace: %w", defaultRouteGateway, hostVEthName, err)
	}

	if err := hostNS.RouteAdd(&netops.Route{Destination: mask32(podIP), Device: hostVEthName, Table: podTableID}); err != nil {
		return fmt.Errorf("failed to add route table %d to pod %s IP on host network namespace: %w", podTableID, podIP, err)
	}

	if err := hostNS.RouteAdd(&netops.Route{Gateway: nodeIP.Addr(), Device: hostLink.Name(), Table: sourceTableID}); err != nil {
		return fmt.Errorf("failed to add route table %d to pod %s IP on host network namespace: %w", sourceTableID, podIP, err)
	}

	if err := hostNS.RuleAdd(&netops.Rule{Priority: podTablePriority, Table: podTableID}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add route table %d for pod IP at priority %d: %w", podTableID, podTablePriority, err)
	}

	if err := hostNS.RuleAdd(&netops.Rule{Src: mask32(podIP), IifName: hostVEthName, Priority: sourceTablePriority, Table: sourceTableID}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add route table %d for source routing at priority %d: %w", sourceTableID, sourceTablePriority, err)
	}

	for key, val := range map[string]string{
		"net/ipv4/ip_forward": "1",
		fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", hostVEthName):    "1",
		fmt.Sprintf("net/ipv4/neigh/%s/proxy_delay", hostVEthName): "0",
	} {
		if err := sysctlSet(hostNS, key, val); err != nil {
			return err
		}
	}

	return nil
}

func (t *podNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}
