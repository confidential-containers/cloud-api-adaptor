package securecomms_test

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/wnssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
)

//var logger = log.New(log.Writer(), "[adaptor] ", log.LstdFlags|log.Lmsgprefix)

func WN() bool {
	kubemgr.KubeMgr.DeleteSecret(wnssh.PpSecretName("sid"))
	test.KBSServer("9004")
	test.HTTPServer("8053")
	test.HTTPServer("26443")

	// CAA Initialization
	sshClient, err := wnssh.InitSSHClient([]string{"KUBERNETES_PHASE:KATAAGENT:0"}, []string{"BOTH_PHASES:KBS:9004", "KUBERNETES_PHASE:KUBEAPI:26443", "KUBERNETES_PHASE:DNS:8053"}, true, "127.0.0.1:9004", sshutil.SSHPORT)
	if err != nil {
		log.Fatalf("InitSshClient %v", err)
	}

	////////// CAA StartVM
	ipAddr, _ := netip.ParseAddr("127.0.0.1") // ipAddr of the VM
	ipAddrs := []netip.Addr{ipAddr}
	ctx := context.Background()
	ci, _ := sshClient.InitPP(ctx, "sid")
	if ci == nil {
		log.Fatalf("failed InitiatePeerPodTunnel")
	}

	// mimic adaptor runtime-forwarder
	inPort := ci.GetPort("KATAAGENT")
	if inPort == "" {
		log.Fatalf("failed find port")
	}

	if err := ci.Start(ipAddrs); err != nil {
		log.Fatalf("failed ci.Start: %s", err)
	}

	success := test.HTTPClient(fmt.Sprintf("http://127.0.0.1:%s", inPort))

	////////// CAA StopVM
	ci.DisconnectPP("sid")
	return success
}
