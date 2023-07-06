// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
)

const (
	localTableOriginalPriority = 0
	localTableNewPriority      = 32765
	podTablePriority           = 0
)

func mask32(ip netip.Prefix) netip.Prefix {
	return netip.PrefixFrom(ip.Addr(), ip.Addr().BitLen())
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

func findLinkByAddr(ns netops.Namespace, addr netip.Addr) (netops.Link, error) {

	links, err := ns.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces netns %s", ns.Path())
	}
	var foundLinks []netops.Link
	for _, link := range links {
		ips, err := link.GetAddr()
		if err != nil {
			return nil, fmt.Errorf("failed to get IP addresses assigned to %q on netns %s", link.Name(), ns.Path())
		}
		for _, ip := range ips {
			if ip.Addr() == addr {
				foundLinks = append(foundLinks, link)
				break
			}
		}
	}

	if len(foundLinks) == 0 {
		return nil, fmt.Errorf("failed to find interface that has %s on netns %s", addr.String(), ns.Path())
	}
	if len(foundLinks) > 1 {
		var names []string
		for _, link := range foundLinks {
			names = append(names, link.Name())
		}
		return nil, fmt.Errorf("multiple interfaces have %s on netns %s: %s", addr.String(), ns.Path(), strings.Join(names, ", "))
	}

	return foundLinks[0], nil
}
