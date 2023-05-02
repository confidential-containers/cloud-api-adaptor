// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"fmt"
	"net"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
)

const (
	localTableOriginalPriority = 0
	localTableNewPriority      = 32765
	podTablePriority           = 0
)

func mask32(ip net.IP) *net.IPNet {
	return &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(8*net.IPv4len, 8*net.IPv4len),
	}
}

func sysctlSet(ns netops.Namespace, key string, val string) error {

	err := ns.Run(func() error {
		if _, err := sysctl.Sysctl(key, val); err != nil {
			return fmt.Errorf("failed to set sysctl parameter %q to %q: %w", key, val, err)
		}
		return nil
	})
	return err
}

func findLinkByAddr(ns netops.Namespace, ip net.IP) (netops.Link, error) {

	links, err := ns.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces netns %s", ns.Path())
	}
	var foundLinks []netops.Link
	for _, link := range links {
		ipNets, err := link.GetAddr()
		if err != nil {
			return nil, fmt.Errorf("failed to get IP addresses assigned to %q on netns %s", link.Name(), ns.Path())
		}
		for _, ipNet := range ipNets {
			if ipNet.IP.Equal(ip) {
				foundLinks = append(foundLinks, link)
				break
			}
		}
	}

	if len(foundLinks) == 0 {
		return nil, fmt.Errorf("failed to find interface that has %s on netns %s", ip.String(), ns.Path())
	}
	if len(foundLinks) > 1 {
		var names []string
		for _, link := range foundLinks {
			names = append(names, link.Name())
		}
		return nil, fmt.Errorf("multiple interfaces have %s on netns %s: %s", ip.String(), ns.Path(), strings.Join(names, ", "))
	}

	return foundLinks[0], nil
}
