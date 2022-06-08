// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package routing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"syscall"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"golang.org/x/sys/unix"
)

// Keep alive server is only used to work around the following issue on IBM Cloud.
//
// Conditions:
// * Two VM instances A and B are provisioned with two network interfaces each
// * A network namespace is configured on each instance
// * Veth interfaces and routing tables are configured to enable network communication
//   between the two network namespaces via the second network interfaces of the instances.
// * "Allow IP Spoofing" option is enabled for each network interface
// * No communication is made before an experiment between the second network interfaces
//   of the two instances.
//
// Step to reproduce the issue:
// * Open a TCP connection from A and B
//   Note that this connection requires routing, and requires "Allow IP spoofing" to be enabled.
//
// Expected result:
// * A TCP connection is successfully established
//
// Actual result:
// * A SYN packet from A is successfully delivered to B
// * Then, a SYN+ACK packet from B is successfully delivered to A
// * Then, an ACK packet from A is lost and NOT delivered to B
//
// Cause of the issue:
// * Not yet identified
//
// Work around:
// * Periodically establish TCP connections that does not require routing
//   using the IP addresses assigned to the second network interfaces
//   before establishing TCP connections that requires routing.
//
// Keep alive sever and client implement this work around solution

const (
	defaultKeepAliveListenPort = "10345"
	defaultKeepAliveListenAddr = "0.0.0.0:" + defaultKeepAliveListenPort
)

var keepAlive = struct {
	clientStopCh map[string]chan struct{}
	serverStopCh chan struct{}
	mutex        sync.Mutex
}{
	clientStopCh: make(map[string]chan struct{}),
}

func getKey(podIP net.IP, atWorkerNode bool) string {
	var prefix string
	if atWorkerNode {
		prefix = "worker:"
	} else {
		prefix = "pod:"
	}
	return prefix + podIP.String()
}

func startKeepAlive(ns *netops.NS, podIP net.IP, address string, atWorkerNode bool) error {

	keepAlive.mutex.Lock()
	defer keepAlive.mutex.Unlock()

	if _, ok := keepAlive.clientStopCh[getKey(podIP, atWorkerNode)]; ok {
		return fmt.Errorf("keep alive for pod IP %s is already configured", podIP)
	}

	if atWorkerNode && len(keepAlive.clientStopCh) == 0 {
		keepAlive.serverStopCh = make(chan struct{})
		if err := launchKeepAliveServer(ns, true, keepAlive.serverStopCh); err != nil {
			return err
		}
	}

	clientStopCh := make(chan struct{})
	keepAlive.clientStopCh[getKey(podIP, atWorkerNode)] = clientStopCh
	logger.Printf("keep alive for pod IP %s registered", podIP)

	return launchKeepAliveClient(ns, address, clientStopCh, withVRF(atWorkerNode))
}

func stopKeepAlive(podIP net.IP, atWorkerNode bool) error {

	keepAlive.mutex.Lock()
	defer keepAlive.mutex.Unlock()

	clientStopCh, ok := keepAlive.clientStopCh[getKey(podIP, atWorkerNode)]
	if !ok {
		return fmt.Errorf("keep alive for pod IP %s is not found", podIP)
	}

	close(clientStopCh)
	delete(keepAlive.clientStopCh, getKey(podIP, atWorkerNode))

	if atWorkerNode && len(keepAlive.clientStopCh) == 0 {
		close(keepAlive.serverStopCh)
	}

	return nil
}

func control(network string, address string, c syscall.RawConn) (err error) {

	e := c.Control(func(fd uintptr) {
		err = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, vrf1Name)
	})
	if e != nil {
		return e
	}

	return err
}

type keepAliveClientConfig struct {
	interval time.Duration
	timeout  time.Duration
	useVRF   bool
}

func withVRF(useVRF bool) func(*keepAliveClientConfig) {
	return func(cfg *keepAliveClientConfig) {
		cfg.useVRF = useVRF
	}
}

func withInterval(interval time.Duration) func(*keepAliveClientConfig) {
	return func(cfg *keepAliveClientConfig) {
		cfg.interval = interval
	}
}

func withTimeout(timeout time.Duration) func(*keepAliveClientConfig) {
	return func(cfg *keepAliveClientConfig) {
		cfg.timeout = timeout
	}
}

func launchKeepAliveClient(targetNS *netops.NS, serverAddr string, stopCh chan struct{}, options ...func(*keepAliveClientConfig)) error {

	cfg := keepAliveClientConfig{
		interval: time.Minute * 5,
		timeout:  5 * time.Second,
	}

	for _, f := range options {
		f(&cfg)
	}

	dialer := &net.Dialer{}
	if cfg.useVRF {
		dialer.Control = control
	}

	ns, err := targetNS.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone a network namespace: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				runErr := ns.Run(func() error {
					conn, err = dialer.DialContext(ctx, network, addr)
					return err
				})
				if err == nil && runErr != nil {
					err = runErr
				}
				return
			},
		},
	}

	serverURL := (&url.URL{Scheme: "http", Host: serverAddr, Path: "/"}).String()

	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create a HTTP request for keep alive: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()

	go func() {
		ticker := time.NewTicker(cfg.interval)
		defer ticker.Stop()

		defer ns.Close()

		for {
			select {
			case <-stopCh:
				return
			default:
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.timeout)

			logger.Printf("connecting to keep alive server %s", serverURL)

			res, err := client.Do(req.WithContext(ctx))
			if err != nil {
				logger.Printf("failed to connect to a keep alive server: %s: %v", serverURL, err)
				<-ctx.Done()
				cancel()
				continue
			}
			cancel()

			res.Body.Close()

			<-ticker.C
		}
	}()

	return nil
}

func launchKeepAliveServer(targetNS *netops.NS, useVRF bool, stopCh chan struct{}) error {

	listenAddr := defaultKeepAliveListenAddr

	listenConfig := &net.ListenConfig{}

	if useVRF {
		listenConfig.Control = control
	}

	ns, err := targetNS.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone a network namespace: %w", err)
	}

	var listener net.Listener
	if err := ns.Run(func() error {
		var err error
		listener, err = listenConfig.Listen(context.Background(), "tcp", listenAddr)
		return err
	}); err != nil {
		return fmt.Errorf("failed to listen on %s for keep alive: %w", listenAddr, err)
	}

	httpServer := http.Server{
		Addr: listenAddr,
	}

	go func() {
		defer ns.Close()

		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("error running http server for keep alive: %v", err)
		}
	}()

	go func() {
		<-stopCh
		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Printf("error on shutdown of http server for keep alive: %v", err)
		}
	}()

	return nil
}
