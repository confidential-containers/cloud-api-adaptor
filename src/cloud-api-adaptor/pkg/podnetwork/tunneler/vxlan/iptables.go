// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/go-iptables/iptables"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

const (
	iptablesOutputChainName     = "peerpod-OUTPUT"
	iptablesPreRoutingChainName = "peerpod-PREROUTING"
)

type iptablesRule struct {
	table string
	base  string
	chain string
	spec  []string
}

func iptablesRules(addr, port, id string) []*iptablesRule {

	comment := fmt.Sprintf("peerpod [vni:%s]", id)

	return []*iptablesRule{
		{
			table: "raw",
			base:  "OUTPUT",
			chain: iptablesOutputChainName,
			spec: []string{
				"-m", "comment", "--comment", comment,
				"-d", addr,
				"-p", "udp", "-m", "udp", "--dport", port,
				"-j", "NOTRACK",
			},
		},
		{
			table: "raw",
			base:  "PREROUTING",
			chain: iptablesPreRoutingChainName,
			spec: []string{
				"-m", "comment", "--comment", comment,
				"-s", addr,
				"-p", "udp", "-m", "udp", "--dport", port,
				"-j", "NOTRACK",
			},
		},
	}
}

var iptablesMutex sync.Mutex

func iptablesSetup(ns netops.Namespace, dstAddr netip.Addr, dstPort, vxlanID int) error {

	iptablesMutex.Lock()
	defer iptablesMutex.Unlock()

	addr := dstAddr.String()
	port := strconv.Itoa(dstPort)
	id := strconv.Itoa(vxlanID)

	return ns.Run(func() error {

		ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4))
		if err != nil {
			return fmt.Errorf("failed to initialize iptables: %w", err)
		}

		for _, rule := range iptablesRules(addr, port, id) {

			exists, err := ipt.ChainExists(rule.table, rule.chain)
			if err != nil {
				return fmt.Errorf("failed to check the existence of iptables chain %q: %w", rule.chain, err)
			}

			if !exists {
				// Add "-N <chain>"
				if err := ipt.NewChain(rule.table, rule.chain); err != nil {
					return fmt.Errorf("failed to create iptables chain %s on table %s: %w", rule.chain, rule.table, err)
				}
				// Add "-A <base> -j <chain>"
				if err := ipt.AppendUnique(rule.table, rule.base, "-j", rule.chain); err != nil {
					return fmt.Errorf("failed to add iptables rule \"-t %s -A %s -j %s\": %w", rule.table, rule.chain, rule.chain, err)
				}
			}

			if err := ipt.AppendUnique(rule.table, rule.chain, rule.spec...); err != nil {
				return fmt.Errorf("failed to add iptables rule \"-t %s -A %s %s\": %w", rule.table, rule.chain, strings.Join(rule.spec, " "), err)
			}
		}

		return nil
	})
}

func iptablesTeardown(ns netops.Namespace, dstAddr netip.Addr, dstPort, vxlanID int) error {

	iptablesMutex.Lock()
	defer iptablesMutex.Unlock()

	addr := dstAddr.String()
	port := strconv.Itoa(dstPort)
	id := strconv.Itoa(vxlanID)

	return ns.Run(func() error {

		ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4))
		if err != nil {
			return fmt.Errorf("failed to initialize iptables: %w", err)
		}

		for _, rule := range iptablesRules(addr, port, id) {

			if err := ipt.Delete(rule.table, rule.chain, rule.spec...); err != nil {
				return fmt.Errorf("failed to delete iptables rule \"-t %s -A %s %s\": %w", rule.table, rule.chain, strings.Join(rule.spec, " "), err)
			}

			list, err := ipt.List(rule.table, rule.chain)
			if err != nil {
				return fmt.Errorf("failed to list rules in chain %s on table %s: %w", rule.chain, rule.table, err)
			}

			if len(list) > 1 {
				// There are remaining rules other than "-N <chain>"
				continue
			}

			// Delete "-A <base> -j <chain>"
			if err := ipt.DeleteIfExists(rule.table, rule.base, "-j", rule.chain); err != nil {
				return fmt.Errorf("failed to delete iptables rule \"-t %s -A %s -j %s\": %w", rule.table, rule.chain, rule.chain, err)
			}
			// Delete "-N <chain>"
			if err := ipt.DeleteChain(rule.table, rule.chain); err != nil {
				return fmt.Errorf("failed to delete iptables chain %s on table %s: %w", rule.chain, rule.table, err)
			}
		}

		return nil
	})
}
