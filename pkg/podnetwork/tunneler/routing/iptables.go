// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"fmt"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/coreos/go-iptables/iptables"
)

const (
	chainName   = "PEERPOD"
	ruleComment = "peerpod"
)

type iptablesRule struct {
	table string
	chain string
	spec  []string
}

func setIPTablesRules(ns *netops.NS, hostInterface string) error {

	var iptablesRules = []iptablesRule{
		{
			table: "raw",
			chain: chainName,
			spec:  []string{"-i", vrf1Name, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"},
		},
		{
			table: "raw",
			chain: chainName,
			spec:  []string{"-i", vrf2Name, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"},
		},
		{
			table: "raw",
			chain: chainName,
			spec:  []string{"-i", hostInterface, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"},
		},
		{
			table: "raw",
			chain: "PREROUTING",
			spec:  []string{"-j", chainName},
		},
		{
			table: "filter",
			chain: chainName,
			spec:  []string{"-i", vrf1Name, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"},
		},
		{
			table: "filter",
			chain: chainName,
			spec:  []string{"-i", vrf2Name, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"},
		},
		{
			table: "filter",
			chain: chainName,
			spec:  []string{"-i", hostInterface, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"},
		},
		{
			table: "filter",
			chain: "FORWARD",
			spec:  []string{"-j", chainName},
		},
	}

	return ns.Run(func() error {

		ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4))
		if err != nil {
			return fmt.Errorf("failed to initialize iptables: %w", err)
		}

		for _, rule := range iptablesRules {

			exists, err := ipt.ChainExists(rule.table, rule.chain)
			if err != nil {
				return fmt.Errorf("failed to check the existence of iptables chain %q: %w", rule.chain, err)
			}

			if !exists {
				if err := ipt.NewChain(rule.table, rule.chain); err != nil {
					return fmt.Errorf("failed to create iptables chain %q: %w", rule.chain, err)
				}
			}

			if err := ipt.AppendUnique(rule.table, rule.chain, rule.spec...); err != nil {
				return fmt.Errorf("failed to add iptables rule \"-t %s -A %s %s\": %w", rule.table, rule.chain, strings.Join(rule.spec, " "), err)
			}
		}

		return nil
	})
}
