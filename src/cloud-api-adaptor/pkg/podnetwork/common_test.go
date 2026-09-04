// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	testutils "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tuntest"
)

func TestInferGatewayFromSubnet(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		want    string
		wantErr bool
	}{
		{name: "standard /24", prefix: "192.168.1.4/24", want: "192.168.1.1"},
		{name: "standard /16", prefix: "10.129.2.111/23", want: "10.129.2.1"},
		{name: "already the gateway address", prefix: "10.0.0.1/24", want: "10.0.0.1"},
		{name: "IPv6 is unsupported", prefix: "fd00::4/64", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tc.prefix)
			gw, err := inferGatewayFromSubnet(prefix)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, gw.String())
		})
	}

	t.Run("invalid prefix", func(t *testing.T) {
		_, err := inferGatewayFromSubnet(netip.Prefix{})
		require.Error(t, err)
	})
}

// TestGetSecondaryInterfaceDetailsCrossSubnet simulates the Azure multi-NIC
// scenario: the secondary interface has an IPv4 address but no default
// route, and it is on a different subnet than the primary interface (so the
// gateway cannot be reused from the primary's route and must be inferred).
func TestGetSecondaryInterfaceDetailsCrossSubnet(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	ns, _ := tuntest.NewNamedNS(t, "test-crosssubnet")
	defer tuntest.DeleteNamedNS(t, ns)

	tuntest.BridgeAdd(t, ns, "eth0")
	tuntest.AddrAdd(t, ns, "eth0", "10.129.2.111/23")
	tuntest.RouteAdd(t, ns, "", "10.129.2.1", "eth0")

	tuntest.BridgeAdd(t, ns, "eth1")
	tuntest.AddrAdd(t, ns, "eth1", "192.168.1.4/24")
	// Deliberately no default route on eth1: Azure does not hand out a
	// default route to secondary NICs.

	err := ns.Run(func() error {
		secIface, secAddrCIDR, secRoute, err := getSecondaryInterfaceDetails(ns, "eth0")
		require.NoError(t, err)
		require.Equal(t, "eth1", secIface)
		require.Equal(t, "192.168.1.4/24", secAddrCIDR.String())
		require.NotNil(t, secRoute)
		require.Equal(t, "eth1", secRoute.Device)
		require.Equal(t, "192.168.1.1", secRoute.Gateway.String())
		require.True(t, secRoute.Destination.Bits() == 0, "expected the inferred route to be a default route")
		return nil
	})
	require.NoError(t, err)
}

// TestGetSecondaryInterfaceDetailsSameSubnet exercises the pre-existing
// fallback where the secondary interface shares the primary's subnet and
// has no default route of its own; the primary's gateway is reused.
func TestGetSecondaryInterfaceDetailsSameSubnet(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	ns, _ := tuntest.NewNamedNS(t, "test-samesubnet")
	defer tuntest.DeleteNamedNS(t, ns)

	tuntest.BridgeAdd(t, ns, "eth0")
	tuntest.AddrAdd(t, ns, "eth0", "10.0.0.4/24")
	tuntest.RouteAdd(t, ns, "", "10.0.0.1", "eth0")

	tuntest.BridgeAdd(t, ns, "eth1")
	tuntest.AddrAdd(t, ns, "eth1", "10.0.0.5/24")

	err := ns.Run(func() error {
		secIface, _, secRoute, err := getSecondaryInterfaceDetails(ns, "eth0")
		require.NoError(t, err)
		require.Equal(t, "eth1", secIface)
		require.Equal(t, "eth1", secRoute.Device)
		require.Equal(t, "10.0.0.1", secRoute.Gateway.String())
		return nil
	})
	require.NoError(t, err)
}

func TestGetSecondaryInterfaceDetailsNoneFound(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	ns, _ := tuntest.NewNamedNS(t, "test-nosecondary")
	defer tuntest.DeleteNamedNS(t, ns)

	tuntest.BridgeAdd(t, ns, "eth0")
	tuntest.AddrAdd(t, ns, "eth0", "10.0.0.4/24")
	tuntest.RouteAdd(t, ns, "", "10.0.0.1", "eth0")

	err := ns.Run(func() error {
		_, _, _, err := getSecondaryInterfaceDetails(ns, "eth0")
		require.True(t, errors.Is(err, ErrNoSecondaryInterface))
		return nil
	})
	require.NoError(t, err)
}

// TestSetupExternalNetworkCrossSubnet verifies that a secondary interface
// discovered on a different subnet than the primary is correctly moved into
// the pod namespace with its address and inferred default route in place.
func TestSetupExternalNetworkCrossSubnet(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	hostNS, _ := tuntest.NewNamedNS(t, "test-extnet-host")
	defer tuntest.DeleteNamedNS(t, hostNS)

	tuntest.BridgeAdd(t, hostNS, "eth0")
	tuntest.AddrAdd(t, hostNS, "eth0", "10.129.2.111/23")
	tuntest.RouteAdd(t, hostNS, "", "10.129.2.1", "eth0")

	podNS, _ := tuntest.NewNamedNS(t, "test-extnet-pod")
	defer tuntest.DeleteNamedNS(t, podNS)

	// eth1 must be a veth endpoint, not a bridge: Linux bridge master
	// devices cannot be moved between network namespaces (SetNamespace
	// fails with EINVAL), whereas a veth endpoint can, matching how real
	// NICs behave. peerNS just anchors the other end of the pair and is
	// otherwise unused.
	peerNS, _ := tuntest.NewNamedNS(t, "test-extnet-peer")
	defer tuntest.DeleteNamedNS(t, peerNS)

	tuntest.VethAdd(t, hostNS, "eth1", peerNS, "peer0")
	tuntest.AddrAdd(t, hostNS, "eth1", "192.168.1.4/24")

	err := hostNS.Run(func() error {
		return setupExternalNetwork(hostNS, "eth0", podNS)
	})
	require.NoError(t, err)

	err = podNS.Run(func() error {
		link, err := podNS.LinkFind("eth1")
		require.NoError(t, err)

		addrs, err := link.GetAddr()
		require.NoError(t, err)
		require.Len(t, addrs, 1)
		require.Equal(t, "192.168.1.4/24", addrs[0].String())

		defRoutes, err := podNS.GetDefaultRoutes()
		require.NoError(t, err)
		require.Len(t, defRoutes, 1)
		require.Equal(t, "eth1", defRoutes[0].Device)
		require.Equal(t, "192.168.1.1", defRoutes[0].Gateway.String())
		return nil
	})
	require.NoError(t, err)
}
