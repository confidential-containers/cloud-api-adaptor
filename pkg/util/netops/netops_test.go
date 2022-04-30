// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"os"
	"runtime"
	"testing"

	"github.com/vishvananda/netns"
)

func TestRoute(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Log("This test requires root privileges. Skipping")
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oldns, err := netns.Get()
	if err != nil {
		t.Fatalf("Failed to get the current network namespace: %v", err)
	}

	podns, err := netns.New()
	defer func() {
		if err := netns.Set(oldns); err != nil {
			t.Fatalf("Failed to set a network namespace: %v", err)
		}
		if err := podns.Close(); err != nil {
			t.Fatalf("Failed to close a network namespace: %v", err)
		}
	}()
}

func TestRouteList(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Log("This test requires root privileges. Skipping")
		return
	}

	ns, err := GetNS()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	defer ns.Close()

	routes, err := ns.GetRoutes()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	for _, route := range routes {
		t.Logf("Route: %#v", route)
	}
}
