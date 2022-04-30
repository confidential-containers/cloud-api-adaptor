// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork/tuntest"
)

type mockWorkerNodeTunneler struct{}

func newMockWorkerNodeTunneler() tunneler.Tunneler {
	return &mockWorkerNodeTunneler{}
}

func (t *mockWorkerNodeTunneler) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {
	return nil
}

func (t *mockWorkerNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}

type mockPodNodeTunneler struct{}

func newMockPodNodeTunneler() tunneler.Tunneler {
	return &mockPodNodeTunneler{}
}

func (t *mockPodNodeTunneler) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {
	return nil
}

func (t *mockPodNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {
	return nil
}

func TestWorkerNode(t *testing.T) {

	mockTunnelType := "mock"
	tunneler.Register(mockTunnelType, newMockWorkerNodeTunneler, newMockPodNodeTunneler)

	workerNodeNS := tuntest.NewNamedNS(t, "test-workernode")
	defer tuntest.DeleteNamedNS(t, workerNodeNS)

	tuntest.BridgeAdd(t, workerNodeNS, "ens0")
	tuntest.AddrAdd(t, workerNodeNS, "ens0", "192.168.0.2/24")
	tuntest.BridgeAdd(t, workerNodeNS, "ens1")
	tuntest.AddrAdd(t, workerNodeNS, "ens1", "192.168.1.2/24")
	tuntest.RouteAdd(t, workerNodeNS, "", "192.168.0.1", "ens0")

	workerPodNS := tuntest.NewNamedNS(t, "test-workerpod")
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

			workerNode := NewWorkerNode(mockTunnelType, hostInterface)
			require.NotNil(t, workerNode, "hostInterface=%q", hostInterface)

			config, err := workerNode.Inspect(workerPodNS.Path)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			err = workerNode.Setup(workerPodNS.Path, []net.IP{net.ParseIP("192.168.0.3"), net.ParseIP("192.168.0.3")}, config)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			require.Equal(t, "172.16.0.2/24", config.PodIP, "hostInterface=%q", hostInterface)
			require.Equal(t, "eth0", config.InterfaceName, "hostInterface=%q", hostInterface)
			require.Equal(t, 1500, config.MTU, "hostInterface=%q", hostInterface)
			require.Equal(t, hostInterface == "ens1", config.Dedicated, "hostInterface=%q", hostInterface)
			require.Equal(t, expected.workerNodeIP, config.WorkerNodeIP, "hostInterface=%q", hostInterface)
			require.Equal(t, mockTunnelType, config.TunnelType, "hostInterface=%q", hostInterface)

			require.Equal(t, len(config.Routes), 1, "hostInterface=%q", hostInterface)
			require.Empty(t, config.Routes[0].Dst, "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[0].GW, "172.16.0.1", "hostInterface=%q", hostInterface)
			require.Equal(t, config.Routes[0].Dev, "eth0", "hostInterface=%q", hostInterface)

			err = workerNode.Teardown(workerPodNS.Path, config)
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			return nil
		})
		require.Nil(t, err, "hostInterface=%q", hostInterface)
	}
}

func TestPodNode(t *testing.T) {

	mockTunnelType := "mock"
	tunneler.Register(mockTunnelType, newMockWorkerNodeTunneler, newMockPodNodeTunneler)

	podNodeNS := tuntest.NewNamedNS(t, "test-podnode")
	defer tuntest.DeleteNamedNS(t, podNodeNS)

	tuntest.BridgeAdd(t, podNodeNS, "ens0")
	tuntest.AddrAdd(t, podNodeNS, "ens0", "192.168.0.3/24")
	tuntest.BridgeAdd(t, podNodeNS, "ens1")
	tuntest.AddrAdd(t, podNodeNS, "ens1", "192.168.1.3/24")
	tuntest.RouteAdd(t, podNodeNS, "", "192.168.0.1", "ens0")

	podNS := tuntest.NewNamedNS(t, "test-pod")
	defer tuntest.DeleteNamedNS(t, podNS)

	for hostInterface, expected := range map[string]struct {
		podNodeIP    string
		workerNodeIP string
	}{
		"": {
			podNodeIP:    "192.168.0.3",
			workerNodeIP: "192.168.0.2",
		},
		"ens0": {
			podNodeIP:    "192.168.0.3",
			workerNodeIP: "192.168.0.2",
		},
		"ens1": {
			podNodeIP:    "192.168.1.3",
			workerNodeIP: "192.168.1.2",
		},
	} {

		err := podNodeNS.Run(func() error {

			config := &tunneler.Config{
				PodIP: "172.16.0.2",
				Routes: []*tunneler.Route{
					{
						Dst: "",
						GW:  "172.16.0.1",
						Dev: "eth0",
					},
				},
				InterfaceName: "eth0",
				MTU:           1500,
				WorkerNodeIP:  expected.workerNodeIP,
				TunnelType:    mockTunnelType,
				Dedicated:     hostInterface == "ens1",
			}

			podNode := NewPodNode(podNS.Path, hostInterface, config)
			require.NotNil(t, podNode, "hostInterface=%q", hostInterface)

			err := podNode.Setup()
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			err = podNode.Teardown()
			require.Nil(t, err, "hostInterface=%q", hostInterface)

			return nil
		})
		require.Nil(t, err, "hostInterface=%q", hostInterface)
	}
}

func TestPluginDetectHostInterface(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Log("This test requires root privileges. Skipping")
		return
	}

	hostNS := tuntest.NewNamedNS(t, "test-host")
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
		if e, a := fmt.Sprintf("failed to identify IP address assigned to host interface eth1 on netns %s", hostNS.Path), err.Error(); e != a {
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
