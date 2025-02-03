// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"log"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"unicode"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[podnetwork] ", log.LstdFlags|log.Lmsgprefix)

func init() {
	tunneler.Register("vxlan", vxlan.NewWorkerNodeTunneler, vxlan.NewPodNodeTunneler)
}

// extractInterfaceNumber splits the interface name into prefix and numeric parts
// Example ens1 => ens, 1
// eth10 => eth, 10
func extractInterfaceNumber(iface string) (prefix string, num int) {
	// Find the index where the first digit appears
	idx := strings.IndexFunc(iface, unicode.IsDigit)

	if idx == -1 {
		// No numeric part, return the full name as prefix with highest int value
		return iface, math.MaxInt
	}

	// Split into prefix and numeric part
	prefix = iface[:idx]
	num, err := strconv.Atoi(iface[idx:])
	if err != nil {
		// If conversion fails, default to highest int value
		// This is safer than returning 0, as eth0, ens0 is a valid interface number
		num = math.MaxInt
	}

	return prefix, num
}

// findPrimaryInterface identifies the primary interface on the given network namespace.
// An interface is considered to be primary if it is attached to the default route.
func findPrimaryInterface(ns netops.Namespace) (string, error) {
	routes, err := ns.RouteList(&netops.Route{Destination: netops.DefaultPrefix})
	if err != nil {
		return "", fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path(), err)
	}

	var primaryDev string
	var primaryPrefix string
	var primaryNum = math.MaxInt
	var priority = math.MaxInt

	for _, r := range routes {
		// Default route check
		if r.Destination.Bits() == 0 && r.Priority < priority {
			dev := r.Device
			devPrefix, devNum := extractInterfaceNumber(dev)

			// If we haven't selected any device yet, or:
			// The prefix matches the current primary one and has a lower number
			// We expect the routes to be on same interface types on the pod VM
			// eg. ens0/ens1 or eth0/eth1 etc.
			if primaryDev == "" || (devPrefix == primaryPrefix && devNum < primaryNum) {
				primaryDev = dev
				primaryPrefix = devPrefix
				primaryNum = devNum
			}
		}
	}

	if primaryDev == "" {
		return "", fmt.Errorf("failed to identify primary interface on network namespace %q", ns.Path())
	}

	logger.Printf("Primary interface: %s", primaryDev)

	return primaryDev, nil
}

func setupExternalNetwork(hostNS netops.Namespace, hostPrimaryInterface string, podNS netops.Namespace) error {

	// Get the secondary interface details
	secIface, secAddrCIDR, secRoute, err := getSecondaryInterfaceDetails(hostNS, hostPrimaryInterface)
	if err != nil {
		return fmt.Errorf("failed to get secondary interface details: %w", err)
	}

	// Move the secondary interface to the pod network namespace
	err = moveInterfaceToNamespace(hostNS, podNS, secIface, secAddrCIDR, secRoute)
	if err != nil {
		return err
	}

	return nil

}

func getInterfaceDetails(ns netops.Namespace, iface string) (
	addrCIDR netip.Prefix, defRoute *netops.Route, err error) {

	link, err := ns.LinkFind(iface)
	if err != nil {
		return netip.Prefix{}, nil, fmt.Errorf("failed to find link %q: %w", iface, err)
	}

	addrs, err := link.GetAddr()
	if err != nil {
		return netip.Prefix{}, nil, fmt.Errorf("failed to get addresses for link %q: %w", link.Name(), err)
	}

	for _, a := range addrs {
		if a.Addr().Is4() {
			addrCIDR = a
			break
		}
	}

	routes, err := ns.RouteList(&netops.Route{Destination: netops.DefaultPrefix})
	if err != nil {
		return netip.Prefix{}, nil, fmt.Errorf("failed to get routes on namespace %q: %w", ns.Path(), err)
	}

	for _, r := range routes {
		if r.Device == iface && r.Destination.Bits() == 0 && r.Priority < math.MaxInt {
			defRoute = r
			break
		}
	}

	fmt.Printf("Interface %q IPv4 address %q and default route %v\n", iface, addrCIDR, defRoute)

	return addrCIDR, defRoute, nil
}

func getSecondaryInterfaceDetails(ns netops.Namespace, primaryInterface string) (
	secIface string, secAddrCIDR netip.Prefix, secRoute *netops.Route, err error) {

	links, err := ns.LinkList()
	if err != nil {
		return "", netip.Prefix{}, nil, fmt.Errorf("failed to list links in namespace %q: %w", ns.Path(), err)
	}

	for _, link := range links {
		if isInterfaceFilteredOut(link.Name()) || link.Name() == primaryInterface {
			continue
		}

		addr, route, err := getInterfaceDetails(ns, link.Name())
		if err != nil {
			return "", netip.Prefix{}, nil, err
		}

		// The first interface other than the primary having an IPv4 address and a default route
		// or an IPbv4 address in the same subnet as the primary interface with no default route
		// is considered the secondary interface.
		if addr.IsValid() && route != nil {
			secIface = link.Name()
			secAddrCIDR = addr
			secRoute = route
			fmt.Printf("Secondary interface %q found with IPv4 address %q\n", secIface, secAddrCIDR)
			break
		}

		// If route == nil, then check if the addr is in the same subnet as the primary interface
		// Use default route for the primary as the route for the secondary
		if addr.IsValid() && route == nil {
			fmt.Printf("Route is nil, checking if %q is in the same subnet as %q\n", addr, primaryInterface)
			priAddrCIDR, defRoute, err := getInterfaceDetails(ns, primaryInterface)
			if err != nil {
				return "", netip.Prefix{}, nil, err
			}
			if priAddrCIDR.Bits() == addr.Bits() {
				secIface = link.Name()
				secAddrCIDR = addr
				secRoute = &netops.Route{
					Destination: defRoute.Destination,
					Gateway:     defRoute.Gateway,
					Device:      secIface,
					// Change the Route.Device to the secondary interface
				}
				fmt.Printf("Secondary interface %q found with IPv4 address %q\n", secIface, secAddrCIDR)
				break
			}
		}
	}

	return secIface, secAddrCIDR, secRoute, nil
}

// isInterfaceFilteredOut filters out interface names that begin with certain prefixes.
// This is used to ignore virtual, loopback, and other non-relevant interfaces.
//
// The prefixes to filter out are:
// "veth", "lo", "docker", "podman", "br-", "cni", "tunl", "tun", "tap"
var allowedPrefixes = []string{
	"veth", "lo", "docker", "podman", "br-", "cni", "tunl", "tun", "tap",
}

func isInterfaceFilteredOut(ifName string) bool {

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(ifName, prefix) {
			return true
		}
	}

	return false
}

// Function to move the network interface to a different network namespace
// Also set the default route and address for the interface in the new namespace

func moveInterfaceToNamespace(srcNs, dstNs netops.Namespace, iface string, addrCIDR netip.Prefix, route *netops.Route) error {

	// Get the network interface object
	link, err := srcNs.LinkFind(iface)
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", iface, err)
	}

	// Move the network interface to the new namespace
	err = link.SetNamespace(dstNs)
	if err != nil {
		return fmt.Errorf("failed to move link %q to namespace %q: %w", iface, dstNs.Path(), err)
	}

	// Set the address for the network interface in the new namespace
	err = link.AddAddr(addrCIDR)
	if err != nil {
		return fmt.Errorf("failed to set address %q for link %q: %w", addrCIDR, iface, err)
	}

	// Bring up the network interface in the new namespace
	err = link.SetUp()
	if err != nil {
		return fmt.Errorf("failed to bring up link %q: %w", iface, err)
	}

	// Get existing default route in the new namespace
	defRoutes, err := dstNs.GetDefaultRoutes()
	if err != nil {
		return fmt.Errorf("failed to get default route in namespace %q: %w", dstNs.Path(), err)
	}

	// Delete existing default route in the new namespace
	for _, r := range defRoutes {
		err = dstNs.RouteDel(r)
		if err != nil {
			return fmt.Errorf("failed to delete default route %v in namespace %q: %w", r, dstNs.Path(), err)
		}
	}

	// Set the default route for the network interface in the new namespace
	err = dstNs.RouteAdd(route)
	if err != nil {
		return fmt.Errorf("failed to set route %v for link %q: %w", route, iface, err)
	}

	return nil
}
