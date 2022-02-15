// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"net"
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
