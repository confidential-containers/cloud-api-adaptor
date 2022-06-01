// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package netops

import (
	"runtime"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/internal/testing"
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
