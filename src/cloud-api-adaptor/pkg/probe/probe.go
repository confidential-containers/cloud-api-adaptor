// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var logger = log.New(log.Writer(), "[probe/probe] ", log.LstdFlags|log.Lmsgprefix)
var podsReadizProbesDone bool
var checker Checker
var startTime time.Time

const DefaultCCRuntimeClassName string = "kata-remote"

func StartupHandler(w http.ResponseWriter, r *http.Request) {
	if !podsReadizProbesDone {
		ret, err := checker.GetAllPeerPods(startTime)
		podsReadizProbesDone = ret
		if err != nil || !podsReadizProbesDone {
			logger.Printf("Not all PeerPods ready, because %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	logger.Printf("All PeerPods standup. we do not check the PeerPods status any more.")
	w.WriteHeader(http.StatusOK)
}

func Start(ctx context.Context, socketPath string) {
	startTime = time.Now()

	port := os.Getenv("PROBE_PORT")
	if port == "" {
		port = "8000"
	}
	logger.Printf("Using port: %s", port)
	podsReadizProbesDone = false

	clientset, err := CreateClientset()
	if err != nil {
		logger.Printf("failed to CreateClientset, error %s", err)
		return
	}
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: GetRuntimeclassName(),
		SocketPath:       socketPath,
	}
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
					logger.Printf("failed to set SO_REUSEPORT: %v", err)
				}
			})
		},
	}
	ln, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		logger.Printf("failed to listen on probe port: %s", err)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/startup", StartupHandler)
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("probe server shutdown error: %v", err)
		}
	}()
	if err = server.Serve(ln); err != nil && err != http.ErrServerClosed {
		logger.Printf("failed to start startup probe server, error %s", err)
	}
}
