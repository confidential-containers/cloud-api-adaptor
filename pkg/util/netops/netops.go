// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type Namespace interface {
	AddrAdd(name string, addr *net.IPNet) error
	Close() error
	GetHardwareAddr(name string) (string, error)
	GetIP(name string) ([]net.IP, error)
	GetIPNet(name string) ([]*net.IPNet, error)
	GetMTU(name string) (int, error)
	LinkAdd(name string, link netlink.Link) error
	LinkDel(name string) error
	LinkList() ([]string, error)
	LinkNameByAddr(ip net.IP) (string, error)
	LinkSetMaster(name, masterName string) error
	LinkSetNS(name string, targetNS Namespace) error
	LinkSetName(name, newName string) error
	LinkSetUp(name string) error
	Path() string
	RedirectAdd(src, dst string) error
	RedirectDel(src string) error
	RouteAdd(route *Route) error
	RouteDel(route *Route) error
	RouteList(filters ...*Route) ([]*Route, error)
	RuleAdd(rule *Rule) error
	RuleDel(rule *Rule) error
	RuleList(rule *Rule) ([]*Rule, error)
	Run(fn func() error) error
	SetHardwareAddr(name, hwAddr string) error
	SetMTU(name string, mtu int) error
	VethAdd(name string, peerNS Namespace, peerName string) error
}

type namespace struct {
	handle   *netlink.Handle
	path     string
	nsHandle netns.NsHandle
}

// OpenNamespace returns a namespace specified by a path
func OpenNamespace(nsPath string) (Namespace, error) {

	nsHandle, err := netns.GetFromPath(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get network namespace %s: %w", nsPath, err)
	}

	handle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get a handle for network namespace %s: %w", nsPath, err)
	}

	ns := &namespace{
		path:     nsPath,
		nsHandle: nsHandle,
		handle:   handle,
	}

	return ns, nil
}

// OpenCurrentNamespace returns the current network namespace
func OpenCurrentNamespace() (Namespace, error) {

	pid := os.Getpid()
	tid := unix.Gettid()
	path := fmt.Sprintf("/proc/%d/task/%d/ns/net", pid, tid)

	return OpenNamespace(path)
}

// CreateNamedNamespace creates a new named network namespace, and returns its path
func CreateNamedNamespace(name string) (string, error) {
	runtime.LockOSThread()
	defer runtime.LockOSThread()

	old, err := netns.Get()
	if err != nil {
		return "", fmt.Errorf("failed to get the current network namespace: %w", err)
	}
	defer func() {
		if e := netns.Set(old); e != nil {
			err = fmt.Errorf("failed to set back to the old network namespace: %w (previous error %v)", e, err)
		}
	}()

	nsHandle, err := netns.NewNamed(name)
	if err != nil {
		return "", fmt.Errorf("failed to create a new named network namespace: %w", err)
	}
	if err := nsHandle.Close(); err != nil {
		return "", fmt.Errorf("failed to close a new named network namespace: %w", err)
	}

	return filepath.Join("/run/netns", name), err
}

// DeleteNamedNamespace deletes a named NS
func DeleteNamedNamespace(name string) error {
	if err := netns.DeleteNamed(name); err != nil {
		return fmt.Errorf("failed to delete a named network namespace %s: %w", name, err)
	}
	return nil
}

func (ns *namespace) Path() string {
	return ns.path
}

func (ns *namespace) fd() int {
	return int(ns.nsHandle)
}

// Close closes an NS
func (ns *namespace) Close() error {

	ns.handle.Close()

	if err := ns.nsHandle.Close(); err != nil {
		return fmt.Errorf("failed to close network namespace %v: %w", ns.handle, err)
	}

	return nil
}

// Run calls a function in a network namespace
func (ns *namespace) Run(fn func() error) (err error) {
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
		if err = netns.Set(ns.nsHandle); err != nil {
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

// GetIP returns a list of IP addresses assigned to a link
func (ns *namespace) GetIP(linkName string) ([]net.IP, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	addrs, err := ns.handle.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to get addressess assigned to %s (type: %s, netns: %s): %w", linkName, link.Type(), ns.path, err)
	}

	var ips []net.IP
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}

	return ips, nil
}

// GetIPNet returns a list of IPNets assigned to a link
func (ns *namespace) GetIPNet(linkName string) ([]*net.IPNet, error) {

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
func (ns *namespace) GetMTU(linkName string) (int, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return 0, fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	mtu := link.Attrs().MTU

	return mtu, nil
}

// SetMTU sets MTU size of a link
func (ns *namespace) SetMTU(linkName string, mtu int) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d: %w", linkName, mtu, err)
	}

	return nil
}

func (ns *namespace) GetHardwareAddr(linkName string) (string, error) {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return "", fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	hwAddr := link.Attrs().HardwareAddr.String()

	return hwAddr, nil
}

func (ns *namespace) SetHardwareAddr(linkName, hwAddr string) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	mac, err := net.ParseMAC(hwAddr)
	if err != nil {
		return fmt.Errorf("failed to parse hardware address %q: %w", hwAddr, err)

	}

	if err := ns.handle.LinkSetHardwareAddr(link, mac); err != nil {
		return fmt.Errorf("failed to set hardware address %q to %s (netns: %s): %w", hwAddr, linkName, ns.Path(), err)
	}

	return nil
}

func (ns *namespace) LinkNameByAddr(ip net.IP) (string, error) {

	links, err := ns.handle.LinkList()
	if err != nil {
		return "", fmt.Errorf("failed to get a list of interfaces (netns: %s): %w", ns.Path(), err)
	}

	var names []string
	for _, link := range links {
		name := link.Attrs().Name
		addrs, err := ns.GetIP(name)
		if err != nil {
			return "", fmt.Errorf("failed to obtain addresses assigned to %s (netns: %s)", name, ns.Path())
		}
		for _, addr := range addrs {
			if addr.Equal(ip) {
				names = append(names, name)
				break
			}
		}
	}

	if len(names) == 0 {
		return "", fmt.Errorf("failed to find interface that has %s on netns %s", ip.String(), ns.Path())
	}
	if len(names) > 1 {
		return "", fmt.Errorf("multiple interfaces have %s on netns %s: %s", ip.String(), ns.Path(), strings.Join(names, ", "))
	}

	return names[0], nil
}

func (ns *namespace) LinkList() ([]string, error) {

	links, err := ns.handle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of interfaces: %w", err)
	}

	var names []string

	for _, link := range links {
		names = append(names, link.Attrs().Name)
	}

	return names, nil
}

// LinkAdd creates a new link with an attribute specified by link
func (ns *namespace) LinkAdd(linkName string, link netlink.Link) error {
	attrs := link.Attrs()
	*attrs = netlink.NewLinkAttrs()
	attrs.Name = linkName
	if err := ns.handle.LinkAdd(link); err != nil {
		return fmt.Errorf("failed to create an interface of %s: %s:  %w", link.Type(), linkName, err)
	}
	return nil
}

// LinkDel deletes a link
func (ns *namespace) LinkDel(linkName string) error {
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
func (ns *namespace) LinkSetMaster(linkName, masterName string) error {
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
func (ns *namespace) LinkSetNS(linkName string, targetNS Namespace) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetNsFd(link, targetNS.(*namespace).fd()); err != nil {
		return fmt.Errorf("failed to change network namespace of interface %s from %s to %s: %w", linkName, ns.path, targetNS.Path(), err)
	}

	return nil
}

// LinkSetName changes name of a link
func (ns *namespace) LinkSetName(linkName, newName string) error {

	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}

	if err := ns.handle.LinkSetName(link, newName); err != nil {
		return fmt.Errorf("failed to change name of interface %s on %s to %s: %w", linkName, ns.path, newName, err)
	}

	return nil
}

// AddrAdd adds an IP address to a link
func (ns *namespace) AddrAdd(linkName string, addr *net.IPNet) error {
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
func (ns *namespace) LinkSetUp(linkName string) error {
	link, err := ns.handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", linkName, err)
	}
	if err := ns.handle.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to set link state up: %s: %w", linkName, err)
	}
	return nil
}

type Route struct {
	Destination *net.IPNet
	Source      net.IP
	Gateway     net.IP
	Device      string
	Priority    int
	Table       int
	Type        int
	Protocol    int
	Onlink      bool
}

func (r1 *Route) compare(r2 *Route) bool {

	if r1.Table != r2.Table {
		return r1.Table < r2.Table
	}

	d1 := r1.Destination
	d2 := r2.Destination

	if !(d1 == nil && d2 == nil) {
		if d1 == nil {
			return true
		}
		if d2 == nil {
			return false
		}

		cmp := bytes.Compare(d1.IP.To4(), d2.IP.To4())
		if cmp != 0 {
			return cmp < 0
		}

		l1, _ := d1.Mask.Size()
		l2, _ := d2.Mask.Size()
		if l1 != l2 {
			return l1 < l2
		}
	}

	return r1.Priority < r2.Priority
}

func (ns *namespace) routeListFiltered(filter *Route) ([]*netlink.Route, error) {

	var nlRoute netlink.Route
	var filterMask uint64

	if dst := filter.Destination; dst != nil {
		if ones, bits := dst.Mask.Size(); ones > 0 && bits > 0 {
			nlRoute.Dst = dst
		}
		filterMask |= netlink.RT_FILTER_DST
	}
	if filter.Source != nil {
		nlRoute.Src = filter.Source
		filterMask |= netlink.RT_FILTER_SRC
	}
	if filter.Gateway != nil {
		nlRoute.Gw = filter.Gateway
		filterMask |= netlink.RT_FILTER_GW
	}
	if dev := filter.Device; dev != "" {
		link, err := ns.handle.LinkByName(dev)
		if err != nil {
			return nil, fmt.Errorf("failed to get interface %s: %w", dev, err)
		}
		nlRoute.LinkIndex = link.Attrs().Index
		filterMask |= netlink.RT_FILTER_OIF
	}
	if filter.Table != 0 {
		nlRoute.Table = filter.Table
		filterMask |= netlink.RT_FILTER_TABLE
	}
	if filter.Type != 0 {
		nlRoute.Type = filter.Type
		filterMask |= netlink.RT_FILTER_TYPE
	}
	if filter.Protocol != 0 {
		nlRoute.Protocol = netlink.RouteProtocol(filter.Protocol)
		filterMask |= netlink.RT_FILTER_PROTOCOL
	}

	list, err := ns.handle.RouteListFiltered(netlink.FAMILY_V4, &nlRoute, filterMask)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path(), err)
	}

	var nlRoutes []*netlink.Route
	for _, r := range list {
		r := r
		nlRoutes = append(nlRoutes, &r)
	}

	return nlRoutes, nil
}

// RouteList gets a list of routes on the main table
func (ns *namespace) RouteList(filters ...*Route) ([]*Route, error) {

	if len(filters) == 0 {
		defaultFilters := []*Route{
			{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Protocol: unix.RTPROT_STATIC},
			{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Protocol: unix.RTPROT_BOOT},
			{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Protocol: unix.RTPROT_DHCP},
		}
		filters = append(filters, defaultFilters...)
	}

	var routes []*Route
	for _, filter := range filters {
		nlRoutes, err := ns.routeListFiltered(filter)
		if err != nil {
			return nil, err
		}

		for _, r := range nlRoutes {

			var dev string
			if r.LinkIndex > 0 {
				link, err := ns.handle.LinkByIndex(r.LinkIndex)
				if err != nil {
					return nil, fmt.Errorf("failed to get a link with index %d of a route: %w", r.LinkIndex, err)
				}
				dev = link.Attrs().Name
			}

			onlink := r.Flags&int(netlink.FLAG_ONLINK) != 0

			route := &Route{
				Destination: r.Dst,
				Source:      r.Src,
				Gateway:     r.Gw,
				Device:      dev,
				Priority:    r.Priority,
				Table:       r.Table,
				Type:        r.Type,
				Protocol:    int(r.Protocol),
				Onlink:      onlink,
			}

			routes = append(routes, route)
		}
	}

	sort.SliceStable(routes, func(i, j int) bool {
		return routes[i].compare(routes[j])
	})

	return routes, nil
}

// RouteAdd adds a new route
func (ns *namespace) RouteAdd(route *Route) error {

	nlRoute := &netlink.Route{
		Dst:      route.Destination,
		Src:      route.Source,
		Gw:       route.Gateway,
		Priority: route.Priority,
		Table:    route.Table,
		Type:     route.Type,
	}

	if route.Device != "" {
		link, err := ns.handle.LinkByName(route.Device)
		if err != nil {
			return fmt.Errorf("failed to get interface %s: %w", route.Device, err)
		}
		nlRoute.LinkIndex = link.Attrs().Index
	}
	if route.Onlink {
		nlRoute.Flags = int(netlink.FLAG_ONLINK)
	}
	if route.Gateway == nil {
		nlRoute.Scope = netlink.SCOPE_LINK
	}
	if err := ns.handle.RouteAdd(nlRoute); err != nil {
		return fmt.Errorf("failed to create a route (table: %d, dest: %s, gw: %s) with flags %d: %w", nlRoute.Table, nlRoute.Dst.String(), nlRoute.Gw.String(), nlRoute.Flags, err)
	}
	return nil
}

// RouteDel deletes routes
func (ns *namespace) RouteDel(route *Route) error {

	routes, err := ns.routeListFiltered(route)
	if err != nil {
		return fmt.Errorf("failed to get a list of routes: dst: %s, gw: %s, dev %s: %w", route.Destination, route.Gateway, route.Device, err)
	}

	if len(routes) == 0 {
		return fmt.Errorf("failed to identify routes to be deleted: dest: %s, gw: %s, dev %s: %w", route.Destination, route.Gateway, route.Device, err)
	}

	for _, r := range routes {
		if err := ns.handle.RouteDel(r); err != nil {
			return fmt.Errorf("failed to delete a route: dest: %s, gw: %s, dev %s: %w", route.Destination, route.Gateway, route.Device, err)
		}
	}
	return nil
}

type Rule struct {
	Src      *net.IPNet
	IifName  string
	Priority int
	Table    int
}

// RuleAdd adds a new rule in the routing policy database
func (ns *namespace) RuleAdd(rule *Rule) error {
	nlRule := netlink.NewRule()
	nlRule.Src = rule.Src
	nlRule.IifName = rule.IifName
	nlRule.Priority = rule.Priority
	nlRule.Table = rule.Table

	if err := ns.handle.RuleAdd(nlRule); err != nil {
		return fmt.Errorf("failed to add a rule: %w", err)
	}
	return nil
}

// RuleDel deletes a rule in the routing policy database
func (ns *namespace) RuleDel(rule *Rule) error {
	nlRule := netlink.NewRule()
	nlRule.Src = rule.Src
	nlRule.IifName = rule.IifName
	nlRule.Priority = rule.Priority
	nlRule.Table = rule.Table

	if err := ns.handle.RuleDel(nlRule); err != nil {
		return fmt.Errorf("failed to delete a rule: %w", err)
	}
	return nil
}

// RuleList gets a list of rules in the routing policy database
func (ns *namespace) RuleList(rule *Rule) ([]*Rule, error) {
	nlRule := netlink.NewRule()
	var filterMask uint64
	if rule.Src != nil {
		nlRule.Src = rule.Src
		filterMask |= netlink.RT_FILTER_SRC
	}
	if rule.IifName != "" {
		nlRule.IifName = rule.IifName
		filterMask |= netlink.RT_FILTER_IIF
	}
	if rule.Priority != 0 {
		nlRule.Priority = rule.Priority
		filterMask |= netlink.RT_FILTER_PRIORITY
	}
	nlRules, err := ns.handle.RuleListFiltered(netlink.FAMILY_V4, nlRule, filterMask)
	if err != nil {
		return nil, fmt.Errorf("failed to get rules: %w", err)
	}

	var rules []*Rule
	for _, nlRule := range nlRules {
		rule := &Rule{
			Src:      nlRule.Src,
			IifName:  nlRule.IifName,
			Priority: nlRule.Priority,
			Table:    nlRule.Table,
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// VethAdd adds a veth pair
func (ns *namespace) VethAdd(name string, peerNS Namespace, peerName string) error {
	if err := ns.LinkAdd(name, &netlink.Veth{PeerName: peerName, PeerNamespace: netlink.NsFd(peerNS.(*namespace).nsHandle)}); err != nil {
		return fmt.Errorf("failed to add veth pair %s and %s: %w", name, peerName, err)
	}
	return nil
}

// RedirectAdd adds a tc ingress qdisc and redirect filter that redirects all traffic from src to dst
func (ns *namespace) RedirectAdd(src, dst string) error {
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
func (ns *namespace) RedirectDel(src string) error {
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
			if err = ns.handle.FilterDel(filter); err != nil {
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
