package wnssh

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/ppssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
)

func TestSshProxyReverseKBS(t *testing.T) {
	sshport := "6003"
	kubemgr.InitKubeMgrMock()

	s9001 := test.KBSServer("9001")
	s8053 := test.HTTPServer("8053")
	s26443 := test.HTTPServer("26443")
	s7121 := test.HTTPServer("7121")
	if s9001 == nil || s8053 == nil || s26443 == nil || s7121 == nil {
		t.Error("Failed - could not create server")
	}
	test.CreatePKCS8Secret(t)

	// CAA Initialization
	sshClient, err := InitSSHClient([]string{"KUBERNETES_PHASE:KATAAGENT:0"}, []string{"BOTH_PHASES:KBS:9001", "KUBERNETES_PHASE:KUBEAPI:26443", "KUBERNETES_PHASE:DNS:8053"}, true, "127.0.0.1:9001", sshport)
	if err != nil {
		log.Fatalf("InitSshClient %v", err)
	}

	////////// CAA StartVM
	ipAddr, _ := netip.ParseAddr("127.0.0.1") // ipAddr of the VM
	ipAddrs := []netip.Addr{ipAddr}
	ci, _ := sshClient.InitPP(context.Background(), "sid")
	if ci == nil {
		log.Fatalf("failed InitiatePeerPodTunnel")
	}

	// mimic adaptor runtime-forwarder
	inPort := ci.GetPort("KATAAGENT")
	if inPort == "" {
		log.Fatalf("failed find port")
	}

	// create a podvm
	gkc := test.NewGetKeyClient("7030")
	ctx2, cancel2 := context.WithCancel(context.Background())

	ppSecrets := ppssh.NewPpSecrets(ppssh.GetSecret(gkc.GetKey))
	ppSecrets.AddKey(ppssh.WNPublicKey)
	ppSecrets.AddKey(ppssh.PPPrivateKey)

	sshServer := ppssh.NewSSHServer([]string{"BOTH_PHASES:KBS:7030", "KUBERNETES_PHASE:KUBEAPI:16443", "KUBERNETES_PHASE:DNS:9053"}, []string{"KUBERNETES_PHASE:KATAAGENT:127.0.0.1:7121"}, ppSecrets, sshport)
	_ = sshServer.Start(ctx2)

	// Forwarder Initialization

	if err := ci.Start(ipAddrs); err != nil {
		log.Fatalf("failed ci.Start: %s", err)
	}

	success := test.HTTPClient(fmt.Sprintf("http://127.0.0.1:%s", inPort))
	if !success {
		t.Error("Expected success")
	}
	////////// CAA StopVM
	ci.DisconnectPP("sid")
	cancel2()
}
