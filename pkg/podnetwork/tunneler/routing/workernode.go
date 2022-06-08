// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var logger = log.New(log.Writer(), "[tunneler/routing] ", log.LstdFlags|log.Lmsgprefix)

const (
	sourceRouteTablePriority = 505

	secondPodInterface = "eth1"

	vrf1Name = "ppvrf1"
	vrf2Name = "ppvrf2"

	vrf1TableID = 49001
	vrf2TableID = 49002
	minTableID  = 50000
	maxTableID  = 59999

	daemonListenPort = "15150"
)

const (
	vethPrefix = "ppveth"
)

type workerNodeTunneler struct {
}

func NewWorkerNodeTunneler() tunneler.Tunneler {
	return &workerNodeTunneler{}
}

func (t *workerNodeTunneler) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {

	if !config.Dedicated {
		return errors.New("shared subnet is not supported")
	}

	if len(podNodeIPs) != 2 {
		return errors.New("secondary pod node IP is not available")
	}

	podNodeIP := podNodeIPs[1]

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	workerNodeIP, _, err := net.ParseCIDR(config.WorkerNodeIP)
	if err != nil {
		return fmt.Errorf("failed to parse worker node IP %q", config.WorkerNodeIP)
	}
	hostInterface, err := hostNS.LinkNameByAddr(workerNodeIP)
	if err != nil {
		return fmt.Errorf("failed to identify host interface that has %s on netns %s", workerNodeIP.String(), hostNS.Path)
	}

	logger.Print("Ensure routing table entries and VRF devices on host")

	if err := hostNS.RuleAdd(nil, "", localTableNewPriority, unix.RT_TABLE_LOCAL); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add local table at priority %d: %w", localTableNewPriority, err)
	}
	if err = hostNS.RuleDel(nil, "", localTableOriginalPriority, unix.RT_TABLE_LOCAL); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete local table at priority %d: %w", localTableOriginalPriority, err)
	}

	if err := hostNS.LinkAdd(vrf1Name, &netlink.Vrf{Table: vrf1TableID}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add vrf %s: %w", vrf1Name, err)
	}

	if err := hostNS.LinkSetUp(vrf1Name); err != nil {
		return fmt.Errorf("failed to set vrf %s up: %w", vrf1Name, err)
	}

	if err := hostNS.LinkSetMaster(hostInterface, vrf1Name); err != nil {
		return fmt.Errorf("failed to set master of %s to vrf %s: %w", hostInterface, vrf1Name, err)
	}

	if err := hostNS.LinkAdd(vrf2Name, &netlink.Vrf{Table: vrf2TableID}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add vrf %s: %w", vrf2Name, err)
	}

	if err := hostNS.LinkSetUp(vrf2Name); err != nil {
		return fmt.Errorf("failed to set vrf %s up: %w", vrf2Name, err)
	}

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	vethName, err := hostNS.VethAddPrefix(vethPrefix, podNS, secondPodInterface)
	if err != nil {
		return err
	}
	if err := podNS.LinkSetUp(secondPodInterface); err != nil {
		return err
	}

	logger.Printf("Create a veth pair between host and Pod network namespace %s", nsPath)
	logger.Printf("    Host: %s", vethName)
	logger.Printf("    Pod:  %s", secondPodInterface)

	podIP, _, err := net.ParseCIDR(config.PodIP)
	if err != nil {
		return fmt.Errorf("failed to parse PodIP: %w", err)
	}

	podInterface := config.InterfaceName

	logger.Printf("Add tc redirect filters between %s and %s on pod network namespace %s", podInterface, secondPodInterface, nsPath)

	if err := podNS.RedirectAdd(podInterface, secondPodInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", podInterface, secondPodInterface, err)
	}

	if err := podNS.RedirectAdd(secondPodInterface, podInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", secondPodInterface, podInterface, err)
	}

	if err := hostNS.LinkSetMaster(vethName, vrf2Name); err != nil {
		return err
	}

	hwAddr, err := podNS.GetHardwareAddr(podInterface)
	if err != nil {
		return fmt.Errorf("failed to get hardware address of pod interface %s (netns %s): %w", podInterface, podNS.Path, err)
	}

	if err := hostNS.SetHardwareAddr(vethName, hwAddr); err != nil {
		return fmt.Errorf("failed to set hardware address %q to veth interface %s (netns %s): %w", hwAddr, vethName, hostNS.Path, err)
	}

	if err := hostNS.LinkSetUp(vethName); err != nil {
		return err
	}

	logger.Printf("Add a routing table entry to route traffic to Pod IP %s to PodVM IP %s", podIP, podNodeIP)

	// TODO: remove this sleep.
	// Without this sleep, add route fails due to "failed to create a route: network is unreachable",
	// when pod network is created for the first time
	time.Sleep(time.Second)

	if err := hostNS.RouteAdd(vrf2TableID, mask32(podIP), podNodeIP, hostInterface); err != nil {
		return fmt.Errorf("failed to add a route to pod VM: %w", err)
	}

	logger.Printf("Add Pod IP %s to %s and delete local route", podIP, vethName)
	// FIXME: Proxy arp does not become effective when no IP address is added to the interface, so we add pod IP to this interface, and delete its local route.
	if err := hostNS.AddrAdd(vethName, mask32(podIP)); err != nil {
		return err
	}
	if err := hostNS.LocalRouteDel(vrf2TableID, mask32(podIP), vethName); err != nil {
		return err
	}

	tableID := minTableID
	for {
		var err error
		tableID, err = hostNS.GetAvailableTableID(vrf1Name, sourceRouteTablePriority, tableID, maxTableID)
		if err != nil {
			return err
		}
		if err := hostNS.RouteAddOnlink(tableID, nil, net.ParseIP(config.Routes[0].GW), vethName); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add a route from a pod VM to a pod proxy: %w", err)
		}
		tableID++
	}
	logger.Printf("Add a routing table entry to route traffic from Pod VM %s back to pod network namespace %s", podNodeIP, nsPath)
	if err := hostNS.RuleAdd(mask32(podIP), vrf1Name, sourceRouteTablePriority, tableID); err != nil {
		return err
	}

	logger.Printf("Enable proxy ARP on %s", vethName)
	for key, val := range map[string]string{
		"net/ipv4/ip_forward": "1",
		fmt.Sprintf("net/ipv4/conf/%s/accept_local", vethName): "1",
		fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", vethName):    "1",
		fmt.Sprintf("net/ipv4/neigh/%s/proxy_delay", vethName): "0",
	} {
		if err := hostNS.SysctlSet(key, val); err != nil {
			return err
		}
	}

	if err := setIPTablesRules(hostNS, hostInterface); err != nil {
		return err
	}

	if err := startKeepAlive(hostNS, podIP, net.JoinHostPort(podNodeIP.String(), daemonListenPort), true); err != nil {
		return err
	}

	return nil
}

func (t *workerNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("Failed close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podIP, _, err := net.ParseCIDR(config.PodIP)
	if err != nil {
		return fmt.Errorf("failed to parse pod IP: %w", err)
	}

	logger.Printf("Delete routing table entries for Pod IP %s", podIP)

	if err := hostNS.RouteDel(vrf2TableID, mask32(podIP), nil, hostInterface); err != nil {
		return err
	}
	rules, err := hostNS.RuleList(mask32(podIP), vrf1Name, sourceRouteTablePriority, 0)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return fmt.Errorf("failed to identify rule %s vrf %s pref %d", podIP, vrf1Name, sourceRouteTablePriority)
	}
	for _, rule := range rules {
		if rule.Table == 0 {
			return fmt.Errorf("failed to identify table ID for rule %s vrf %s pref %d", podIP, vrf1Name, sourceRouteTablePriority)
		}
		err := hostNS.RuleDel(mask32(podIP), vrf1Name, sourceRouteTablePriority, rule.Table)
		if err != nil {
			return fmt.Errorf("failed to delete a rule %s vrf %s pref %d table %d: %w", podIP, vrf1Name, sourceRouteTablePriority, rule.Table, err)
		}
	}

	logger.Printf("Delete tc redirect filters on %s and %s in the network namespace %s", config.InterfaceName, hostInterface, nsPath)

	if err := podNS.RedirectDel(config.InterfaceName); err != nil {
		return fmt.Errorf("failed to delete a tc redirect filter from %s to %s: %w", config.InterfaceName, secondPodInterface, err)
	}

	if err := podNS.RedirectDel(secondPodInterface); err != nil {
		return fmt.Errorf("failed to delete a tc redirect filter from %s to %s: %w", secondPodInterface, config.InterfaceName, err)
	}

	logger.Printf("Delete veth %s in the network namespace %s", secondPodInterface, nsPath)

	if err := podNS.LinkDel(secondPodInterface); err != nil {
		return fmt.Errorf("failed to delete a veth interface %s at %s: %w", secondPodInterface, podNS.Name, err)
	}

	if err := stopKeepAlive(podIP, true); err != nil {
		logger.Printf("failed to stop keep alive: %v", err)
	}

	return nil
}
