package sshproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
	"golang.org/x/crypto/ssh"
)

const (
	PPSID            = "pp-sid/"
	PPPrivateKey     = PPSID + "privateKey"
	Attestation      = "Attestation"
	Kubernetes       = "Kubernetes"
	KubernetesPhase  = "KUBERNETES_PHASE"
	AttestationPhase = "ATTESTATION_PHASE"
	BothPhases       = "BOTH_PHASES"
	Phase            = "Phase"
	Upgrade          = "Upgrade"
)

var logger = sshutil.Logger

type SSHPeer struct {
	sid            string
	phase          string
	sshConn        ssh.Conn
	ctx            context.Context
	done           chan bool
	outbounds      map[string]*Outbound
	inbounds       map[string]*Inbound
	wg             sync.WaitGroup
	upgrade        bool
	outboundsReady chan bool
	closeOnce      sync.Once
}

// Inbound side of the Tunnel - incoming tcp connections from local clients
type Inbound struct {
	// tcp peers
	Name        string
	TCPListener *net.TCPListener
	Connections chan *net.Conn
	Phase       string // ATTESTATION_PHASE, KUBERNETES_PHASE, BOTH_PHASES
}

type Outbound struct {
	// tcp peers
	Name    string
	OutAddr string
	Phase   string // ATTESTATION_PHASE, KUBERNETES_PHASE, BOTH_PHASES
}

type Outbounds struct {
	list []*Outbound
}

type Inbounds struct {
	list []*Inbound
}

func (outbounds *Outbounds) AddTags(tags []string) error {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		inPort, host, name, phase, err := ParseTag(tag)
		if err != nil {
			return fmt.Errorf("failed to parse outbound tag %s: %v", tag, err)
		}
		if host == "" {
			host = "127.0.0.1"
		}
		outbounds.Add(inPort, host, name, phase)
	}
	return nil
}

func (outbounds *Outbounds) Add(port int, host, name, phase string) {
	outbound := &Outbound{
		Phase:   phase,
		Name:    name,
		OutAddr: fmt.Sprintf("%s:%d", host, port),
	}
	outbounds.list = append(outbounds.list, outbound)
}

func (inbounds *Inbounds) AddTags(tags []string, inboundPorts map[string]string, wg *sync.WaitGroup) error {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		inPort, namespace, name, phase, err := ParseTag(tag)
		if err != nil {
			return fmt.Errorf("failed to parse inbound tag %s: %v", tag, err)
		}
		retPort, err := inbounds.Add(namespace, inPort, name, phase, wg)
		if err != nil {
			return fmt.Errorf("failed to add inbound: %v", err)
		}
		if inboundPorts != nil {
			inboundPorts[name] = retPort
		}
	}
	return nil
}

func (inbounds *Inbounds) listen(namespace string, tcpAddr *net.TCPAddr, name string) (tcpListener *net.TCPListener, err error) {
	if namespace == "" {
		return net.ListenTCP("tcp", tcpAddr)
	}

	var ns netops.Namespace
	ns, netopsErr := netops.OpenNamespace(filepath.Join("/run/netns", namespace))
	if netopsErr != nil {
		return nil, fmt.Errorf("inbound %s failed to OpenNamespace '%s': %w", name, namespace, netopsErr)
	}
	defer ns.Close()

	netopsErr = ns.Run(func() error {
		tcpListener, err = net.ListenTCP("tcp", tcpAddr)
		return err
	})
	if netopsErr != nil {
		return nil, fmt.Errorf("inbound %s failed to ListenTCP '%s': %w", name, namespace, netopsErr)
	}
	return
}

func (inbounds *Inbounds) Add(namespace string, inPort int, name, phase string, wg *sync.WaitGroup) (string, error) {
	tcpAddr := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: inPort,
	}
	tcpListener, err := inbounds.listen(namespace, tcpAddr, name)
	if err != nil {
		return "", fmt.Errorf("inbound failed to listen to host: %s port '%d' - err: %v", name, inPort, err)
	}
	_, retPort, err := net.SplitHostPort(tcpListener.Addr().String())
	if err != nil {
		panic(err)
	}
	logger.Printf("Inbound listening to port %s in namespace %s", retPort, namespace)

	inbound := &Inbound{
		Phase:       phase,
		TCPListener: tcpListener,
		Connections: make(chan *net.Conn),
		Name:        name,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			tcpConn, err := tcpListener.Accept()
			if err != nil {
				close(inbound.Connections)
				return
			}
			inbound.Connections <- &tcpConn
		}
	}()
	inbounds.list = append(inbounds.list, inbound)
	return retPort, nil
}

// NewInbound create an Inbound and listen to incoming client connections
func (inbounds *Inbounds) DelAll() {
	for _, inbound := range inbounds.list {
		inbound.TCPListener.Close()
	}
	inbounds.list = [](*Inbound){}
}

// ParseTag() parses an inbound or outbound tag
// Outbound tags with structure <Phase>:<Name>:<Port>
//
//	are interperted to approach 127.0.0.1:<Port>
//
// Outbound tags with structure <Phase>:<Name>:<Host>:<Port>
//
//	are interperted to approach <Host>:<Port>
//
// Inbound tags with structure <Phase>:<Name>:<Port>
//
//	are interperted to serve 127.0.0.1:<Port> on host network namespace
//
// Inbound tags with structure <Phase>:<Name>:<Namespace>:<Port>
//
//	are interperted to serve 127.0.0.1:<Port> on <Namespace> network namepsace
func ParseTag(tag string) (port int, host, name, phase string, err error) {
	var inPort string
	var uint64port uint64

	splits := strings.Split(tag, ":")
	if len(splits) == 3 {
		phase = splits[0]
		name = splits[1]
		inPort = splits[2]
	} else if len(splits) == 4 {
		phase = splits[0]
		name = splits[1]
		host = splits[2] // host for outbound or network namespace for inbound
		inPort = splits[3]
	} else {
		err = fmt.Errorf("illegal tag: %s", tag)
		return
	}

	if uint64port, err = strconv.ParseUint(inPort, 10, 16); err != nil {
		err = fmt.Errorf("illegal tag port '%s' - err: %v", inPort, err)
	} else {
		port = int(uint64port)
	}

	if phase != AttestationPhase && phase != KubernetesPhase && phase != BothPhases {
		err = fmt.Errorf("illegal tag phase '%s'", phase)
	}
	if name == "" {
		err = fmt.Errorf("illegal tag name '%s'", phase)
	}
	return
}

// NewSSHPeer
func NewSSHPeer(ctx context.Context, phase string, sshConn ssh.Conn, chans <-chan ssh.NewChannel, sshReqs <-chan *ssh.Request, sid string) *SSHPeer {
	peer := &SSHPeer{
		sid:            sid,
		phase:          phase,
		sshConn:        sshConn,
		ctx:            ctx,
		done:           make(chan bool, 1),
		outbounds:      make(map[string]*Outbound),
		inbounds:       make(map[string]*Inbound),
		outboundsReady: make(chan bool),
	}

	if chans == nil || sshReqs == nil {
		logger.Fatalf("NewSshPeer with illegal parameters chans %v sshReqs %v", chans, sshReqs)
	}

	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		for {
			select {
			case req := <-sshReqs:
				if req == nil {
					peer.Close("sshReqs closed")
					return
				}
				if req.WantReply {
					if req.Type == Phase {
						logger.Printf("%s phase: peer reported phase %s", phase, string(req.Payload))
						_ = req.Reply(true, []byte(peer.phase))
						continue
					}
					if phase == Attestation && req.Type == Upgrade {
						logger.Printf("%s phase: peer reported it is upgrading to Kubernetes phase", phase)
						_ = req.Reply(true, []byte(peer.phase))
						peer.upgrade = true
						continue
					}
					_ = req.Reply(false, nil)
				}

			case <-ctx.Done():
				peer.Close("Context canceled")
				return
			case ch := <-chans:
				if ch == nil {
					peer.Close("chans closed")
					return
				}
				switch ch.ChannelType() {
				default:
					logger.Printf("%s phase: NewSshPeer rejected channel for %s", phase, ch.ChannelType())
					_ = ch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel ** type: %v", ch.ChannelType()))
				case "tunnel":
					name := string(ch.ExtraData())
					<-peer.outboundsReady
					outbound := peer.outbounds[name]
					if outbound == nil || (outbound.Phase == AttestationPhase && phase != Attestation) || (outbound.Phase == KubernetesPhase && phase != Kubernetes) {
						logger.Printf("%s phase: NewSshPeer rejected tunnel channel: %s", phase, name)
						_ = ch.Reject(ssh.UnknownChannelType, fmt.Sprintf("%s phase: NewSshPeer rejected tunnel channel - port not allowed: %s", phase, name))
						continue
					}
					chChan, chReqs, err := ch.Accept()
					if err != nil {
						logger.Printf("%s phase: NewSshPeer failed to accept tunnel channel: %s", phase, err)
						peer.Close("Accept failed")
					}
					logger.Printf("%s phase: NewSshPeer - peer requested a tunnel channel for %s", phase, name)
					if outbound.Name == sshutil.KBS {
						outbound.acceptProxy(chChan, chReqs, sid, &peer.wg)
					} else {
						outbound.accept(chChan, chReqs, &peer.wg)
					}
				}
			}
		}
	}()
	ok, peerPhase, err := peer.sshConn.SendRequest(Phase, true, []byte(phase))
	if !ok {
		logger.Printf("%s phase: NewSshPeer - peer did not ok phase verification", phase)
		peer.Close("Phase verification failed")
		return nil
	}
	if err != nil {
		logger.Printf("%s phase: NewSshPeer - peer did not ok phase verification, err: %v", phase, err)
		peer.Close("Phase verification failed")
		return nil
	}
	if string(peerPhase) != phase {
		logger.Printf("%s phase: NewSshPeer - peer is in a different phase %s", phase, string(peerPhase))
		peer.Close("Phase verification failed")
		return nil
	}
	return peer
}

func (peer *SSHPeer) Wait() {
	peer.wg.Wait()
}

func (peer *SSHPeer) Close(who string) {
	peer.closeOnce.Do(func() {
		logger.Printf("%s phase: peer done by >>> %s <<<", peer.phase, who)
		peer.sshConn.Close()
		close(peer.done)
	})
}

func (peer *SSHPeer) IsUpgraded() bool {
	return peer.upgrade
}

func (peer *SSHPeer) Upgrade() {
	ok, _, err := peer.sshConn.SendRequest(Upgrade, true, []byte{})
	if !ok {
		logger.Printf("%s phase: SshPeer upgrade failed", peer.phase)
		peer.Close("Phase verification failed")
		return
	}
	if err != nil {
		logger.Printf("%s phase:SshPeer upgrade failed, err: %v", peer.phase, err)
		peer.Close("Phase verification failed")
		return
	}
}

func (peer *SSHPeer) AddTags(inbounds Inbounds, outbounds Outbounds) {
	peer.AddOutbounds(outbounds)
	peer.AddInbounds(inbounds)
}

// NewInbound create an Inbound and listen to incoming client connections
func (peer *SSHPeer) AddInbound(inbound *Inbound) {
	if (inbound.Phase == KubernetesPhase && peer.phase == Attestation) || (inbound.Phase == AttestationPhase && peer.phase == Kubernetes) {
		return
	}
	logger.Printf("%s phase: AddInbound: %s", peer.phase, inbound.Name)
	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		for {
			select {
			case conn, ok := <-inbound.Connections:
				if !ok {
					return
				}
				logger.Printf("%s phase: Inbound accept: %s", peer.phase, inbound.Name)
				NewInboundInstance(*conn, peer, inbound)
			case <-peer.done:
				return
			}
		}
	}()
	peer.inbounds[inbound.Name] = inbound
}

func NewInboundInstance(tcpConn io.ReadWriteCloser, peer *SSHPeer, inbound *Inbound) {
	sshChan, channelReqs, err := peer.sshConn.OpenChannel("tunnel", []byte(inbound.Name))
	if err != nil {
		logger.Printf("%s phase: NewInboundInstance OpenChannel %s error: %s", peer.phase, inbound.Name, err)
		return
	}
	logger.Printf("%s phase: NewInboundInstance OpenChannel opening tunnel for: %s", peer.phase, inbound.Name)

	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		for {
			select {
			// go ssh.DiscardRequests(channelReqs)
			case req := <-channelReqs:
				if req == nil {
					logger.Printf("%s phase: Inbound %s channelReqs closed", peer.phase, inbound.Name)
					tcpConn.Close()
					sshChan.Close()
					return
				}
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
			case <-peer.done:
				tcpConn.Close()
				sshChan.Close()
				return
			}
		}
	}()

	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		_, err = io.Copy(tcpConn, sshChan)
		tcpConn.Close()
		sshChan.Close()
	}()

	peer.wg.Add(1)
	go func() {
		defer peer.wg.Done()
		_, err = io.Copy(sshChan, tcpConn)
		sshChan.Close()
		tcpConn.Close()
	}()
}

// NewOutbound create an outbound and connect to an outgoing server
func (peer *SSHPeer) AddOutbounds(outbounds Outbounds) {
	for _, outbound := range outbounds.list {
		peer.AddOutbound(outbound)
	}
}

func (peer *SSHPeer) Ready() {
	// signal that all outbounds were added
	close(peer.outboundsReady)
}

// NewOutbound create an outbound and connect to an outgoing server
func (peer *SSHPeer) AddInbounds(inbounds Inbounds) {
	for _, inbound := range inbounds.list {
		peer.AddInbound(inbound)
	}
}

// NewOutbound create an outbound and connect to an outgoing server
func (peer *SSHPeer) AddOutbound(outbound *Outbound) {
	peer.outbounds[outbound.Name] = outbound
}

type SID string

func (sid SID) urlModifier(path string) string {
	if strings.HasSuffix(path, PPPrivateKey) {
		return strings.Replace(path, PPSID, fmt.Sprintf("pp-%s/", sid), 1)
	}
	return path
}

func (outbound *Outbound) acceptProxy(chChan ssh.Channel, chReqs <-chan *ssh.Request, sid string, wg *sync.WaitGroup) {
	remoteURL, err := url.Parse("http://" + outbound.OutAddr)
	if err != nil {
		logger.Printf("Outbound %s acceptProxy error parsing address %s: %v", outbound.Name, outbound.OutAddr, err)
		return
	}

	// The proxy is a Handler - it has a ServeHTTP method
	proxy := httputil.NewSingleHostReverseProxy(remoteURL)
	proxy.Transport = http.DefaultTransport
	logger.Printf("Outbound %s acceptProxy setting up for sid %s", outbound.Name, sid)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for req := range chReqs {
			if req == nil {
				chChan.Close()
				return
			}
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Printf("Outbound %s acceptProxy recovered: %v", outbound.Name, r)
			}
			wg.Done()
		}()
		for {
			bufIoReader := bufio.NewReader(chChan)
			if bufIoReader == nil {
				logger.Printf("Outbound %s acceptProxy nothing to read", outbound.Name)
				chChan.Close()
				return
			}
			req, err := http.ReadRequest(bufIoReader)
			if err != nil {
				if err != io.EOF {
					logger.Printf("Outbound %s acceptProxy error in proxy ReadRequest: %v", outbound.Name, err)
					chChan.Close()
					return
				}
			}
			req.URL.Path = SID(sid).urlModifier(req.URL.Path)
			req.URL.Scheme = "http"
			req.URL.Host = outbound.OutAddr
			logger.Printf("Outbound %s acceptProxy modified URL to %s of host %s", outbound.Name, req.URL.Path, req.URL.Host)

			resp, err := proxy.Transport.RoundTrip(req)
			if err != nil {
				logger.Printf("Outbound %s acceptProxy error in proxy.Transport.RoundTrip: %v", outbound.Name, err)
				chChan.Close()
				return
			}

			if err = resp.Write(chChan); err != nil {
				logger.Printf("Outbound %s acceptProxy to %s error in proxy resp.Write: %v", outbound.Name, req.URL.Path, err)
			}
			logger.Printf("Outbound %s acceptProxy to %s status code %d", outbound.Name, req.URL.Path, resp.StatusCode)
		}
	}()
}

func (outbound *Outbound) accept(chChan ssh.Channel, chReqs <-chan *ssh.Request, wg *sync.WaitGroup) {
	tcpConn, err := net.Dial("tcp", outbound.OutAddr)
	if err != nil {
		logger.Printf("Outbound %s accept dial address %s err: %s - closing channel", outbound.Name, outbound.OutAddr, err)
		chChan.Close()
		return
	}

	logger.Printf("Outbound %s accept dial success - connected to %s", outbound.Name, outbound.OutAddr)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for req := range chReqs {
			if req == nil {
				chChan.Close()
				return
			}
			if req.WantReply {
				logger.Printf("Outbound %s accept chReqs closed", outbound.Name)
				_ = req.Reply(false, nil)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = io.Copy(tcpConn, chChan)
		tcpConn.Close()
		chChan.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = io.Copy(chChan, tcpConn)
		chChan.Close()
		tcpConn.Close()
	}()
}
