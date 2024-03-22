// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
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
)

const (
	vethPrefix = "ppveth"
)

type workerNodeTunneler struct {
}

func NewWorkerNodeTunneler() tunneler.Tunneler {
	return &workerNodeTunneler{}
}

func (t *workerNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {

	if !config.Dedicated {
		return errors.New("shared subnet is not supported")
	}

	if len(podNodeIPs) != 2 {
		return errors.New("secondary pod node IP is not available")
	}

	podNodeIP := podNodeIPs[1]

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	workerNodeIP := config.WorkerNodeIP
	if !workerNodeIP.IsValid() {
		return fmt.Errorf("WorkerNodeIP is not valid: %#v", config.WorkerNodeIP)
	}
	hostLink, err := findLinkByAddr(hostNS, workerNodeIP.Addr())
	if err != nil {
		return fmt.Errorf("failed to find an interface that has IP address %s on netns %s: %w", workerNodeIP.String(), hostNS.Path(), err)
	}

	logger.Print("Ensure routing table entries and VRF devices on host")

	if err := hostNS.RuleAdd(&netops.Rule{Priority: localTableNewPriority, Table: unix.RT_TABLE_LOCAL}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to add local table at priority %d: %w", localTableNewPriority, err)
	}
	if err = hostNS.RuleDel(&netops.Rule{Priority: localTableOriginalPriority, Table: unix.RT_TABLE_LOCAL}); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete local table at priority %d: %w", localTableOriginalPriority, err)
	}

	vrf1, err := hostNS.LinkAdd(vrf1Name, &netops.VRF{Table: vrf1TableID})
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			vrf1, err = hostNS.LinkFind(vrf1Name)
		}
		if err != nil {
			return fmt.Errorf("failed to add vrf %s: %w", vrf1Name, err)
		}
	}

	if err := vrf1.SetUp(); err != nil {
		return fmt.Errorf("failed to set vrf %s up: %w", vrf1Name, err)
	}

	if err := hostLink.SetMaster(vrf1); err != nil {
		return fmt.Errorf("failed to set master of %s to vrf %s: %w", hostLink.Name(), vrf1.Name(), err)
	}

	vrf2, err := hostNS.LinkAdd(vrf2Name, &netops.VRF{Table: vrf2TableID})
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			vrf2, err = hostNS.LinkFind(vrf2Name)
		}
		if err != nil {
			return fmt.Errorf("failed to add vrf %s: %w", vrf2Name, err)
		}
	}

	if err := vrf2.SetUp(); err != nil {
		return fmt.Errorf("failed to set vrf %s up: %w", vrf2Name, err)
	}

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	veth, err := createVethWithPrefix(vethPrefix, hostNS, podNS, secondPodInterface)
	if err != nil {
		return err
	}
	secondPodInterfaceLink, err := podNS.LinkFind(secondPodInterface)
	if err != nil {
		return fmt.Errorf("failed to find interface %q on %s: %w", secondPodInterface, nsPath, err)
	}
	if err := secondPodInterfaceLink.SetUp(); err != nil {
		return err
	}

	logger.Printf("Create a veth pair between host and Pod network namespace %s", nsPath)
	logger.Printf("    Host: %s", veth.Name())
	logger.Printf("    Pod:  %s", secondPodInterface)

	podIP := config.PodIP
	if !config.PodIP.IsValid() {
		return fmt.Errorf("PodIP is not valid: %#v", podIP)
	}

	podInterface := config.InterfaceName

	logger.Printf("Add tc redirect filters between %s and %s on pod network namespace %s", podInterface, secondPodInterface, nsPath)

	if err := podNS.RedirectAdd(podInterface, secondPodInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", podInterface, secondPodInterface, err)
	}

	if err := podNS.RedirectAdd(secondPodInterface, podInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", secondPodInterface, podInterface, err)
	}

	if err := veth.SetMaster(vrf2); err != nil {
		return err
	}

	podInterfaceLink, err := podNS.LinkFind(podInterface)
	if err != nil {
		return fmt.Errorf("failed to find of pod interface %q (netns %s): %w", podInterface, podNS.Path(), err)
	}

	hwAddr, err := podInterfaceLink.GetHardwareAddr()
	if err != nil {
		return fmt.Errorf("failed to get hardware address of pod interface %s (netns %s): %w", podInterface, podNS.Path(), err)
	}

	if err := veth.SetHardwareAddr(hwAddr); err != nil {
		return fmt.Errorf("failed to set hardware address %q to veth interface %s (netns %s): %w", hwAddr, veth.Name(), hostNS.Path(), err)
	}

	if err := veth.SetUp(); err != nil {
		return err
	}

	logger.Printf("Add a routing table entry to route traffic to Pod IP %s to PodVM IP %s", podIP, podNodeIP)

	// TODO: remove this sleep.
	// Without this sleep, add route fails due to "failed to create a route: network is unreachable",
	// when pod network is created for the first time
	time.Sleep(time.Second)

	if err := hostNS.RouteAdd(&netops.Route{Destination: mask32(podIP), Gateway: podNodeIP, Device: hostLink.Name(), Table: vrf2TableID}); err != nil {
		return fmt.Errorf("failed to add a route to pod VM: %w", err)
	}

	logger.Printf("Add Pod IP %s to %s and delete local route", podIP, veth.Name())
	// FIXME: Proxy arp does not become effective when no IP address is added to the interface, so we add pod IP to this interface, and delete its local route.
	if err := veth.AddAddr(mask32(podIP)); err != nil {
		return err
	}
	if err := hostNS.RouteDel(&netops.Route{Destination: mask32(podIP), Device: veth.Name(), Table: vrf2TableID, Type: unix.RTN_LOCAL, Protocol: unix.RTPROT_KERNEL}); err != nil {
		return err
	}

	tableID := minTableID
	for {
		var err error
		tableID, err = getAvailableTableID(hostNS, vrf1Name, sourceRouteTablePriority, tableID, maxTableID)
		if err != nil {
			return err
		}
		if err := hostNS.RouteAdd(&netops.Route{Gateway: config.Routes[0].GW, Device: veth.Name(), Table: tableID, Onlink: true}); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add a route from a pod VM to a pod proxy: %w", err)
		}
		tableID++
	}
	logger.Printf("Add a routing table entry to route traffic from Pod VM %s back to pod network namespace %s", podNodeIP, nsPath)
	if err := hostNS.RuleAdd(&netops.Rule{Src: mask32(podIP), IifName: vrf1Name, Priority: sourceRouteTablePriority, Table: tableID}); err != nil {
		return err
	}

	logger.Printf("Enable proxy ARP on %s", veth.Name())
	for key, val := range map[string]string{
		"net/ipv4/ip_forward": "1",
		fmt.Sprintf("net/ipv4/conf/%s/accept_local", veth.Name()): "1",
		fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", veth.Name()):    "1",
		fmt.Sprintf("net/ipv4/neigh/%s/proxy_delay", veth.Name()): "0",
	} {
		if err := sysctlSet(hostNS, key, val); err != nil {
			return err
		}
	}

	if err := setIPTablesRules(hostNS, hostLink.Name()); err != nil {
		return err
	}

	return nil
}

func (t *workerNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("Failed close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podIP := config.PodIP
	if !podIP.IsValid() {
		return fmt.Errorf("PodIP is not valid: %#v", podIP)
	}

	logger.Printf("Delete routing table entries for Pod IP %s", podIP)

	if err := hostNS.RouteDel(&netops.Route{Destination: mask32(podIP), Device: hostInterface, Table: vrf2TableID}); err != nil {
		return err
	}
	rules, err := hostNS.RuleList(&netops.Rule{Src: mask32(podIP), IifName: vrf1Name, Priority: sourceRouteTablePriority})
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
		err := hostNS.RuleDel(&netops.Rule{Src: mask32(podIP), IifName: vrf1Name, Priority: sourceRouteTablePriority, Table: rule.Table})
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

	secondPodInterfaceLink, err := podNS.LinkFind(secondPodInterface)
	if err != nil {
		return fmt.Errorf("failed to find interface %q on %s: %w", secondPodInterface, nsPath, err)
	}
	if err := secondPodInterfaceLink.Delete(); err != nil {
		return fmt.Errorf("failed to delete a veth interface %s at %s: %w", secondPodInterface, podNS.Path(), err)
	}

	return nil
}

func createVethWithPrefix(vethPrefix string, hostNS, peerNS netops.Namespace, peerName string) (netops.Link, error) {

	links, err := hostNS.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces on host: %w", err)
	}
	index := 1
	for {
		vethName := fmt.Sprintf("%s%d", vethPrefix, index)
		var found bool
		for _, link := range links {
			if link.Name() == vethName {
				found = true
				break
			}
		}
		if !found {
			link, err := hostNS.LinkAdd(vethName, &netops.VEth{PeerNamespace: peerNS, PeerName: peerName})
			if err == nil {
				return link, nil
			}
			if !errors.Is(err, os.ErrExist) {
				return nil, fmt.Errorf("failed to add veth pair %s and %s: %w", vethName, peerName, err)
			}
		}
		index++
	}
}

func getAvailableTableID(ns netops.Namespace, iif string, priority, min, max int) (int, error) {

	rule := netlink.NewRule()
	rule.Priority = priority
	rule.IifName = iif

	rules, err := ns.RuleList(&netops.Rule{IifName: iif, Priority: priority})
	if err != nil {
		return 0, fmt.Errorf("failed to get rules: %w", err)
	}

	used := make(map[int]bool)
	for _, rule := range rules {
		used[rule.Table] = true
	}

	for id := min; id <= max; id++ {
		if !used[id] {
			return id, nil
		}
	}
	return 0, fmt.Errorf("No table ID is available")
}
