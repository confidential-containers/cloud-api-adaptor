package ppssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshproxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/wnssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
	"golang.org/x/crypto/ssh"
)

// getKey uses kbs-client to obtain keys such as pp-sid/privateKey, sshclient/publicKey
func getKey(key string) (data []byte, err error) {
	url := fmt.Sprintf("http://127.0.0.1:9002/kbs/v0/resource/default/%s", key)

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				log.Printf("getKey client.DialContext() addr: %s", addr)
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getKey %s client.Get err: %w", key, err)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getKey %s io.ReadAll err - %w", key, err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("getKey %s client.Get received error code of : %d - %s", key, resp.StatusCode, string(data))
	}

	log.Printf("getKey %s statusCode %d success", key, resp.StatusCode)
	return
}

func createPKCS8Pem(t *testing.T) []byte {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Errorf("createPKCS8Keys ed25519.GenerateKey err: %v", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Errorf("createPKCS8Keys MarshalPKCS8PrivateKey err: %v", err)
	}
	kbscPrivatePem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		},
	)
	return kbscPrivatePem
}

func createKeys(t *testing.T) (privateKeyBytes, publicKeyBytes []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Errorf("CreateSecret rsa.GenerateKey err: %v", err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Errorf("CreateSecret ssh.NewPublicKey err: %v", err)
	}
	publicKeyBytes = ssh.MarshalAuthorizedKey(publicKey)
	privateKeyBytes = sshutil.RsaPrivateKeyPEM(privateKey)
	return
}

func getSigner(t *testing.T, privateKeyBytes []byte) ssh.Signer {
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		t.Errorf("Unable to parse private key: %v", err)
	}
	return signer
}

func getAttestationClient(t *testing.T, sshport string) (clientSSHPeer *sshproxy.SSHPeer, clientConn net.Conn) {
	var err error
	serverAddr := "127.0.0.1:" + sshport
	clientConn, err = net.Dial("tcp", serverAddr)
	if err != nil {
		t.Errorf("unable to Dial %s: %v", serverAddr, err)
	}

	privateKeyBytes, _ := createKeys(t)
	clientConfig := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		HostKeyAlgorithms: []string{"rsa-sha2-256", "rsa-sha2-512"},
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(getSigner(t, privateKeyBytes)),
		},
		Timeout: 5 * time.Minute,
	}

	clientNetConn, clientChans, clientSSHReqs, err := ssh.NewClientConn(clientConn, serverAddr, clientConfig)
	if err != nil {
		t.Errorf("failed to NewServerConn client: %v", err)
	}

	clientSSHPeer = sshproxy.NewSSHPeer(context.Background(), sshproxy.Attestation, clientNetConn, clientChans, clientSSHReqs, "fake")
	return
}

func getKubernetesClient(t *testing.T, sshport string, sPublicKey, cPrivateKey []byte) (clientSSHPeer *sshproxy.SSHPeer, clientConn net.Conn) {
	sSSHPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(sPublicKey)
	if err != nil {
		t.Errorf("Unable to ParseAuthorizedKey serverPublicKey: %v", err)

	}
	serverAddr := "127.0.0.1:" + sshport
	clientConn, err = net.Dial("tcp", serverAddr)
	if err != nil {
		t.Errorf("unable to Dial %s: %v", serverAddr, err)
	}
	clientConfig := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if !bytes.Equal(key.Marshal(), sSSHPublicKey.Marshal()) {
				logger.Printf("ssh host key mismatch - %s", key.Type())
				return fmt.Errorf("ssh host key mismatch")
			}
			logger.Printf("ssh host key match - %s", key.Type())
			return nil
		},
		HostKeyAlgorithms: []string{"rsa-sha2-256", "rsa-sha2-512"},
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(getSigner(t, cPrivateKey)),
		},
		Timeout: 5 * time.Minute,
	}

	clientNetConn, clientChans, clientSSHReqs, err := ssh.NewClientConn(clientConn, serverAddr, clientConfig)
	if err != nil {
		t.Errorf("failed to NewServerConn client: %v", err)
	}

	clientSSHPeer = sshproxy.NewSSHPeer(context.Background(), sshproxy.Kubernetes, clientNetConn, clientChans, clientSSHReqs, "fake")
	return
}

func TestPpssh(t *testing.T) {
	var wg sync.WaitGroup
	var err error

	sshport := "6002"
	inboundPorts := map[string]string{}
	inbounds := sshproxy.Inbounds{}
	outbounds := sshproxy.Outbounds{}

	if err := outbounds.AddTags([]string{"BOTH_PHASES:KBS:127.0.0.1:7015", "  	"}); err != nil {
		t.Error(err)
	}
	if err := inbounds.AddTags([]string{"KUBERNETES_PHASE:ABC:7005", "  	"}, inboundPorts, &wg); err != nil {
		t.Error(err)
	}

	// Forwarder Initialization
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	ppSecrets := NewPpSecrets(GetSecret(getKey))
	ppSecrets.AddKey(WNPublicKey)
	ppSecrets.AddKey(PPPrivateKey)

	sshServer := NewSSHServer([]string{"BOTH_PHASES:KBS:9002"}, []string{"KUBERNETES_PHASE:ABC:127.0.0.1:7105"}, ppSecrets, sshport)
	_ = sshServer.Start(ctx)
	clientSSHPeer, conn := getAttestationClient(t, sshport)
	clientSSHPeer.AddTags(inbounds, outbounds)

	test.KBSServer("7015")

	cPrivateKey, cPublicKey := createKeys(t)
	sPrivateKey, sPublicKey := createKeys(t)

	kc := wnssh.InitKbsClient("127.0.0.1:7015")
	err = kc.SetPemSecret(createPKCS8Pem(t))
	if err != nil {
		t.Errorf("KbsClient - %v", err)
	}
	err = kc.PostResource("default/sshclient/publicKey", cPublicKey)
	if err != nil {
		t.Errorf("PostResource: %v", err)
	}
	err = kc.PostResource("default/pp-fake/privateKey", sPrivateKey)
	if err != nil {
		t.Errorf("PostResource: %v", err)
	}

	s := test.HTTPServer("7105")
	if s == nil {
		t.Error("Failed - could not create server")
	}

	clientSSHPeer.Ready()
	clientSSHPeer.Wait()
	if !clientSSHPeer.IsUpgraded() {
		t.Errorf("attestation phase closed without being upgraded")
	}
	conn.Close()

	clientSSHPeer, conn = getKubernetesClient(t, sshport, sPublicKey, cPrivateKey)
	clientSSHPeer.AddTags(inbounds, outbounds)

	clientSSHPeer.Ready()
	success := test.HTTPClient("http://127.0.0.1:7005")
	if !success {
		t.Error("Failed - not successful")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}
	conn.Close()
	cancel()
}
