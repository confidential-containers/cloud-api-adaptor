// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tuntest"
	"github.com/coreos/go-iptables/iptables"
)

func TestIPTables(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	workerNS := tuntest.NewNamedNS(t, "test-host")
	defer tuntest.DeleteNamedNS(t, workerNS)

	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4))
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	if err := workerNS.Run(func() error {

		return ipt.ChangePolicy("filter", "FORWARD", "DROP")

	}); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	hostInterface := "ens4"

	if err := setIPTablesRules(workerNS, hostInterface); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	if err := workerNS.Run(func() error {

		if exists, err := ipt.ChainExists("filter", chainName); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("filter", chainName, "-i", vrf1Name, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("filter", chainName, "-i", vrf2Name, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("filter", chainName, "-i", hostInterface, "-m", "comment", "--comment", ruleComment, "-j", "ACCEPT"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.ChainExists("raw", chainName); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("raw", chainName, "-i", vrf1Name, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("raw", chainName, "-i", vrf2Name, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		if exists, err := ipt.Exists("raw", chainName, "-i", hostInterface, "-m", "comment", "--comment", ruleComment, "-j", "NOTRACK"); err != nil {
			return err
		} else if e, a := true, exists; e != a {
			t.Fatalf("Expect %v, got %v", e, a)
		}

		return nil

	}); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}
	// Check idempotency
	if err := setIPTablesRules(workerNS, hostInterface); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}
}
