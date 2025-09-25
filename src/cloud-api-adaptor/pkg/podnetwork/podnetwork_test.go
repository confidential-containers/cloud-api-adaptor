// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testutils "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tuntest"
)

type mockWorkerNodeTunneler struct{}

func newMockWorkerNodeTunneler() (tunneler.Tunneler, error) {
	return &mockWorkerNodeTunneler{}, nil
}

func (t *mockWorkerNodeTunneler) Configure(n *tunneler.NetworkConfig, config *tunneler.Config) error {
	return nil
}

func (t *mockWorkerNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {
	return nil
}

func (t *mockWorkerNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}

type mockPodNodeTunneler struct{}

func newMockPodNodeTunneler() (tunneler.Tunneler, error) {
	return &mockPodNodeTunneler{}, nil
}

func (t *mockPodNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {
	return nil
}

func (t *mockPodNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}

func TestWorkerNode(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	mockTunnelType := "mock"
	tunneler.Register(mockTunnelType, newMockWorkerNodeTunneler, newMockPodNodeTunneler)

	workerNodeNS, _ := tuntest.NewNamedNS(t, "test-workernode")
	defer tuntest.DeleteNamedNS(t, workerNodeNS)

	tuntest.BridgeAdd(t, workerNodeNS, "ens0")
	tuntest.AddrAdd(t, workerNodeNS, "ens0", "192.168.0.2/24")
	tuntest.BridgeAdd(t, workerNodeNS, "ens1")
	tuntest.AddrAdd(t, workerNodeNS, "ens1", "192.168.1.2/24")
	tuntest.RouteAdd(t, workerNodeNS, "", "192.168.0.1", "ens0")

	workerPodNS, _ := tuntest.NewNamedNS(t, "test-workerpod")
	defer tuntest.DeleteNamedNS(t, workerPodNS)

	tuntest.BridgeAdd(t, workerPodNS, "eth0")
	tuntest.AddrAdd(t, workerPodNS, "eth0", "172.16.0.2/24")
	tuntest.RouteAdd(t, workerPodNS, "", "172.16.0.1", "eth0")

	for hostInterface, expected := range map[string]struct {
		podNodeIP    string
		workerNodeIP string
	}{
		"": {
			workerNodeIP: "192.168.0.2/24",
		},
		"ens0": {
			workerNodeIP: "192.168.0.2/24",
		},
		"ens1": {
			workerNodeIP: "192.168.1.2/24",
		},
	} {

		err := workerNodeNS.Run(func() error {

			workerNode, err := NewWorkerNode(&tunneler.NetworkConfig{TunnelType: mockTunnelType, HostInterface: hostInterface})
			require.NotNil(t, workerNode, "hostInterface=%q", hostInterface)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			config, err := workerNode.Inspect(workerPodNS.Path())
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			err = workerNode.Setup(workerPodNS.Path(), []netip.Addr{netip.MustParseAddr("192.168.0.3"), netip.MustParseAddr("192.168.0.3")}, config)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			require.Equal(t, "172.16.0.2/24", config.PodIP.String(), "hostInterface=%q", hostInterface)
			require.Equal(t, "eth0", config.InterfaceName, "hostInterface=%q", hostInterface)
			require.Equal(t, 1500, config.MTU, "hostInterface=%q", hostInterface)
			require.Equal(t, hostInterface == "ens1", config.Dedicated, "hostInterface=%q", hostInterface)
			require.Equal(t, expected.workerNodeIP, config.WorkerNodeIP.String(), "hostInterface=%q", hostInterface)
			require.Equal(t, mockTunnelType, config.TunnelType, "hostInterface=%q", hostInterface)

			require.Equal(t, len(config.Routes), 2, "hostInterface=%q", hostInterface)
			dstRoute := fmt.Sprintf("%s/%d", config.Routes[0].Dst.Addr().Unmap().String(), config.Routes[0].Dst.Bits())
			require.Equal(t, dstRoute, "0.0.0.0/0", "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[0].GW.String(), "172.16.0.1", "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[0].Dev, "eth0", "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[1].Dst.String(), "172.16.0.0/24", "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[1].GW.IsValid(), false, "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[1].Dev, "eth0", "hostInterface=%q", hostInterface)

			err = workerNode.Teardown(workerPodNS.Path(), config)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			return nil
		})
		require.Nil(t, err, "hostInterface=%q", hostInterface)
	}
}

func TestPodNode(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	mockTunnelType := "mock"
	tunneler.Register(mockTunnelType, newMockWorkerNodeTunneler, newMockPodNodeTunneler)

	podNodeNS, _ := tuntest.NewNamedNS(t, "test-podnode")
	defer tuntest.DeleteNamedNS(t, podNodeNS)

	tuntest.BridgeAdd(t, podNodeNS, "ens0")
	tuntest.AddrAdd(t, podNodeNS, "ens0", "192.168.0.3/24")
	tuntest.BridgeAdd(t, podNodeNS, "ens1")
	tuntest.AddrAdd(t, podNodeNS, "ens1", "192.168.1.3/24")
	tuntest.RouteAdd(t, podNodeNS, "", "192.168.0.1", "ens0")

	for hostInterface, expected := range map[string]struct {
		podNodeIP    string
		workerNodeIP string
	}{
		"": {
			podNodeIP:    "192.168.0.3",
			workerNodeIP: "192.168.0.2/24",
		},
		"ens0": {
			podNodeIP:    "192.168.0.3",
			workerNodeIP: "192.168.0.2/24",
		},
		"ens1": {
			podNodeIP:    "192.168.1.3",
			workerNodeIP: "192.168.1.2/24",
		},
	} {

		podNS, _ := tuntest.NewNamedNS(t, "test-pod")
		func() {
			defer tuntest.DeleteNamedNS(t, podNS)

			tuntest.BridgeAdd(t, podNS, "eth0")
			tuntest.AddrAdd(t, podNS, "eth0", "172.16.0.2/24")

			err := podNodeNS.Run(func() error {

				config := &tunneler.Config{
					PodIP: netip.MustParsePrefix("172.16.0.2/24"),
					Routes: []*tunneler.Route{
						{
							Dst: netip.MustParsePrefix("0.0.0.0/0"),
							GW:  netip.MustParseAddr("172.16.0.1"),
							Dev: "eth0",
						},
						{
							Dst: netip.MustParsePrefix("172.16.0.0/24"),
							Dev: "eth0",
						},
					},
					InterfaceName: "eth0",
					MTU:           1500,
					WorkerNodeIP:  netip.MustParsePrefix(expected.workerNodeIP),
					TunnelType:    mockTunnelType,
					Dedicated:     hostInterface == "ens1",
				}

				podNode := NewPodNode(podNS.Path(), hostInterface, config)
				require.NotNil(t, podNode, "hostInterface=%q", hostInterface)

				err := podNode.Setup()
				require.Nil(t, err, "hostInterface=%q", hostInterface)

				err = podNode.Teardown()
				require.Nil(t, err, "hostInterface=%q", hostInterface)

				return nil
			})
			require.Nil(t, err, "hostInterface=%q", hostInterface)
		}()
	}
}

func TestPluginDetectHostInterface(t *testing.T) {
	testutils.SkipTestIfNotRoot(t)

	hostNS, _ := tuntest.NewNamedNS(t, "test-host")
	defer tuntest.DeleteNamedNS(t, hostNS)

	tuntest.BridgeAdd(t, hostNS, "eth0")
	tuntest.BridgeAdd(t, hostNS, "eth1")

	tuntest.AddrAdd(t, hostNS, "eth0", "10.10.0.2/24")

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		_, err := detectIP(hostNS, "eth1", 1*time.Second)
		errCh <- err
	}()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timeout does not occur")
	case err := <-errCh:
		if e, a := fmt.Sprintf("failed to identify IP address assigned to host interface eth1 on netns %s", hostNS.Path()), err.Error(); e != a {
			t.Fatalf("Expect %q, got %q", e, a)
		}
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		tuntest.AddrAdd(t, hostNS, "eth1", "192.168.0.2/24")
	}()

	ip, err := detectIP(hostNS, "eth1", 1500*time.Millisecond)
	if err != nil {
		t.Fatalf("Expect nil, got %v", err)
	}
	if e, a := ip.String(), "192.168.0.2"; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
}
