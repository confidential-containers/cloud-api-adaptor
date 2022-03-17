// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/containernetworking/plugins/pkg/utils/sysctl"
)

var logger = log.New(log.Writer(), "[util/netops] ", log.LstdFlags|log.Lmsgprefix)

// NS represents a network namespace
type NS struct {
	Name     string
	Path     string
	nsHandle netns.NsHandle
	handle   *netlink.Handle
}

// NewNSFromPath returns an NS specified by a namespace path
func NewNSFromPath(nsPath string) (*NS, error) {

	netnsDir := "/run/netns"
	altNetnsDir := "/var/run/netns"

	if filepath.Dir(nsPath) == altNetnsDir {
		nsPath = filepath.Join(netnsDir, filepath.Base(nsPath))
	}

	if filepath.Dir(nsPath) != netnsDir {
		return nil, fmt.Errorf("%s is not in %s", nsPath, netnsDir)
	}

	name := filepath.Base(nsPath)

	nsHandle, err := netns.GetFromPath(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get network namespace %s: %w", nsPath, err)
	}
	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get a handle for network namespace %s: %w", nsPath, err)
	}
	ns := &NS{
		Name:     name,
		Path:     nsPath,
		nsHandle: nsHandle,
		handle:   handle,
	}
	return ns, nil
}

// GetNS returns an NS of the current network namespace
func GetNS() (*NS, error) {

	pid := os.Getpid()
	tid := unix.Gettid()
	path := fmt.Sprintf("/proc/%d/task/%d/ns/net", pid, tid)

	nsHandle, err := netns.GetFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current network namespace: %w", err)
	}
	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get a netlink handle for the current network namespace: %w", err)
	}
	ns := &NS{
		nsHandle: nsHandle,
		handle:   handle,
		Path:     path,
	}
	return ns, nil
}

// NewNamedNS returns an NS of the current network namespace
func NewNamedNS(name string) (*NS, error) {
	runtime.LockOSThread()
	defer runtime.LockOSThread()

	old, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get the current network namespace: %w", err)
	}
	defer netns.Set(old)

	nsHandle, err := netns.NewNamed(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get current network namespace: %w", err)
	}
	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get a netlink handle for the current network namespace: %w", err)
	}
	ns := &NS{
		Name:     name,
		Path:     filepath.Join("/run/netns", name),
		nsHandle: nsHandle,
		handle:   handle,
	}
	return ns, nil
}

// Clone returns a copy of a network namespace. A returned network namespace need to be closed separately.
func (ns *NS) Clone() (*NS, error) {

	nsHandle, err := netns.GetFromPath(ns.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get network namespace at %q: %w", ns.Path, err)
	}
	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get a netlink handle for network namespace at %q: %w", ns.Path, err)
	}

	clone := &NS{
		nsHandle: nsHandle,
		handle:   handle,
		Path:     ns.Path,
	}
	return clone, nil
}

// FD returns a file descriptor of the network namespace
func (ns *NS) FD() int {
	return int(ns.nsHandle)
}

// Close closes an NS
func (ns *NS) Close() error {

	ns.handle.Close()

	if err := ns.nsHandle.Close(); err != nil {
		return fmt.Errorf("failed to close network namespace %v: %w", ns.handle, err)
	}

	return nil
}

// Delete deletes a named NS
func (ns *NS) Delete() error {
	if err := netns.DeleteNamed(ns.Name); err != nil {
		return fmt.Errorf("failed to delete a named network namespace %s: %w", ns.Name, err)
	}
	return nil
}

// Run calls a function in a network namespace
func (ns *NS) Run(fn func() error) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oldNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := oldNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	if ns.nsHandle != oldNS {
		if err := netns.Set(ns.nsHandle); err != nil {
			return fmt.Errorf("failed to set a network namespace: %w", err)
		}
		defer func() {
			if e := netns.Set(oldNS); e != nil {
				err = fmt.Errorf("failed to set back to the host network namespace: %w (previous error %v)", err, e)
			}
		}()
	}

	return fn()
}

// SysctlGet returns a kernel parameter value specified by key
func (ns *NS) SysctlGet(key string) (string, error) {
	var val string
	err := ns.Run(func() error {
		var err error

		val, err = sysctl.Sysctl(key)
		if err != nil {
			return fmt.Errorf("failed to get a value of sysctl parameter %q: %w", key, err)
		}
		return err
	})
	return val, err
}

// SysctlSet returns a kernel parameter value specified by key
func (ns *NS) SysctlSet(key string, val string) error {

	err := ns.Run(func() error {
		if _, err := sysctl.Sysctl(key, val); err != nil {
			return fmt.Errorf("failed to set sysctl parameter %q to %q: %w", key, val, err)
		}
		return nil
	})
	return err
}

// GetIP returns a list of IP addresses assigned to a link
func (ns *NS) GetIP(linkName string) ([]net.IP, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	addrs, err := ns.handle.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to get addressess assigned to %s (type: %s, netns: %s): %w", linkName, link.Type(), ns.Path, err)
	}

	var ips []net.IP
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}

	return ips, nil
}

// GetIPNet returns a list of IPNets assigned to a link
func (ns *NS) GetIPNet(linkName string) ([]*net.IPNet, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	addrs, err := ns.handle.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to delete an interface of %s: %s:  %w", linkName, link.Type(), err)
	}

	var ipNets []*net.IPNet
	for _, addr := range addrs {
		ipNets = append(ipNets, addr.IPNet)
	}

	return ipNets, nil
}

// GetMTU returns MTU size of a link
func (ns *NS) GetMTU(linkName string) (int, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return 0, fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	mtu := link.Attrs().MTU

	return mtu, nil
}

// SetMTU sets MTU size of a link
func (ns *NS) SetMTU(linkName string, mtu int) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d: %w", linkName, mtu, err)
	}

	return nil
}

func (ns *NS) GetHardwareAddr(linkName string) (string, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return "", fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	hwAddr := link.Attrs().HardwareAddr.String()

	return hwAddr, nil
}

func (ns *NS) SetHardwareAddr(linkName, hwAddr string) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	mac, err := net.ParseMAC(hwAddr)
	if err != nil {
		return fmt.Errorf("failed to parse hardware address %q: %w", hwAddr, err)

	}

	if err := ns.handle.LinkSetHardwareAddr(link, mac); err != nil {
		return fmt.Errorf("failed to set hardware address %q to %s (netns: %s): %w", hwAddr, linkName, ns.Path, err)
	}

	return nil
}

func (ns *NS) LinkNameByAddr(ip net.IP) (string, error) {

	links, err := ns.handle.LinkList()
	if err != nil {
		return "", fmt.Errorf("failed to get a list of interfaces (netns: %s): %w", ns.Path, err)
	}

	var names []string
	for _, link := range links {
		name := link.Attrs().Name
		addrs, err := ns.GetIP(name)
		if err != nil {
			return "", fmt.Errorf("failed to obtain addresses assigned to %s (netns: %s)", name, ns.Path)
		}
		for _, addr := range addrs {
			if addr.Equal(ip) {
				names = append(names, name)
				break
			}
		}
	}

	if len(names) == 0 {
		return "", fmt.Errorf("failed to find interface that has %s on netns %s", ip.String(), ns.Path)
	}
	if len(names) > 1 {
		return "", fmt.Errorf("multile interfaces have %s on netns %s: %s", ip.String(), ns.Path, strings.Join(names, ", "))
	}

	return names[0], nil
}

func (ns *NS) LinkList() ([]netlink.Link, error) {

	links, err := ns.handle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of interfaces: %w", err)
	}

	return links, nil
}

func (ns *NS) LinkByName(name string) (netlink.Link, error) {

	link, err := ns.handle.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", name, err)
	}

	return link, nil
}

// LinkAdd creates a new link with an attribute specified by link
func (ns *NS) LinkAdd(linkName string, link netlink.Link) error {
	attrs := link.Attrs()
	*attrs = netlink.NewLinkAttrs()
	attrs.Name = linkName
	if err := ns.handle.LinkAdd(link); err != nil {
		return fmt.Errorf("failed to create an interface of %s: %s:  %w", link.Type(), linkName, err)
	}
	return nil
}

// LinkDel deletes a link
func (ns *NS) LinkDel(linkName string) error {
	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	if err := ns.handle.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete an interface of %s: %s:  %w", linkName, link.Type(), err)
	}
	return nil
}

// LinkSetMaster sets a master device of a link
func (ns *NS) LinkSetMaster(linkName, masterName string) error {
	master, err := ns.handle.LinkByName(masterName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", masterName, err)
	}

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	if err := ns.handle.LinkSetMaster(link, master); err != nil {
		return fmt.Errorf("failed to set master device of %s to %s: %w", linkName, masterName, err)
	}
	return nil
}

// LinkSetNS changes network namespace of a link
func (ns *NS) LinkSetNS(linkName string, targetNS *NS) error {

	link, err := ns.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetNsFd(link, targetNS.FD()); err != nil {
		return fmt.Errorf("failed to change network namespace of interface %s from %s to %s: %w", linkName, ns.Path, targetNS.Path, err)
	}

	return nil
}

// LinkSetName changes name of a link
func (ns *NS) LinkSetName(linkName, newName string) error {

	link, err := ns.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetName(link, newName); err != nil {
		return fmt.Errorf("failed to change name of interface %s on %s to %s: %w", linkName, ns.Path, newName, err)
	}

	return nil
}

// AddrAdd adds an IP address to a link
func (ns *NS) AddrAdd(linkName string, addr *net.IPNet) error {
	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	if err := ns.handle.AddrAdd(link, &netlink.Addr{IPNet: addr}); err != nil {
		return fmt.Errorf("failed to assign an IP address to %s: %w", linkName, err)
	}
	return nil
}

// LinkSetUp makes the link status up
func (ns *NS) LinkSetUp(linkName string) error {
	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	if err := ns.handle.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to set link state up: %s: %w", linkName, err)
	}
	return nil
}

// RouteAdd adds a new route
func (ns *NS) RouteAdd(table int, dest *net.IPNet, gw net.IP, dev string) error {
	if dest == nil {
		_, dest, _ = net.ParseCIDR("0.0.0.0/0")
	}
	route := &netlink.Route{
		Table: table,
		Dst:   dest,
		Gw:    gw,
	}
	if dev != "" {
		link, err := ns.handle.LinkByName(dev)
		if err != nil {
			return fmt.Errorf("failed to get interface %s: %w", dev, err)
		}
		route.LinkIndex = link.Attrs().Index
	}
	if gw == nil {
		route.Scope = netlink.SCOPE_LINK
	}
	if err := ns.handle.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to create a route: %w", err)
	}
	return nil
}

func (ns *NS) RouteAddOnlink(table int, dest *net.IPNet, gw net.IP, dev string) error {
	if dest == nil {
		_, dest, _ = net.ParseCIDR("0.0.0.0/0")
	}
	route := &netlink.Route{
		Table: table,
		Dst:   dest,
		Gw:    gw,
		Flags: int(netlink.FLAG_ONLINK),
	}
	if dev != "" {
		link, err := ns.handle.LinkByName(dev)
		if err != nil {
			return fmt.Errorf("failed to get interface %s: %w", dev, err)
		}
		route.LinkIndex = link.Attrs().Index
	}
	if gw == nil {
		route.Scope = netlink.SCOPE_LINK
	}
	if err := ns.handle.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to create a route (table: %d, dest: %s, gw: %s) with flags %d: %w", route.Table, route.Dst.String(), route.Gw.String(), route.Flags, err)
	}
	return nil
}

// RouteDel deletes routes
func (ns *NS) RouteDel(table int, dest *net.IPNet, gw net.IP, dev string) error {
	filterMask := netlink.RT_FILTER_DST
	if dest == nil {
		_, dest, _ = net.ParseCIDR("0.0.0.0/0")
	}
	route := &netlink.Route{
		Dst: dest,
	}
	if table != 0 {
		route.Table = table
		filterMask |= netlink.RT_FILTER_TABLE
	}
	if gw != nil {
		route.Gw = gw
		filterMask |= netlink.RT_FILTER_GW
	}
	if dev != "" {
		link, err := ns.handle.LinkByName(dev)
		if err != nil {
			return fmt.Errorf("failed to get interface %s: %w", dev, err)
		}
		route.LinkIndex = link.Attrs().Index
		filterMask |= netlink.RT_FILTER_OIF
	}
	routes, err := ns.handle.RouteListFiltered(netlink.FAMILY_V4, route, filterMask)
	if err != nil {
		return fmt.Errorf("failed to get a list of routes: table %d, dest: %s, gw: %s, dev %s: %w", table, dest, gw, dev, err)
	}
	for _, route := range routes {
		if err := ns.handle.RouteDel(&route); err != nil {
			return fmt.Errorf("failed to delete a route: table %d, dest: %s, gw: %s, dev %s: %w", table, dest, gw, dev, err)
		}
	}
	return nil
}

type Route struct {
	Dst *net.IPNet
	GW  net.IP
	Dev string
}

// GetRoutes gets a list of routes on the main table
func (ns *NS) GetRoutes() ([]*Route, error) {

	routeList, err := ns.handle.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes on namespace %q: %w", ns.Name, err)
	}

	var routes []*Route
	for _, r := range routeList {

		var dev string
		if r.LinkIndex > 0 {
			link, err := ns.handle.LinkByIndex(r.LinkIndex)
			if err != nil {
				return nil, fmt.Errorf("failed to get a link with index %d of a route: %w", r.LinkIndex, err)
			}
			dev = link.Attrs().Name
		}

		if r.Type != unix.RTN_UNICAST {
			logger.Printf("route (dst:%s, gw:%s: dev:%s) has unexpected type %d. ignoring", r.Dst, r.Gw, dev, r.Type)
			continue
		}

		if r.Table != unix.RT_TABLE_MAIN {
			logger.Printf("route (dst:%s, gw:%s: dev:%s) has unexpected table %d. ignoring", r.Dst, r.Gw, dev, r.Table)
			continue
		}

		if r.Flags != 0 {
			logger.Printf("route (dst:%s, gw:%s: dev:%s) has unexpected flags %d. ignoring", r.Dst, r.Gw, dev, r.Flags)
			continue
		}

		switch r.Protocol {
		case unix.RTPROT_STATIC:
		case unix.RTPROT_BOOT:
		default:
			logger.Printf("route (dst:%s, gw:%s: dev:%s) has unexpected protocol %d. ignoring", r.Dst, r.Gw, dev, r.Protocol)
			continue
		}

		switch r.Scope {
		case netlink.SCOPE_UNIVERSE:
		case netlink.SCOPE_LINK:
		case netlink.SCOPE_HOST:
		default:
			logger.Printf("route (dst:%s, gw:%s: dev:%s) has unexpected scope %d. ignoring", r.Dst, r.Gw, dev, r.Scope)
			continue
		}

		route := &Route{
			Dst: r.Dst,
			GW:  r.Gw,
			Dev: dev,
		}
		routes = append(routes, route)
	}

	return routes, nil
}

// LocalRouteDel deletes a route of the local local
func (ns *NS) LocalRouteDel(table int, dest *net.IPNet, dev string) error {

	link, err := ns.handle.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", dev, err)
	}
	route := &netlink.Route{
		Table:     table,
		Dst:       dest,
		Src:       dest.IP,
		Type:      unix.RTN_LOCAL,
		Protocol:  unix.RTPROT_KERNEL,
		Scope:     unix.RT_SCOPE_HOST,
		LinkIndex: link.Attrs().Index,
	}
	if err := ns.handle.RouteDel(route); err != nil {
		return fmt.Errorf("failed to delete a route: table %d, dest %s, dev %s: %w", table, dest, dev, err)
	}
	return nil
}

// GetAvailableTableID returns a table ID that is not currently used
func (ns *NS) GetAvailableTableID(iif string, priority, min, max int) (int, error) {
	rule := netlink.NewRule()
	rule.Priority = priority
	rule.IifName = iif

	rules, err := ns.handle.RuleListFiltered(netlink.FAMILY_V4, nil, netlink.RT_FILTER_PRIORITY|netlink.RT_FILTER_IIF)
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

// RuleAdd adds a new rule in the routing policy database
func (ns *NS) RuleAdd(src *net.IPNet, iif string, priority int, table int) error {
	rule := netlink.NewRule()
	rule.Src = src
	rule.IifName = iif
	rule.Priority = priority
	rule.Table = table

	if err := ns.handle.RuleAdd(rule); err != nil {
		return fmt.Errorf("failed to add a rule: %w", err)
	}
	return nil
}

// RuleDel deletes a rule in the routing policy database
func (ns *NS) RuleDel(src *net.IPNet, iif string, priority int, table int) error {
	rule := netlink.NewRule()
	rule.Src = src
	rule.IifName = iif
	rule.Priority = priority
	rule.Table = table

	if err := ns.handle.RuleDel(rule); err != nil {
		return fmt.Errorf("failed to delete a rule: %w", err)
	}
	return nil
}

// RuleList gets a list of rules in the routing policy database
func (ns *NS) RuleList(src *net.IPNet, iif string, priority int, table int) ([]netlink.Rule, error) {
	rule := netlink.NewRule()
	var filterMask uint64
	if src != nil {
		rule.Src = src
		filterMask |= netlink.RT_FILTER_SRC
	}
	if iif != "" {
		rule.IifName = iif
		filterMask |= netlink.RT_FILTER_IIF
	}
	if priority != 0 {
		rule.Priority = priority
		filterMask |= netlink.RT_FILTER_PRIORITY
	}
	rules, err := ns.handle.RuleListFiltered(netlink.FAMILY_V4, rule, filterMask)
	if err != nil {
		return nil, fmt.Errorf("failed to get rules: %w", err)
	}
	return rules, nil
}

// DetectPodIP returns IP and link of the default route device
func (ns *NS) DetectPodIP() (net.IP, string, error) {
	routes, err := ns.handle.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get routes on the pod namespace: %w", err)
	}

	var defaultRoute *netlink.Route
	for _, route := range routes {
		if route.Dst == nil {
			defaultRoute = &route
			break
		}
	}
	if defaultRoute == nil {
		return nil, "", fmt.Errorf("failed to get default route on the pod namespace")
	}
	podLinks, err := ns.handle.LinkList()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get interfaces on the pod namespace: %w", err)
	}
	var defaultLink netlink.Link
	var podIP net.IP
	for _, link := range podLinks {
		if link.Attrs().Index == defaultRoute.LinkIndex {
			defaultLink = link
			addrs, err := ns.handle.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get IP address of interface %s on the pod namespace: %w", link.Attrs().Name, err)
			}
			if len(addrs) == 0 {
				return nil, "", fmt.Errorf("found no IPv4 addresses on the interface %s on the pod namespace", link.Attrs().Name)
			}
			if len(addrs) > 1 {
				return nil, "", fmt.Errorf("found multiple IPv4 addresses on the interface %s on the pod namespace", link.Attrs().Name)
			}
			podIP = addrs[0].IP
			break
		}
	}

	return podIP, defaultLink.Attrs().Name, nil
}

// VethAdd adds a veth pair
func (ns *NS) VethAdd(name string, peerNS *NS, peerName string) error {
	if err := ns.LinkAdd(name, &netlink.Veth{PeerName: peerName, PeerNamespace: netlink.NsFd(peerNS.nsHandle)}); err != nil {
		return fmt.Errorf("failed to add veth pair %s and %s: %w", name, peerName, err)
	}
	return nil
}

// VethAddPrefix adds a veth pair. One endpoint is create at ns, and its name begins with vethPrefix.
// The other endpoint is created at peerNS and its name is peerName.
func (ns *NS) VethAddPrefix(vethPrefix string, peerNS *NS, peerName string) (string, error) {
	links, err := ns.handle.LinkList()
	if err != nil {
		return "", fmt.Errorf("failed to get interfaces on host: %w", err)
	}
	index := 1
	for {
		vethName := fmt.Sprintf("%s%d", vethPrefix, index)
		var found bool
		for _, link := range links {
			if link.Attrs().Name == vethName {
				found = true
				break
			}
		}
		if !found {
			err := ns.LinkAdd(vethName, &netlink.Veth{PeerName: peerName, PeerNamespace: netlink.NsFd(peerNS.nsHandle)})
			if err == nil {
				return vethName, nil
			}
			if !errors.Is(err, os.ErrExist) {
				return "", fmt.Errorf("failed to add veth pair %s and %s: %w", vethName, peerName, err)
			}
		}
		index++
	}
}

// RedirectAdd adds a tc ingress qdisc and redirect filter that redirects all traffic from src to dst
func (ns *NS) RedirectAdd(src, dst string) error {
	srcLink, err := ns.handle.LinkByName(src)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", src, err)
	}

	dstLink, err := ns.handle.LinkByName(dst)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", dst, err)
	}

	qdisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: srcLink.Attrs().Index,
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	if err := ns.handle.QdiscAdd(qdisc); err != nil {
		return fmt.Errorf("failed to add qdisc to %s: %w", src, err)
	}

	filter := &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: srcLink.Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs: netlink.ActionAttrs{
					Action: netlink.TC_ACT_STOLEN,
				},
				MirredAction: netlink.TCA_EGRESS_REDIR,
				Ifindex:      dstLink.Attrs().Index,
			},
		},
	}

	if err := ns.handle.FilterAdd(filter); err != nil {
		return fmt.Errorf("failed to add a filter to %s : %w", src, err)
	}

	return nil
}

// RedirectDel deletes a tc ingress qdisc and redirect filters on src
func (ns *NS) RedirectDel(src string) error {
	srcLink, err := ns.handle.LinkByName(src)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", src, err)
	}

	filters, err := ns.handle.FilterList(srcLink, netlink.MakeHandle(0xffff, 0))
	if err != nil {
		return fmt.Errorf("failed to get a list of filters on %s: %w", src, err)
	}
	for _, filter := range filters {
		if _, ok := filter.(*netlink.U32); ok {
			if err := ns.handle.FilterDel(filter); err != nil {
				return fmt.Errorf("failed to delete a filter to %s : %w", src, err)
			}
		}
	}

	qdiscs, err := ns.handle.QdiscList(srcLink)
	if err != nil {
		return err
	}
	for _, qdisc := range qdiscs {
		if _, ok := qdisc.(*netlink.Ingress); ok {
			if err := ns.handle.QdiscDel(qdisc); err != nil {
				return fmt.Errorf("failed to delete a qdisc on %s : %w", src, err)
			}
		}
	}

	return nil
}
