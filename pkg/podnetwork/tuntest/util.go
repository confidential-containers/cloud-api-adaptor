// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package tuntest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

func NewNamedNS(t *testing.T, prefix string) netops.Namespace {
	t.Helper()

	var nsPath string
	name := prefix
	index := 1
	for {
		var err error
		nsPath, err = netops.CreateNamedNamespace(name)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("failed to create a named network namespace %s: %v", name, err)
		}
		index++
		name = fmt.Sprintf("%s-%d", prefix, index)
	}

	ns, err := netops.OpenNamespace(nsPath)
	if err != nil {
		t.Fatalf("failed to open a named network namespace %s: %v", nsPath, err)
	}

	link, err := ns.LinkFind("lo")
	if err != nil {
		t.Fatalf("failed to find an interface: %v", err)
	}

	if err := link.SetUp(); err != nil {
		t.Fatalf("failed to set the link up: lo")
	}

	return ns
}

func DeleteNamedNS(t *testing.T, ns netops.Namespace) {
	t.Helper()

	if err := ns.Close(); err != nil {
		t.Fatal("failed to close a network namespace")
	}
	if err := netops.DeleteNamedNamespace(filepath.Base(ns.Path())); err != nil {
		t.Fatalf("failed to delete a named network namespace: %s: %v", ns.Path(), err)
	}
}

func BridgeAdd(t *testing.T, ns netops.Namespace, name string) {
	t.Helper()

	br, err := ns.LinkAdd(name, &netops.Bridge{})
	if err != nil {
		t.Fatalf("failed to create a bridge: %v", err)
	}
	if err := br.SetUp(); err != nil {
		t.Fatalf("failed to set the link up: %s: %v", name, err)
	}
}

func LinkSetMaster(t *testing.T, ns netops.Namespace, name, masterName string) {
	t.Helper()

	link, err := ns.LinkFind(name)
	if err != nil {
		t.Fatalf("failed to find an interface: %v", err)
	}
	master, err := ns.LinkFind(masterName)
	if err != nil {
		t.Fatalf("failed to find an interface: %v", err)
	}

	if err := link.SetMaster(master); err != nil {
		t.Fatalf("failed to set the master link of %s to %s: %v", name, masterName, err)
	}
}

func VethAdd(t *testing.T, ns netops.Namespace, name string, peerNamespace netops.Namespace, peerName string) {
	t.Helper()

	link, err := ns.LinkAdd(name, &netops.VEth{PeerName: peerName, PeerNamespace: peerNamespace})
	if err != nil {
		t.Fatalf("failed to create a veth pair between two namespaces: %v", err)
	}
	if err := link.SetUp(); err != nil {
		t.Fatalf("failed to set the link up: %s: %v", name, err)
	}
	peer, err := peerNamespace.LinkFind(peerName)
	if err != nil {
		t.Fatalf("failed to find interface %q on %s: %v", peerName, peerNamespace.Path(), err)
	}
	if err := peer.SetUp(); err != nil {
		t.Fatalf("failed to set the link up: %s: %v", peerName, err)
	}
}

func AddrAdd(t *testing.T, ns netops.Namespace, name, addr string) {
	t.Helper()

	ip, err := netlink.ParseIPNet(addr)
	if err != nil {
		t.Fatalf("failed to parse IP %s: %v", addr, err)
	}
	link, err := ns.LinkFind(name)
	if err != nil {
		t.Fatalf("failed to find an interface: %v", err)
	}
	if err := link.AddAddr(ip); err != nil {
		t.Fatalf("failed to add %s to %s: %v", addr, name, err)
	}
}

func HwAddrAdd(t *testing.T, ns netops.Namespace, name, hwAddr string) {
	t.Helper()

	link, err := ns.LinkFind(name)
	if err != nil {
		t.Fatalf("failed to find an interface: %v", err)
	}
	if err := link.SetHardwareAddr(hwAddr); err != nil {
		t.Fatalf("failed to add %s to %s: %v", hwAddr, name, err)
	}
}

func RouteAdd(t *testing.T, ns netops.Namespace, dest, gw, dev string) {
	t.Helper()

	if dest == "" {
		dest = "0.0.0.0/0"
	}
	_, destNet, err := net.ParseCIDR(dest)
	if err != nil {
		t.Fatalf("failed to parse CIDR %s: %v", dest, err)
	}
	var gwIP net.IP
	if gw != "" {
		gwIP = net.ParseIP(gw)
		if gwIP == nil {
			t.Fatalf("failed to parse IP %s: %v", gw, err)
		}
	}
	if err := ns.RouteAdd(&netops.Route{Destination: destNet, Gateway: gwIP, Device: dev}); err != nil {
		t.Fatalf("failed to add a route to %s via %s: %v", dest, gw, err)
	}
}

type httpHandler string

const testHTTPHandler = httpHandler("Hello")

func (h httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s", string(h))
}

type TestHTTPServer struct {
	ns       netops.Namespace
	listener net.Listener
	server   *http.Server
}

func StartHTTPServer(t *testing.T, ns netops.Namespace, addr string) *TestHTTPServer {
	t.Helper()
	s := &TestHTTPServer{
		ns: ns,
	}

	err := ns.Run(func() error {

		var err error
		s.server = &http.Server{
			Handler: testHTTPHandler,
		}
		s.listener, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %v", addr, err)
		}
		go func() {
			err := s.server.Serve(s.listener)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(os.Stderr, "failed to start an HTTP server: %v", err)
			}
		}()
		return nil
	})
	if err != nil {
		t.Fatalf("failed to run a function at a network namespace: %v", err)
	}
	return s
}

func (s *TestHTTPServer) Shutdown(t *testing.T) {
	t.Helper()

	err := s.ns.Run(func() error {
		err := s.server.Shutdown(context.Background())
		s.listener.Close()
		return err
	})
	if err != nil {
		t.Fatal("failed to run a function at a network namespace")
	}
}

func ConnectToHTTPServer(t *testing.T, ns netops.Namespace, addr, localAddr string) {
	t.Helper()

	var tcpAddr net.Addr
	if localAddr != "" {
		var err error
		tcpAddr, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:0", localAddr))
		if err != nil {
			t.Fatalf("failed to get TCP address: %s", localAddr)
		}
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (conn net.Conn, err error) {
				if e := ns.Run(func() error {
					d := &net.Dialer{
						LocalAddr: tcpAddr,
					}
					conn, err = d.DialContext(ctx, network, address)
					return nil
				}); e != nil {
					t.Fatalf("failed to run a dialer at network namespace %s", ns.Path())
				}
				return conn, err
			},
		},
	}

	if err := ns.Run(func() error {

		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", addr), nil)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()

		req = req.WithContext(ctx)

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get an http response at %s from http://%s : %v", ns.Path(), addr, err)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read an http response at %s from http://%s:: %v", ns.Path(), addr, err)
		}
		content := string(body)
		if content != string(testHTTPHandler) {
			return fmt.Errorf("unexpected response at %s from the HTTP server: %s", ns.Path(), content)
		}
		return nil

	}); err != nil {
		t.Fatalf("failed to run a HTTP client at a network namespace: %v", err)
	}
}

func CheckVRF(t *testing.T, ns netops.Namespace) {
	t.Helper()

	name := "vrf0dummy"

	vrf, err := ns.LinkAdd(name, &netops.VRF{Table: 12345})
	if err != nil {

		if errors.Is(err, unix.ENOTSUP) {
			t.Log(
				"vrf is not enabled in the Linux kernel\n",
				"========================================================\n",
				"  Please load the vrf module.\n",
				"  In Ubuntu, run the following commands.\n",
				"    apt-get install \"linux-modules-extra-$(uname -r)\"\n",
				"    modprobe vrf\n",
				"========================================================\n",
			)
		}

		t.Fatalf("failed to create a VRF interface: %v\n", err)
	}

	if err := vrf.Delete(); err != nil {
		t.Fatalf("failed to delete a VRF interface %s: %v\n", name, err)
	}
}
