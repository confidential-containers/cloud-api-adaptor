package wnssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	retry "github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshproxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"golang.org/x/crypto/ssh"
)

var logger = sshutil.Logger

type SSHClient struct {
	kc              *KbsClient
	wnSigner        *ssh.Signer
	inboundStrings  []string
	outboundStrings []string
	sshport         string
	wnPublicKey     []byte
}

type SSHClientInstance struct {
	sid             string
	ppPublicKey     []byte
	ppAddr          []string
	sshClient       *SSHClient
	ctx             context.Context
	cancel          context.CancelFunc
	kubernetesPhase bool
	inbounds        sshproxy.Inbounds
	outbounds       sshproxy.Outbounds
	inboundPorts    map[string]string
	wg              sync.WaitGroup
}

func PpSecretName(sid string) string {
	return "pp-" + sid
}

// InitSSHClient initializes an SSH Client at the WN
// inbound_strings is a slice of strings where each string is an inbound tag
// outbounds_strings is a slice of strings where each string is an outbound tag
// Structure of an inbound tag: "<MyPort>:<InboundName>:<phase>"
// Structure of an outbound tag: "<DesPort>:<DesHost>:<outboundName>:<phase>"
// Phase may be "A" (Attestation), "K" (Kubernetes), or "B" (Both)
func InitSSHClient(inboundStrings, outboundStrings []string, secureCommsTrustee bool, kbsAddress string, sshport string) (*SSHClient, error) {
	logger.Printf("Using PP SecureComms: InitSshClient version %s", sshutil.PpSecureCommsVersion)

	// Read WN Secret
	wnPrivateKey, wnPublicKey, err := kubemgr.KubeMgr.ReadSecret(sshutil.AdaptorSSHSecret)
	if err != nil {
		// auto-create a secret
		wnPrivateKey, wnPublicKey, err = kubemgr.KubeMgr.CreateSecret(sshutil.AdaptorSSHSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to auto create WN secret: %w", err)
		}
	}
	if len(wnPrivateKey) == 0 {
		return nil, fmt.Errorf("missing keys for PeerPod")
	}
	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(wnPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}

	var kc *KbsClient
	if secureCommsTrustee {
		kbscPrivateKey, _, err := kubemgr.KubeMgr.ReadSecret(sshutil.KBSClientSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to read KBS client secret: %w", err)
		}

		kc = InitKbsClient(kbsAddress)
		err = kc.SetPemSecret(kbscPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("KbsClient - %v", err)
		}

		wnSecretPath := "default/sshclient/publicKey"
		logger.Printf("Updating KBS with secret for: %s", wnSecretPath)
		err = kc.PostResource(wnSecretPath, wnPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to PostResource WN Secret: %v", err)
		}
	}

	sshClient := &SSHClient{
		kc:              kc,
		wnSigner:        &signer,
		inboundStrings:  inboundStrings,
		outboundStrings: outboundStrings,
		sshport:         sshport,
		wnPublicKey:     wnPublicKey,
	}

	return sshClient, nil
}

func (ci *SSHClientInstance) GetPort(name string) string {
	var ok bool
	var inPort string
	inPort, ok = ci.inboundPorts[name]
	if !ok {
		return ""
	}
	return inPort
}
func (ci *SSHClientInstance) DisconnectPP(sid string) {

	ci.inbounds.DelAll()

	// Cancel the VM connection
	ci.cancel()
	ci.wg.Wait()
	logger.Print("SshClientInstance DisconnectPP success")

	// Remove peerPod Secret named peerPodId
	kubemgr.KubeMgr.DeleteSecret(PpSecretName(sid))
}

func (c *SSHClient) GetWnPublicKey() []byte {
	return c.wnPublicKey
}

func (c *SSHClient) InitPP(ctx context.Context, sid string) (ci *SSHClientInstance, ppPrivateKey []byte) {
	// Create peerPod Secret named peerPodId
	var ppPublicKey []byte
	var err error
	var kubernetesPhase bool

	// Try reading first in case we resume an existing PP
	logger.Printf("InitPP read/create PP secret named: %s", PpSecretName(sid))
	ppPrivateKey, ppPublicKey, err = kubemgr.KubeMgr.ReadSecret(PpSecretName(sid))
	if err != nil {
		ppPrivateKey, ppPublicKey, err = kubemgr.KubeMgr.CreateSecret(PpSecretName(sid))
		if err != nil {
			logger.Printf("Failed to create PP secret: %v", err)
			return
		}
	} else {
		// we already have a store secret for this PP
		kubernetesPhase = true
	}

	if c.kc != nil {
		// >>> Update the KBS about the SID's Secret !!! <<<
		sidSecretPath := fmt.Sprintf("default/pp-%s/privateKey", sid)
		logger.Printf("Updating KBS with secret for: %s", sidSecretPath)
		err = c.kc.PostResource(sidSecretPath, ppPrivateKey)
		if err != nil {
			logger.Printf("Failed to PostResource PP Secret: %v", err)
			return
		}

	}
	var ppSSHPublicKeyBytes []byte

	if len(ppPublicKey) > 0 {
		ppSSHPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(ppPublicKey)
		if err != nil {
			logger.Printf("Unable to ParseAuthorizedKey serverPublicKey: %v", err)
			return
		}
		ppSSHPublicKeyBytes = ppSSHPublicKey.Marshal()
	}

	ctx, cancel := context.WithCancel(ctx)
	ci = &SSHClientInstance{
		sid:             sid,
		ppPublicKey:     ppSSHPublicKeyBytes,
		sshClient:       c,
		ctx:             ctx,
		cancel:          cancel,
		inboundPorts:    make(map[string]string),
		kubernetesPhase: kubernetesPhase,
	}

	if err := ci.inbounds.AddTags(c.inboundStrings, ci.inboundPorts, &ci.wg); err != nil {
		logger.Fatalf("Failed to parse outbound tag %v: %v", c.inboundStrings, err)
	}

	if err := ci.outbounds.AddTags(c.outboundStrings); err != nil {
		logger.Fatalf("Failed to parse outbound tag %v: %v", c.outboundStrings, err)
	}

	return
}

func (ci *SSHClientInstance) Start(ipAddr []netip.Addr) error {
	ppAddr := make([]string, len(ipAddr))
	for i, ip := range ipAddr {
		ppAddr[i] = ip.String() + ":" + ci.sshClient.sshport
	}

	ci.ppAddr = ppAddr

	if !ci.kubernetesPhase {
		// Attestation phase
		logger.Println("Attestation phase: starting")
		if err := ci.StartAttestation(); err != nil {
			return fmt.Errorf("attestation phase failed: %v", err)
		}
		logger.Println("Attestation phase: done")
		ci.kubernetesPhase = true
	}

	// Kubernetes phase
	ci.wg.Add(1)
	go func() {
		defer ci.wg.Done()
		restarts := 0
		for {
			select {
			case <-ci.ctx.Done():
				logger.Printf("Kubernetes phase: done")
				return
			default:
				logger.Printf("Kubernetes phase: starting (number of restarts %d)", restarts)
				if err := ci.StartKubernetes(); err != nil {
					logger.Printf("Kubernetes phase: failed: %v", err)
				}
				time.Sleep(time.Second)
				restarts += 1
			}
		}
	}()
	return nil
}

func (ci *SSHClientInstance) StartKubernetes() error {
	ctx, cancel := context.WithCancel(ci.ctx)
	defer cancel()
	peer := ci.StartSSHClient(ctx, sshproxy.Kubernetes, ci.ppPublicKey, ci.sid)
	if peer == nil {

		return fmt.Errorf("kubernetes phase: failed StartSshClient")
	}

	peer.AddTags(ci.inbounds, ci.outbounds)

	peer.Ready()
	peer.Wait()
	return nil
}

func (ci *SSHClientInstance) StartAttestation() error {
	ctx, cancel := context.WithCancel(ci.ctx)
	defer cancel()
	peer := ci.StartSSHClient(ctx, sshproxy.Attestation, nil, ci.sid)
	if peer == nil {
		return fmt.Errorf("attestation phase: failed StartSshClient")
	}
	peer.AddTags(ci.inbounds, ci.outbounds)

	peer.Ready()
	peer.Wait()
	if !peer.IsUpgraded() {
		return fmt.Errorf("attestation phase closed without being upgraded")
	}
	return nil
}

func (ci *SSHClientInstance) StartSSHClient(ctx context.Context, phase string, publicKey []byte, sid string) *sshproxy.SSHPeer {
	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if len(publicKey) == 0 {
				logger.Printf("%s phase: ssh skipped validating server's host key (type %s) during attestation", phase, key.Type())
				return nil
			}
			if !bytes.Equal(key.Marshal(), ci.ppPublicKey) {
				logger.Printf("%s phase: ssh host key mismatch - %s", phase, key.Type())
				return fmt.Errorf("%s phase: ssh host key mismatch", phase)
			}
			logger.Printf("%s phase: ssh host key match - %s", phase, key.Type())
			return nil
		},
		HostKeyAlgorithms: []string{"rsa-sha2-256", "rsa-sha2-512"},
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(*ci.sshClient.wnSigner),
		},
		Timeout: 5 * time.Minute,
	}

	// Dial your ssh server.
	var peer *sshproxy.SSHPeer
	_ = retry.Do(
		func() error {
			for _, ppAddr := range ci.ppAddr {
				conn, err := net.DialTimeout("tcp", ppAddr, config.Timeout)
				if err != nil {
					logger.Printf("%s phase: unable to Dial %s: %v", phase, ppAddr, err)
					continue
				}
				logger.Printf("%s phase: ssh connected - %s", phase, conn.RemoteAddr())
				netConn, chans, sshReqs, err := ssh.NewClientConn(conn, ppAddr, config)
				if err != nil {
					logger.Printf("%s phase: unable to connect: %v", phase, err)
					conn.Close()
					continue
				}
				peer = sshproxy.NewSSHPeer(ctx, phase, netConn, chans, sshReqs, sid)
				return nil
			}
			return errors.New("Retry")
		},
		retry.Attempts(100),
		retry.Context(ctx),
		retry.MaxDelay(5*time.Second),
	)
	return peer
}
