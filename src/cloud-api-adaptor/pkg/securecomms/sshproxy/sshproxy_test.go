package sshproxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
	"golang.org/x/crypto/ssh"
)

func getSigner(t *testing.T) ssh.Signer {
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Errorf("Attestation phase: failed to generate host key, err: %v", err)
	}
	privateKeyBytes := sshutil.RsaPrivateKeyPEM(privateKey)
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		t.Errorf("Unable to parse private key: %v", err)
	}
	return signer
}

func getPeers(t *testing.T) (clientSshPeer, serverSshPeer *SshPeer) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Error(err)
	}

	serverAddr := "127.0.0.1:" + port
	clientConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Errorf("unable to net.Dial %s: %v", serverAddr, err)
	}

	serverConn, err := listener.Accept()
	if err != nil {
		t.Errorf("failed to accept incoming connection (Kubernetes phase): %v ", err)
	}

	serverConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	serverConfig.AddHostKey(getSigner(t))

	clientConfig := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		HostKeyAlgorithms: []string{"rsa-sha2-256", "rsa-sha2-512"},
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(getSigner(t)),
		},
		Timeout: 5 * time.Minute,
	}

	done := make(chan bool)
	go func() {
		serverNetConn, serverChans, serverSshReqs, err := ssh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			logger.Panicf("failed to NewServerConn server: %v", err)
		}

		serverSshPeer = NewSshPeer(context.Background(), ATTESTATION, serverNetConn, serverChans, serverSshReqs, "")
		close(done)
	}()

	clientNetConn, clientChans, clientSshReqs, err := ssh.NewClientConn(clientConn, serverAddr, clientConfig)
	if err != nil {
		t.Errorf("failed to NewServerConn client: %v", err)
	}

	clientSshPeer = NewSshPeer(context.Background(), ATTESTATION, clientNetConn, clientChans, clientSshReqs, "")
	<-done
	return
}

func TestSshProxy(t *testing.T) {
	var wg sync.WaitGroup

	clientSshPeer, serverSshPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:127.0.0.1:7020", "  	"}); err != nil {
		t.Error(err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:7010", "  	"}, inboundPorts, &wg); err != nil {
		t.Error(err)
	}

	serverSshPeer.AddOutbounds(outbounds)
	clientSshPeer.AddInbounds(inbounds)

	clientSshPeer.Ready()
	serverSshPeer.Ready()

	s := test.HttpServer("7020")
	success := test.HttpClient("http://127.0.0.1:7010")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSshPeer.Upgrade()
	serverSshPeer.Close("Test Finish")

	clientSshPeer.Wait()
	if !clientSshPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
	inbounds.DelAll()
}

func TestSshProxyReverse(t *testing.T) {
	var wg sync.WaitGroup

	clientSshPeer, serverSshPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:XYZ:7001"}); err != nil {
		t.Errorf("Unable to add outbounds: %v", err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:XYZ:7011"}, inboundPorts, &wg); err != nil {
		t.Errorf("Unable to add inbounds: %v", err)
	}

	clientSshPeer.AddOutbounds(outbounds)
	serverSshPeer.AddInbounds(inbounds)

	clientSshPeer.Ready()
	serverSshPeer.Ready()

	s := test.HttpServer("7001")
	success := test.HttpClient("http://127.0.0.1:7011")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSshPeer.Upgrade()
	serverSshPeer.Close("Test Finish")

	clientSshPeer.Wait()
	if !clientSshPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
}

func TestSshProxyReverseKBS(t *testing.T) {
	var wg sync.WaitGroup

	clientSshPeer, serverSshPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:KBS:7002"}); err != nil {
		t.Errorf("Unable to add outbounds: %v", err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:KBS:7012"}, inboundPorts, &wg); err != nil {
		t.Errorf("Unable to add inbounds: %v", err)
	}

	clientSshPeer.AddOutbounds(outbounds)
	serverSshPeer.AddInbounds(inbounds)

	clientSshPeer.Ready()
	serverSshPeer.Ready()

	s := test.HttpServer("7002")
	success := test.HttpClient("http://127.0.0.1:7012")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSshPeer.Upgrade()
	serverSshPeer.Close("Test finished")

	clientSshPeer.Wait()
	if !clientSshPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
}
