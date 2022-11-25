// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package tuntest

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	testutils "github.com/confidential-containers/cloud-api-adaptor/pkg/internal/testing"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/coreos/go-iptables/iptables"
)

type testPod struct {
	podAddr                       string
	podHwAddr                     string
	podNodePrimaryAddr            string
	podNodeSecondaryAddr          string
	workerPodNS, podNS, podNodeNS *netops.NS
	config                        *tunneler.Config
	hostInterface                 string
	workerNodeTunneler            tunneler.Tunneler
	podNodeTunneler               tunneler.Tunneler
}

func getIP(t *testing.T, addr string) string {
	t.Helper()

	ip, _, err := net.ParseCIDR(addr)
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	return ip.String()
}

func RunTunnelTest(t *testing.T, tunnelType string, newWorkerNodeTunneler, newPodNodeTunneler func() tunneler.Tunneler, dedicated bool) {
	testutils.SkipTestIfNotRoot(t)

	const (
		gatewayIP           = "10.128.0.1"
		gatewayAddr         = gatewayIP + "/24"
		workerPrimaryAddr   = "10.10.0.1/16"
		workerSecondaryAddr = "192.168.0.1/24"
	)

	pods := []*testPod{
		{podAddr: "10.128.0.2/24", podHwAddr: "0a:58:0a:84:03:ce", podNodePrimaryAddr: "10.10.1.2/16", podNodeSecondaryAddr: "192.168.0.2/24"},
		{podAddr: "10.128.0.3/24", podHwAddr: "0a:58:0a:84:03:cf", podNodePrimaryAddr: "10.10.1.3/16", podNodeSecondaryAddr: "192.168.0.3/24"},
	}

	bridgeNS := NewNamedNS(t, "test-bridge")
	defer DeleteNamedNS(t, bridgeNS)

	workerNS := NewNamedNS(t, "test-worker")
	defer DeleteNamedNS(t, workerNS)

	if err := workerNS.Run(func() error {
		ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4))
		if err != nil {
			return err
		}
		if err := ipt.Append("filter", "FORWARD", "-i", "cni0", "-j", "ACCEPT"); err != nil {
			return err
		}
		return ipt.ChangePolicy("filter", "FORWARD", "DROP")
	}); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	BridgeAdd(t, bridgeNS, "br0")
	BridgeAdd(t, bridgeNS, "br1")

	VethAdd(t, workerNS, "enc0", bridgeNS, "worker-eth0")
	VethAdd(t, workerNS, "enc1", bridgeNS, "worker-eth1")

	LinkSetMaster(t, bridgeNS, "worker-eth0", "br0")
	LinkSetMaster(t, bridgeNS, "worker-eth1", "br1")

	BridgeAdd(t, workerNS, "cni0")

	AddrAdd(t, workerNS, "cni0", gatewayAddr)
	AddrAdd(t, workerNS, "enc0", workerPrimaryAddr)
	AddrAdd(t, workerNS, "enc1", workerSecondaryAddr)

	RouteAdd(t, workerNS, "", "10.10.254.1", "enc0")
	AddrAdd(t, bridgeNS, "br0", "10.10.254.1/16")

	for i, pod := range pods {

		pod.workerNodeTunneler = newWorkerNodeTunneler()
		pod.podNodeTunneler = newPodNodeTunneler()

		pod.workerPodNS = NewNamedNS(t, fmt.Sprintf("test-workerpod%d", i))
		defer DeleteNamedNS(t, pod.workerPodNS)

		veth := fmt.Sprintf("veth%d", i)
		VethAdd(t, workerNS, veth, pod.workerPodNS, "eth0")
		LinkSetMaster(t, workerNS, veth, "cni0")

		AddrAdd(t, pod.workerPodNS, "eth0", pod.podAddr)
		HwAddrAdd(t, pod.workerPodNS, "eth0", pod.podHwAddr)
		RouteAdd(t, pod.workerPodNS, "", gatewayIP, "eth0")

		pod.podNodeNS = NewNamedNS(t, fmt.Sprintf("test-podvm%d", i))
		defer DeleteNamedNS(t, pod.podNodeNS)

		vmEth0 := fmt.Sprintf("podvm%d-eth0", i)
		vmEth1 := fmt.Sprintf("podvm%d-eth1", i)

		VethAdd(t, pod.podNodeNS, "enc0", bridgeNS, vmEth0)
		VethAdd(t, pod.podNodeNS, "enc1", bridgeNS, vmEth1)
		AddrAdd(t, pod.podNodeNS, "enc0", pod.podNodePrimaryAddr)
		AddrAdd(t, pod.podNodeNS, "enc1", pod.podNodeSecondaryAddr)
		LinkSetMaster(t, bridgeNS, vmEth0, "br0")
		LinkSetMaster(t, bridgeNS, vmEth1, "br1")

		pod.podNS = NewNamedNS(t, fmt.Sprintf("test-pod%d", i))
		defer DeleteNamedNS(t, pod.podNS)
	}

	for i, pod := range pods {

		pod.config = &tunneler.Config{
			PodIP:         pod.podAddr,
			PodHwAddr:     pod.podHwAddr,
			Routes:        []*tunneler.Route{{Dst: "", GW: "10.128.0.1", Dev: ""}},
			InterfaceName: "eth0",
			MTU:           1500,
			TunnelType:    tunnelType,
			Dedicated:     dedicated,
			Index:         i,
		}

		if tunnelType == "vxlan" {
			pod.config.VXLANPort = 4789     // vxlan.DefaultVXLANPort
			pod.config.VXLANID = 555000 + i // vxlan.DefaultVXLANMinID + index
		}

		podNodeIPs := []net.IP{net.ParseIP(getIP(t, pod.podNodePrimaryAddr))}

		if dedicated {
			podNodeIPs = append(podNodeIPs, net.ParseIP(getIP(t, pod.podNodeSecondaryAddr)))
			pod.hostInterface = "enc1"
			pod.config.WorkerNodeIP = workerSecondaryAddr
		} else {
			pod.hostInterface = "enc0"
			pod.config.WorkerNodeIP = workerPrimaryAddr
		}

		if err := workerNS.Run(func() error {
			return pod.workerNodeTunneler.Setup(pod.workerPodNS.Path, podNodeIPs, pod.config)

		}); err != nil {
			t.Fatalf("Expect no error, got %v", err)
		}

		go func() {
			if err := pod.podNodeNS.Run(func() error {
				httpServer := http.Server{
					Addr: net.JoinHostPort("0.0.0.0", "15150"),
				}
				return httpServer.ListenAndServe()
			}); err != nil {
				t.Logf("Expect no error, got %v", err)
				t.Fail()
			}
		}()

		if err := pod.podNodeNS.Run(func() error {
			return pod.podNodeTunneler.Setup(pod.podNS.Path, podNodeIPs, pod.config)

		}); err != nil {
			t.Fatalf("Expect no error, got %v", err)
		}
	}

	for _, pod := range pods {
		httpServer := StartHTTPServer(t, pod.podNS, fmt.Sprintf("%s:8080", getIP(t, pod.podAddr)))
		defer httpServer.Shutdown(t)
	}

	for i, pod := range pods {
		ConnectToHTTPServer(t, workerNS, fmt.Sprintf("%s:8080", getIP(t, pod.podAddr)), getIP(t, gatewayAddr))
		ConnectToHTTPServer(t, pod.podNS, fmt.Sprintf("%s:8080", getIP(t, pods[(i+1)%len(pods)].podAddr)), getIP(t, pod.podAddr))
	}

	for _, pod := range pods {

		if err := workerNS.Run(func() error {

			return pod.workerNodeTunneler.Teardown(pod.workerPodNS.Path, pod.hostInterface, pod.config)

		}); err != nil {
			t.Fatalf("Expect no error, got %v", err)
		}

		if err := pod.podNodeNS.Run(func() error {

			return pod.podNodeTunneler.Teardown(pod.podNS.Path, pod.hostInterface, pod.config)

		}); err != nil {
			t.Fatalf("Expect no error, got %v", err)
		}
	}
}
