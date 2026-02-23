// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type Namespace interface {
	Close() error
	LinkAdd(name string, device Device) (Link, error)
	LinkFind(name string) (Link, error)
	LinkList() ([]Link, error)
	Path() string
	RedirectAdd(src, dst string) error
	RedirectDel(src string) error
	RouteAdd(route *Route) error
	RouteDel(route *Route) error
	GetDefaultRoutes() ([]*Route, error)
	RouteList(filters ...*Route) ([]*Route, error)
	RuleAdd(rule *Rule) error
	RuleDel(rule *Rule) error
	RuleList(rule *Rule) ([]*Rule, error)
	NeighborAdd(neighbor *Neighbor) error
	NeighborList(filters ...*Neighbor) ([]*Neighbor, error)
	Run(fn func() error) error
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

func RunAsNsPath(nsPath string, fn func() error) (err error) {
	podNS, err := OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
	}
	defer func() {
		if err = podNS.Close(); err != nil {
			err = fmt.Errorf("failed to close a network namespace %q: %w ", podNS.Path(), err)
		}
	}()
	return podNS.Run(fn)
}

type Link interface {
	Name() string
	Namespace() Namespace
	Type() string
	Delete() error

	GetAddr() ([]netip.Prefix, error)
	AddAddr(prefix netip.Prefix) error
	GetHardwareAddr() (string, error)
	SetHardwareAddr(hwAddr string) error
	GetMTU() (int, error)
	SetMTU(mtu int) error
	GetDevice() (Device, error)

	SetMaster(master Link) error
	SetNamespace(target Namespace) error
	SetName(name string) error
	SetUp() error
}

type link struct {
	nlLink netlink.Link
	ns     *namespace
}

func (l *link) Name() string {
	return l.nlLink.Attrs().Name
}

func (l *link) Namespace() Namespace {
	return l.ns
}

func (l *link) Type() string {
	return l.nlLink.Type()
}

func (l *link) GetAddr() ([]netip.Prefix, error) {

	addrs, err := l.ns.handle.AddrList(l.nlLink, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP addresses assigned to %s interface %q:  %w", l.Type(), l.Name(), err)
	}

	var prefixes []netip.Prefix
	for _, addr := range addrs {
		prefixes = append(prefixes, toPrefix(addr.IPNet))
	}

	return prefixes, nil
}

func (l *link) AddAddr(prefix netip.Prefix) error {

	if err := l.ns.handle.AddrAdd(l.nlLink, &netlink.Addr{IPNet: toIPNet(prefix)}); err != nil {
		return fmt.Errorf("failed to assign an IP address %q to %s: %w", prefix.String(), l.Name(), err)
	}

	return nil
}

func (l *link) GetMTU() (int, error) {

	mtu := l.nlLink.Attrs().MTU

	return mtu, nil
}

func (l *link) SetMTU(mtu int) error {

	if err := l.ns.handle.LinkSetMTU(l.nlLink, mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d: %w", l.Name(), mtu, err)
	}

	return nil
}

func (l *link) GetHardwareAddr() (string, error) {

	hwAddr := l.nlLink.Attrs().HardwareAddr.String()

	return hwAddr, nil
}

func (l *link) SetHardwareAddr(hwAddr string) error {

	mac, err := net.ParseMAC(hwAddr)
	if err != nil {
		return fmt.Errorf("failed to parse hardware address %q: %w", hwAddr, err)

	}

	if err := l.ns.handle.LinkSetHardwareAddr(l.nlLink, mac); err != nil {
		return fmt.Errorf("failed to set hardware address %q to %s (netns: %s): %w", hwAddr, l.Name(), l.ns.Path(), err)
	}

	return nil
}

func (l *link) SetMaster(master Link) error {

	if err := l.ns.handle.LinkSetMaster(l.nlLink, master.(*link).nlLink); err != nil {
		return fmt.Errorf("failed to set master device of %s to %s: %w", l.Name(), l.Name(), err)
	}
	return nil
}

func (l *link) SetNamespace(target Namespace) error {

	if err := l.ns.handle.LinkSetNsFd(l.nlLink, target.(*namespace).fd()); err != nil {
		return fmt.Errorf("failed to change network namespace of interface %s from %s to %s: %w", l.Name(), l.ns.path, target.Path(), err)
	}

	l.ns = target.(*namespace)

	return nil
}

func (l *link) SetName(name string) error {

	if err := l.ns.handle.LinkSetName(l.nlLink, name); err != nil {
		return fmt.Errorf("failed to change name of interface %s on %s to %s: %w", l.Name(), l.ns.path, name, err)
	}

	return nil
}

func (l *link) SetUp() error {

	if err := l.ns.handle.LinkSetUp(l.nlLink); err != nil {
		return fmt.Errorf("failed to set link state up: %s: %w", l.Name(), err)
	}
	return nil
}

func (l *link) Delete() error {

	if err := l.ns.handle.LinkDel(l.nlLink); err != nil {
		return fmt.Errorf("failed to delete an interface of %s: %s:  %w", l.Name(), l.Type(), err)
	}
	return nil
}

func (l *link) GetDevice() (dev Device, err error) {

	switch v := l.nlLink.(type) {
	case *netlink.Vxlan:
		dev = &VXLAN{
			Group: toAddr(v.Group),
			ID:    v.VxlanId,
			Port:  v.Port,
		}
	default:
		// TODO: Support Bridge, VXLAN, ...
		return nil, fmt.Errorf("device info is not available: %s", l.nlLink.Type())
	}
	return dev, nil
}

type Device interface {
	getLink() netlink.Link
}

type VEth struct {
	PeerName      string
	PeerNamespace Namespace
}

func (d *VEth) getLink() netlink.Link {

	return &netlink.Veth{
		PeerName:      d.PeerName,
		PeerNamespace: netlink.NsFd(d.PeerNamespace.(*namespace).nsHandle),
	}
}

type Bridge struct{}

func (d *Bridge) getLink() netlink.Link {

	return &netlink.Bridge{}
}

type VXLAN struct {
	Group netip.Addr
	ID    int
	Port  int
}

func (d *VXLAN) getLink() netlink.Link {

	return &netlink.Vxlan{
		Group:   toIP(d.Group),
		VxlanId: d.ID,
		Port:    d.Port,
	}
}

func (ns *namespace) LinkFind(name string) (Link, error) {

	nlLinks, err := ns.handle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of interfaces (netns: %s): %w", ns.path, err)
	}

	for _, nlLink := range nlLinks {
		if nlLink.Attrs().Name == name {
			l := &link{
				nlLink: nlLink,
				ns:     ns,
			}
			return l, nil
		}
	}

	return nil, fmt.Errorf("failed to find interface %q on netns %s", name, ns.path)
}

func (ns *namespace) LinkList() ([]Link, error) {

	nlLinks, err := ns.handle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of interfaces: %w", err)
	}

	var links []Link

	for _, nlLink := range nlLinks {
		link := &link{
			nlLink: nlLink,
			ns:     ns,
		}
		links = append(links, link)
	}

	return links, nil
}

// LinkAdd creates a new link with an attribute specified by device
func (ns *namespace) LinkAdd(name string, device Device) (Link, error) {

	nlLink := device.getLink()
	nlLink.Attrs().Name = name

	if err := ns.handle.LinkAdd(nlLink); err != nil {
		return nil, fmt.Errorf("failed to create %s interface %q: %s:  %w", nlLink.Type(), name, ns.Path(), err)
	}

	link, err := ns.LinkFind(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find created %s interface %q on %s:  %w", nlLink.Type(), name, ns.Path(), err)
	}

	return link, err
}

var DefaultPrefix = netip.MustParsePrefix("0.0.0.0/0")

type Route struct {
	Destination netip.Prefix
	Source      netip.Addr
	Gateway     netip.Addr
	Device      string
	Priority    int
	Table       int
	Type        int
	Protocol    RouteProtocol
	Scope       RouteScope
	Onlink      bool
}

func (r1 *Route) compare(r2 *Route) bool {

	if r1.Table != r2.Table {
		return r1.Table < r2.Table
	}

	d1 := r1.Destination
	d2 := r2.Destination

	if !d1.IsValid() {
		d1 = DefaultPrefix
	}
	if !d2.IsValid() {
		d2 = DefaultPrefix
	}

	cmp := bytes.Compare(d1.Addr().AsSlice(), d2.Addr().AsSlice())
	if cmp != 0 {
		return cmp < 0
	}

	l1 := d1.Bits()
	l2 := d2.Bits()
	if l1 != l2 {
		return l1 < l2
	}

	return r1.Priority < r2.Priority
}

func (ns *namespace) routeListFiltered(filter *Route) ([]*netlink.Route, error) {

	var nlRoute netlink.Route
	var filterMask uint64

	if dst := filter.Destination; dst.IsValid() {
		if dst.Bits() > 0 {
			nlRoute.Dst = toIPNet(dst)
		}
		filterMask |= netlink.RT_FILTER_DST
	}
	if filter.Source.IsValid() {
		nlRoute.Src = toIP(filter.Source)
		filterMask |= netlink.RT_FILTER_SRC
	}
	if filter.Gateway.IsValid() {
		nlRoute.Gw = toIP(filter.Gateway)
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
			{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Protocol: unix.RTPROT_KERNEL},
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
				Destination: toPrefix(r.Dst),
				Source:      toAddr(r.Src),
				Gateway:     toAddr(r.Gw),
				Device:      dev,
				Priority:    r.Priority,
				Table:       r.Table,
				Type:        r.Type,
				Protocol:    RouteProtocol(r.Protocol),
				Scope:       RouteScope(r.Scope),
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

// Return default routes if present
func (ns *namespace) GetDefaultRoutes() ([]*Route, error) {

	routes, err := ns.RouteList(&Route{})
	if err != nil {
		return nil, fmt.Errorf("failed to get a list of routes: %w", err)
	}

	var defRoutes []*Route
	for _, r := range routes {
		if r.Destination.Bits() == 0 && r.Priority < math.MaxInt {
			defRoutes = append(defRoutes, r)
		}
	}

	return defRoutes, nil
}

// RouteAdd adds a new route
func (ns *namespace) RouteAdd(route *Route) error {

	nlRoute := &netlink.Route{
		Dst:      toIPNet(route.Destination),
		Src:      toIP(route.Source),
		Gw:       toIP(route.Gateway),
		Priority: route.Priority,
		Table:    route.Table,
		Type:     route.Type,
		Scope:    netlink.Scope(route.Scope),
		Protocol: netlink.RouteProtocol(route.Protocol),
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
	if !route.Gateway.IsValid() {
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
		return fmt.Errorf("failed to identify routes to be deleted: dest: %s, gw: %s, dev %s", route.Destination, route.Gateway, route.Device)
	}

	for _, r := range routes {
		if err := ns.handle.RouteDel(r); err != nil {
			return fmt.Errorf("failed to delete a route: dest: %s, gw: %s, dev %s: %w", route.Destination, route.Gateway, route.Device, err)
		}
	}
	return nil
}

func initLookupTable[T fmt.Stringer](s []T) map[string]T {

	m := make(map[string]T)
	for _, x := range s {
		m[x.String()] = x
	}
	return m
}

func unmarshalJSON[T any](name string, data []byte, table map[string]T) (v T, _ error) {

	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return v, fmt.Errorf("invalid %s: %v: %w", name, data, err)
	}

	v, ok := table[str]
	if !ok {
		return v, fmt.Errorf("unknown %s: %s", name, str)
	}
	return v, nil
}

type RouteProtocol netlink.RouteProtocol

var routeProtocols = initLookupTable([]RouteProtocol{
	unix.RTPROT_STATIC,
	unix.RTPROT_BOOT,
	unix.RTPROT_DHCP,
	unix.RTPROT_KERNEL,
})

func (p RouteProtocol) String() string {
	return netlink.RouteProtocol(p).String()
}

func (p RouteProtocol) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *RouteProtocol) UnmarshalJSON(data []byte) error {

	v, err := unmarshalJSON("route protocol", data, routeProtocols)
	if err != nil {
		return err
	}
	*p = v
	return nil
}

type RouteScope netlink.Scope

var routeScopes = initLookupTable([]netlink.Scope{
	netlink.SCOPE_UNIVERSE,
	netlink.SCOPE_SITE,
	netlink.SCOPE_LINK,
	netlink.SCOPE_HOST,
	netlink.SCOPE_NOWHERE,
})

func (s RouteScope) String() string {
	return netlink.Scope(s).String()
}

func (s RouteScope) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *RouteScope) UnmarshalJSON(data []byte) error {

	rs, err := unmarshalJSON("route scope", data, routeScopes)
	if err != nil {
		return err
	}
	*s = RouteScope(rs)
	return nil
}

type Rule struct {
	Src      netip.Prefix
	IifName  string
	Priority int
	Table    int
}

// RuleAdd adds a new rule in the routing policy database
func (ns *namespace) RuleAdd(rule *Rule) error {
	nlRule := netlink.NewRule()
	nlRule.Src = toIPNet(rule.Src)
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
	nlRule.Src = toIPNet(rule.Src)
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
	if rule.Src.IsValid() {
		nlRule.Src = toIPNet(rule.Src)
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
			Src:      toPrefix(nlRule.Src),
			IifName:  nlRule.IifName,
			Priority: nlRule.Priority,
			Table:    nlRule.Table,
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

type Neighbor struct {
	IP           netip.Addr
	HardwareAddr string
	Dev          string
	State        NeighborState
}

type NeighborState uint16

const NeighborStatePermanent = netlink.NUD_PERMANENT

var neighorStateMap = map[NeighborState]string{
	netlink.NUD_NONE:       "none",
	netlink.NUD_INCOMPLETE: "incomplete",
	netlink.NUD_REACHABLE:  "reachable",
	netlink.NUD_STALE:      "stale",
	netlink.NUD_DELAY:      "delay",
	netlink.NUD_PROBE:      "probe",
	netlink.NUD_FAILED:     "failed",
	netlink.NUD_NOARP:      "noarp",
	netlink.NUD_PERMANENT:  "permanent",
}

func (s NeighborState) String() string {

	str, ok := neighorStateMap[s]
	if !ok {
		return "unknown"
	}
	return str
}

var neighborStates = initLookupTable(maps.Keys(neighorStateMap))

func (s NeighborState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *NeighborState) UnmarshalJSON(data []byte) error {

	v, err := unmarshalJSON("neighbor state", data, neighborStates)
	if err != nil {
		return err
	}
	*s = v
	return nil
}

// NeighborAdd adds a new neighbor entry
func (ns *namespace) NeighborAdd(neighbor *Neighbor) error {

	if !neighbor.IP.IsValid() {
		return fmt.Errorf("failed to add a new neighbor entry: invalid IP address %q", neighbor.IP)
	}

	if neighbor.Dev == "" {
		return fmt.Errorf("failed to add a new neighbor entry: no device name")
	}

	link, err := ns.handle.LinkByName(neighbor.Dev)
	if err != nil {
		return fmt.Errorf("failed to add a new neighbor entry: device %s not found: %w", neighbor.Dev, err)
	}

	hwAddr, err := net.ParseMAC(neighbor.HardwareAddr)
	if err != nil {
		return fmt.Errorf("failed to add a new neighbor entry: invalid hardware address %q", neighbor.HardwareAddr)
	}

	state := neighbor.State
	if state.String() == "none" || state.String() == "unknown" {
		return fmt.Errorf("failed to add a new neighbor entry: invalid neighbor state %q", state.String())
	}

	nlNeigh := &netlink.Neigh{
		IP:           toIP(neighbor.IP),
		LinkIndex:    link.Attrs().Index,
		HardwareAddr: hwAddr,
		State:        int(state),
	}

	if err := ns.handle.NeighAdd(nlNeigh); err != nil {
		return fmt.Errorf("failed to add a new neighbor entry %q: %w", nlNeigh, err)
	}

	return nil
}

// NeighborList gets a list of neighbors
func (ns *namespace) NeighborList(filters ...*Neighbor) ([]*Neighbor, error) {

	var neighbors []*Neighbor
	for _, filter := range filters {

		if filter.State.String() == "unknown" {
			return nil, fmt.Errorf("unknown neighbor state 0x%x is specified", filter.State)
		}

		msg := netlink.Ndmsg{
			Family: netlink.FAMILY_V4,
			State:  uint16(filter.State),
		}

		if filter.Dev != "" {
			link, err := ns.handle.LinkByName(filter.Dev)
			if err != nil {
				return nil, fmt.Errorf("failed to get a dev link named %s on network namespace %s: %w", filter.Dev, ns.Path(), err)
			}
			msg.Index = uint32(link.Attrs().Index)
		}
		nlNeighbors, err := ns.handle.NeighListExecute(msg)
		if err != nil {
			return nil, err
		}

		for _, nlNeigh := range nlNeighbors {

			link, err := ns.handle.LinkByIndex(nlNeigh.LinkIndex)
			if err != nil {
				return nil, fmt.Errorf("failed to get a link with index %d of neighbor %s: %w", nlNeigh.LinkIndex, nlNeigh.String(), err)
			}
			dev := link.Attrs().Name

			state := NeighborState(nlNeigh.State)
			if state.String() == "unknown" {
				return nil, fmt.Errorf("unknown neighbor state 0x%x found in entry %q", nlNeigh.State, nlNeigh.String())
			}

			n := &Neighbor{
				IP:           toAddr(nlNeigh.IP),
				Dev:          dev,
				HardwareAddr: nlNeigh.HardwareAddr.String(),
				State:        state,
			}
			neighbors = append(neighbors, n)
		}
	}

	return neighbors, nil
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

func toAddr(ip net.IP) netip.Addr {

	addr, _ := netip.AddrFromSlice(ip)

	return addr
}

func toPrefix(ipnet *net.IPNet) netip.Prefix {

	if ipnet == nil {
		return netip.PrefixFrom(netip.IPv4Unspecified(), 0)
	}

	addr, _ := netip.AddrFromSlice(ipnet.IP)
	ones, _ := ipnet.Mask.Size()

	return netip.PrefixFrom(addr, ones)
}

func toIP(addr netip.Addr) net.IP {

	if !addr.IsValid() {
		return nil
	}
	return addr.AsSlice()
}

func toIPNet(prefix netip.Prefix) *net.IPNet {

	if !prefix.IsValid() {
		return nil
	}
	addr := prefix.Addr()

	return &net.IPNet{
		IP:   net.IP(addr.AsSlice()),
		Mask: net.CIDRMask(prefix.Bits(), addr.BitLen()),
	}
}
