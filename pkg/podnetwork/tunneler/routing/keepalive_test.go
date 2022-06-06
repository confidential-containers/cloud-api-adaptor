// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"net"
	"net/http"
	"testing"
	"time"

	testutils "github.com/confidential-containers/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tuntest"
	"github.com/vishvananda/netlink"
)

func TestKeepAlive(t *testing.T) {
	// TODO: enable this test once https://github.com/confidential-containers/cloud-api-adaptor/issues/52 is fixed
	testutils.SkipTestIfRunningInCI(t)
	testutils.SkipTestIfNotRoot(t)

	workerNS := tuntest.NewNamedNS(t, "test-host")
	defer tuntest.DeleteNamedNS(t, workerNS)

	vm1NS := tuntest.NewNamedNS(t, "test-podvm1")
	defer tuntest.DeleteNamedNS(t, vm1NS)

	ifName := "eth1"
	tuntest.VethAdd(t, workerNS, ifName, vm1NS, ifName)
	tuntest.AddrAdd(t, workerNS, ifName, "192.168.0.1/24")
	tuntest.AddrAdd(t, vm1NS, ifName, "192.168.0.2/24")

	if err := workerNS.LinkAdd(vrf1Name, &netlink.Vrf{Table: vrf1TableID}); err != nil {
		t.Fatalf("failed to add vrf %s: %v", vrf1Name, err)
	}

	if err := workerNS.LinkSetUp(vrf1Name); err != nil {
		t.Fatalf("failed to set vrf %s up: %v", vrf1Name, err)
	}

	if err := workerNS.LinkSetMaster(ifName, vrf1Name); err != nil {
		t.Fatalf("failed to set master of %s to vrf %s: %v", ifName, vrf1Name, err)
	}

	stopCh := make(chan struct{})

	if err := launchKeepAliveClient(workerNS, net.JoinHostPort("192.168.0.2", daemonListenPort), stopCh, withVRF(true), withInterval(200*time.Millisecond), withTimeout(100*time.Millisecond)); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	if err := launchKeepAliveServer(workerNS, true, stopCh); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := launchKeepAliveClient(vm1NS, net.JoinHostPort("192.168.0.1", defaultKeepAliveListenPort), stopCh, withVRF(false), withInterval(200*time.Millisecond), withTimeout(100*time.Millisecond)); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	go func() {
		if err := vm1NS.Run(func() error {
			httpServer := http.Server{
				Addr: net.JoinHostPort("0.0.0.0", daemonListenPort),
			}
			return httpServer.ListenAndServe()
		}); err != nil {
			t.Logf("Expect no error, got %v", err)
			t.Fail()
		}
	}()

	time.Sleep(time.Second)
	close(stopCh)
}
