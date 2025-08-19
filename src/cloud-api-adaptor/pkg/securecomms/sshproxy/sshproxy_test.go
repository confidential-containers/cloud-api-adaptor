package sshproxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tuntest"
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

func getPeers(t *testing.T) (clientSSHPeer, serverSSHPeer *SSHPeer) {
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
		serverNetConn, serverChans, serverSSHReqs, err := ssh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			logger.Panicf("failed to NewServerConn server: %v", err)
		}

		serverSSHPeer = NewSSHPeer(context.Background(), Attestation, serverNetConn, serverChans, serverSSHReqs, "")
		close(done)
	}()

	clientNetConn, clientChans, clientSSHReqs, err := ssh.NewClientConn(clientConn, serverAddr, clientConfig)
	if err != nil {
		t.Errorf("failed to NewServerConn client: %v", err)
	}

	clientSSHPeer = NewSSHPeer(context.Background(), Attestation, clientNetConn, clientChans, clientSSHReqs, "")
	<-done
	return
}

func TestSshProxy(t *testing.T) {
	var wg sync.WaitGroup

	clientSSHPeer, serverSSHPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:127.0.0.1:7020", "  	"}); err != nil {
		t.Error(err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:7010", "  	"}, inboundPorts, &wg); err != nil {
		t.Error(err)
	}

	serverSSHPeer.AddOutbounds(outbounds)
	clientSSHPeer.AddInbounds(inbounds)

	clientSSHPeer.Ready()
	serverSSHPeer.Ready()

	s := test.HTTPServer("7020")
	if s == nil {
		t.Error("Failed - could not create server")
	}
	success := test.HTTPClient("http://127.0.0.1:7010")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSSHPeer.Upgrade()
	serverSSHPeer.Close("Test Finish")

	clientSSHPeer.Wait()
	if !clientSSHPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
	inbounds.DelAll()
}

func TestSshProxyWithNamespace(t *testing.T) {
	var wg sync.WaitGroup

	clientSSHPeer, serverSSHPeer := getPeers(t)

	testNs, testNsStr := tuntest.NewNamedNS(t, "test-TestSshProxyWithNamespace")
	defer tuntest.DeleteNamedNS(t, testNs)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:127.0.0.1:7020", "  	"}); err != nil {
		t.Error(err)
		return
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:ABC:" + testNsStr + ":7010", "  	"}, inboundPorts, &wg); err != nil {
		t.Error(err)
		return
	}

	serverSSHPeer.AddOutbounds(outbounds)
	clientSSHPeer.AddInbounds(inbounds)

	clientSSHPeer.Ready()
	serverSSHPeer.Ready()

	s := test.HTTPServer("7020")
	if s == nil {
		t.Error("Failed - could not create server")
		return
	}
	success := test.HTTPClientInNamespace("http://127.0.0.1:7010", testNs.Path())
	if !success {
		t.Error("Failed - not successful")
		return
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
		return
	}

	serverSSHPeer.Upgrade()
	serverSSHPeer.Close("Test Finish")

	clientSSHPeer.Wait()
	if !clientSSHPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
		return
	}
	inbounds.DelAll()
}

func TestSshProxyReverse(t *testing.T) {
	var wg sync.WaitGroup

	clientSSHPeer, serverSSHPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:XYZ:7001"}); err != nil {
		t.Errorf("Unable to add outbounds: %v", err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:XYZ:7011"}, inboundPorts, &wg); err != nil {
		t.Errorf("Unable to add inbounds: %v", err)
	}

	clientSSHPeer.AddOutbounds(outbounds)
	serverSSHPeer.AddInbounds(inbounds)

	clientSSHPeer.Ready()
	serverSSHPeer.Ready()

	s := test.HTTPServer("7001")
	if s == nil {
		t.Error("Failed - could not create server")
	}
	success := test.HTTPClient("http://127.0.0.1:7011")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSSHPeer.Upgrade()
	serverSSHPeer.Close("Test Finish")

	clientSSHPeer.Wait()
	if !clientSSHPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
}

func TestSshProxyReverseKBS(t *testing.T) {
	var wg sync.WaitGroup

	clientSSHPeer, serverSSHPeer := getPeers(t)

	outbounds := Outbounds{}
	if err := outbounds.AddTags([]string{"ATTESTATION_PHASE:KBS:7002"}); err != nil {
		t.Errorf("Unable to add outbounds: %v", err)
	}
	inboundPorts := map[string]string{}
	inbounds := Inbounds{}
	if err := inbounds.AddTags([]string{"ATTESTATION_PHASE:KBS:7012"}, inboundPorts, &wg); err != nil {
		t.Errorf("Unable to add inbounds: %v", err)
	}

	clientSSHPeer.AddOutbounds(outbounds)
	serverSSHPeer.AddInbounds(inbounds)

	clientSSHPeer.Ready()
	serverSSHPeer.Ready()

	s := test.HTTPServer("7002")
	if s == nil {
		t.Error("Failed - could not create server")
	}
	success := test.HTTPClient("http://127.0.0.1:7012")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}

	serverSSHPeer.Upgrade()
	serverSSHPeer.Close("Test finished")

	clientSSHPeer.Wait()
	if !clientSSHPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
}

func TestParseTag(t *testing.T) {

	tests := []struct {
		name      string
		tag       string
		wantPort  int
		wantHost  string
		wantName  string
		wantPhase string
		wantErr   bool
	}{
		{name: "<Phase>:<Name>:<Port>", tag: "KUBERNETES_PHASE:nn:12", wantPort: 12, wantHost: "", wantName: "nn", wantPhase: "KUBERNETES_PHASE", wantErr: false},
		{name: "<Phase>:<Name>:<Host/NS>:<Port>", tag: "ATTESTATION_PHASE:nn:12", wantPort: 12, wantHost: "", wantName: "nn", wantPhase: "ATTESTATION_PHASE", wantErr: false},
		{name: "<Bad Phase>:<Name>:<Port>", tag: "MY_PHASE:nn:12", wantPort: 12, wantHost: "", wantName: "nn", wantPhase: "MY_PHASE", wantErr: true},
		{name: "<X>:<Y>", tag: "ATTESTATION_PHASE:12", wantPort: 0, wantHost: "", wantName: "", wantPhase: "", wantErr: true},
		{name: "<X>", tag: "ATTESTATION_PHASE", wantPort: 0, wantHost: "", wantName: "", wantPhase: "", wantErr: true},
		{name: "<X>:<Y>:<Z><A><B>", tag: "ATTESTATION_PHASE:12", wantPort: 0, wantHost: "", wantName: "", wantPhase: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, gotHost, gotName, gotPhase, err := ParseTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPort != tt.wantPort {
				t.Errorf("ParseTag() gotPort = %v, want %v", gotPort, tt.wantPort)
			}
			if gotHost != tt.wantHost {
				t.Errorf("ParseTag() gotHost = %v, want %v", gotHost, tt.wantHost)
			}
			if gotName != tt.wantName {
				t.Errorf("ParseTag() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotPhase != tt.wantPhase {
				t.Errorf("ParseTag() gotPhase = %v, want %v", gotPhase, tt.wantPhase)
			}
		})
	}
}
