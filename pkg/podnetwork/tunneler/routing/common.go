// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"fmt"
	"net"

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
