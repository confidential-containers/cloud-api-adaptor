package ppssh

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshproxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"golang.org/x/crypto/ssh"
)

const (
	UNPROVEN_WN_PUBLIC_KEY_PATH = "/run/unprovenWnPublicKey"
	SINGLETON_PATH              = "/run/sshSingleton"
	PP_PRIVATE_KEY              = "pp-sid/privateKey"   // Peer Pod Private Key
	WN_PUBLIC_KEY               = "sshclient/publicKey" // Worker Node Public Key
)

var logger = sshutil.Logger

type SshServer struct {
	inbounds  sshproxy.Inbounds
	outbounds sshproxy.Outbounds
	wg        sync.WaitGroup
	readyCh   chan struct{}
	getSecret GetSecret
	sshport   string
	listener  net.Listener
	ctx       context.Context
}

// NewSshServer initializes an SSH Server at the PP
// inbound_strings is a slice of strings where each string is an inbound tag
// outbounds_strings is a slice of strings where each string is an outbound tag
// Structure of an inbound tag: "<MyPort>:<InboundName>:<phase>"
// Structure of an outbound tag: "<DesPort>:<DesHost>:<outboundName>:<phase>"
// Phase may be "A" (Attestation), "K" (Kubernetes), or "B" (Both)
func NewSshServer(inbound_strings, outbounds_strings []string, getSecret GetSecret, sshport string) *SshServer {
	s := &SshServer{
		getSecret: getSecret,
		sshport:   sshport,
		readyCh:   make(chan struct{}),
	}
	logger.Printf("Using PP SecureComms: InitSshServer version %s", sshutil.PpSecureCommsVersion)

	if err := s.inbounds.AddTags(inbound_strings, nil, &s.wg); err != nil {
		logger.Fatalf("Failed to parse outbound tags %v: %v", inbound_strings, err)
	}
	if err := s.outbounds.AddTags(outbounds_strings); err != nil {
		logger.Fatalf("Failed to parse outbound tags %v: %v", outbounds_strings, err)
	}
	return s
}

func (s *SshServer) Ready() chan struct{} {
	return s.readyCh
}

func (s *SshServer) kubernetesPhase(kubernetesPhaseConfig *ssh.ServerConfig) {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	var peer *sshproxy.SshPeer
	for peer == nil && (ctx.Err() == nil) {
		logger.Printf("Kubernetes phase: waiting for client to connect\n")
		nConn, err := s.listener.Accept()
		if err != nil {
			logger.Fatal("failed to accept incoming connection (Kubernetes phase): ", err)
		}

		logger.Printf("Kubernetes client connected\n")
		peer, err = kubernetesSShService(ctx, nConn, kubernetesPhaseConfig)
		if err != nil {
			logger.Printf("Retrying after Kubernetes phase failed with: %s", err)
			peer = nil
			continue
		}

		peer.AddTags(s.inbounds, s.outbounds)
		peer.Ready()

		peer.Wait()
		logger.Printf("KubernetesSShService exiting")
	}
}

func (s *SshServer) attestationPhase() *ssh.ServerConfig {
	// Singleton - accept an unproven connection for attestation
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	var peer *sshproxy.SshPeer
	for (peer == nil) && (ctx.Err() == nil) {
		logger.Printf("Attestation phase: waiting for client to connect\n")
		nConn, err := s.listener.Accept()
		if err != nil {
			logger.Fatal("Attestation phase: failed to accept incoming connection: ", err)
		}

		logger.Printf("Attestation phase: client connected\n")

		peer, err = attestationSShService(ctx, nConn)
		if err != nil {
			logger.Print(err.Error())
			peer = nil
		}
	}

	peer.AddTags(s.inbounds, s.outbounds)

	peer.Ready()

	for ctx.Err() == nil {
		logger.Printf("Attestation phase: getting keys from KBS\n")
		ppSecrets := NewPpSecrets(s.getSecret)
		ppSecrets.AddKey(WN_PUBLIC_KEY)
		ppSecrets.AddKey(PP_PRIVATE_KEY)
		ppSecrets.Go() // wait for the keys
		config, err := initKubernetesPhaseSshConfig(ppSecrets)
		if err == nil {
			logger.Printf("Attestation phase: InitKubernetesPhaseSshConfig is ready\n")
			peer.Upgrade()
			return config
		}
		logger.Printf("Attestation phase: failed getting keys from KBS: %v\n", err)
	}
	return nil
}

func (s *SshServer) Start(ctx context.Context) error {
	var err error

	logger.Printf("SSH service starting on port: %s", s.sshport)
	s.ctx = ctx
	s.listener, err = net.Listen("tcp", "0.0.0.0:"+s.sshport)
	if err != nil {
		logger.Fatal("Failed to listen for connection: ", err)
	}
	close(s.readyCh) // notify systemd that the service is ready

	go func() {
		kubernetesPhaseConfig := s.attestationPhase()
		if kubernetesPhaseConfig == nil {
			logger.Fatal("Attestation phase failed")
		}
		for ctx.Err() == nil {
			s.kubernetesPhase(kubernetesPhaseConfig)
		}
		s.listener.Close()
	}()
	return nil
}

func Singleton() {
	// Singleton - make sure we run the ssh service once per boot.
	if _, err := os.Stat(SINGLETON_PATH); !errors.Is(err, os.ErrNotExist) {
		logger.Fatal("SSH service runs in singleton mode and cannot be executed twice")
	}
	singleton, err := os.Create(SINGLETON_PATH)
	if err != nil {
		logger.Fatalf("Failed to create Singleton file: %v", err)
	}
	singleton.Close()
}

func getAttestationPhaseKeys() (ppPrivateKeyBytes []byte, tePublicKeyBytes []byte) {
	var err error

	// Attestation phase - may have unproven tePublicKeyBytes
	tePublicKeyBytes, err = os.ReadFile(UNPROVEN_WN_PUBLIC_KEY_PATH)
	if err != nil {
		tePublicKeyBytes = nil
	}

	// Private Key generation - unproven to the client, key is generated on the fly
	ppPrivateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		logger.Fatalf("Attestation phase: failed to generate host key, err: %v", err)
	}

	// Validate Private Key
	err = ppPrivateKey.Validate()
	if err != nil {
		logger.Fatalf("Attestation phase: failed to validate host key, err: %v", err)
	}

	ppPrivateKeyBytes = sshutil.RsaPrivateKeyPEM(ppPrivateKey)
	logger.Printf("Attestation phase: SSH server initialized keys")
	return
}

func setConfigHostKey(config *ssh.ServerConfig, ppPrivateKeyBytes []byte) error {
	serverSigner, err := ssh.ParsePrivateKey(ppPrivateKeyBytes)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %w", err)
	}
	config.AddHostKey(serverSigner)
	return nil
}

func setPublicKey(config *ssh.ServerConfig, tePublicKeyBytes []byte) error {
	teSshPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(tePublicKeyBytes)
	if err != nil {
		return fmt.Errorf("unable to parse public key: %w", err)
	}

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config.PublicKeyCallback = func(c ssh.ConnMetadata, clientPublicKey ssh.PublicKey) (*ssh.Permissions, error) {
		if bytes.Equal(teSshPublicKey.Marshal(), clientPublicKey.Marshal()) {
			return &ssh.Permissions{
				// Record the public key used for authentication.
				Extensions: map[string]string{
					"pubkey-fp": ssh.FingerprintSHA256(clientPublicKey),
				},
			}, nil
		}
		return nil, fmt.Errorf("unknown public key for %q", c.User())
	}
	return nil
}

func initAttestationPhaseSshConfig() (*ssh.ServerConfig, error) {
	config := &ssh.ServerConfig{}

	ppPrivateKeyBytes, tePublicKeyBytes := getAttestationPhaseKeys()

	if tePublicKeyBytes != nil { // connect with an client public key
		if err := setPublicKey(config, tePublicKeyBytes); err != nil {
			return nil, err
		}
	} else {
		config.NoClientAuth = true
		logger.Printf("Attestation phase: SSH server initialized with NoClientAuth")
	}
	if err := setConfigHostKey(config, ppPrivateKeyBytes); err != nil {
		return nil, err
	}
	return config, nil
}

func initKubernetesPhaseSshConfig(ppSecrets *PpSecrets) (*ssh.ServerConfig, error) {
	config := &ssh.ServerConfig{}

	ppPrivateKeyBytes := ppSecrets.GetKey(PP_PRIVATE_KEY)
	wnPublicKeyBytes := ppSecrets.GetKey(WN_PUBLIC_KEY)

	if ppPrivateKeyBytes == nil || wnPublicKeyBytes == nil || len(ppPrivateKeyBytes) == 0 || len(wnPublicKeyBytes) == 0 { // connect with an client public key
		return nil, fmt.Errorf("kubernetes phase: missing SSH server key") // should never happen
	}
	if err := setPublicKey(config, wnPublicKeyBytes); err != nil {
		return nil, err
	}
	if err := setConfigHostKey(config, ppPrivateKeyBytes); err != nil {
		return nil, err
	}
	return config, nil
}

func kubernetesSShService(ctx context.Context, nConn net.Conn, kubernetesPhaseConfig *ssh.ServerConfig) (*sshproxy.SshPeer, error) {
	logger.Printf("Kubernetes phase: connected")

	// Handshake on the incoming net.Conn.
	conn, chans, sshReqs, err := ssh.NewServerConn(nConn, kubernetesPhaseConfig)
	if err != nil {
		logger.Printf("Kubernetes phase: failed to handshake: %s", err)
		return nil, err
	}

	if conn.Permissions != nil {
		logger.Printf("Kubernetes phase: logged-in with key %s", conn.Permissions.Extensions["pubkey-fp"])
	} else {
		logger.Printf("Kubernetes phase: logged-in without key")
	}

	// Starting ssh tunnel services for attestation phase
	peer := sshproxy.NewSshPeer(ctx, sshproxy.KUBERNETES, conn, chans, sshReqs, "")
	if peer == nil {
		return nil, fmt.Errorf("failed to connect to an ssh peer")
	}
	return peer, nil
}

func attestationSShService(ctx context.Context, nConn net.Conn) (*sshproxy.SshPeer, error) {
	logger.Printf("Attestation phase: connected")
	attestationPhaseConfig, err := initAttestationPhaseSshConfig()
	if err != nil {
		logger.Fatal(err)
	}
	// Handshake on the incoming net.Conn.
	conn, chans, sshReqs, err := ssh.NewServerConn(nConn, attestationPhaseConfig)
	if err != nil {
		err = fmt.Errorf("failed to handshake: %v", err)
		return nil, err
	}

	if conn.Permissions != nil {
		logger.Printf("Attestation phase: logged-in with key %s", conn.Permissions.Extensions["pubkey-fp"])
	} else {
		logger.Printf("Attestation phase: logged-in without key")
	}

	// Starting ssh tunnel services for attestation phase
	peer := sshproxy.NewSshPeer(ctx, sshproxy.ATTESTATION, conn, chans, sshReqs, "")
	return peer, nil
}
