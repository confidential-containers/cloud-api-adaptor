// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"runtime"
	"testing"

	testutils "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/internal/testing"
	"github.com/vishvananda/netns"
)

func TestRoute(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oldns, err := netns.Get()
	if err != nil {
		t.Fatalf("Failed to get the current network namespace: %v", err)
	}

	podns, err := netns.New()
	if err != nil {
		t.Fatalf("Failed to create network namespace: %v", err)
	}
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
	testutils.SkipTestIfNotRoot(t)

	ns, err := OpenCurrentNamespace()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	defer ns.Close()

	routes, err := ns.RouteList()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	for _, route := range routes {
		t.Logf("Route: dst:%s, gw:%s, dev:%s, prio: %d", route.Destination.String(), route.Gateway.String(), route.Device, route.Priority)
	}
}
