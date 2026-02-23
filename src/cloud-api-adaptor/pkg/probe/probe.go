// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"log"
	"net/http"
	"os"
	"time"
)

var logger = log.New(log.Writer(), "[probe/probe] ", log.LstdFlags|log.Lmsgprefix)
var podsReadizProbesDone bool
var checker Checker
var startTime time.Time

const DefaultCCRuntimeClassName string = "kata-remote"

func StartupHandler(w http.ResponseWriter, r *http.Request) {
	opened, err := checker.IsSocketOpen()
	if err != nil {
		logger.Printf("UDS not opened, because %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !opened {
		logger.Printf("UDS not opened")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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

func Start(socketPath string) {
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
	http.HandleFunc("/startup", StartupHandler)
	err = http.ListenAndServe(":"+port, nil)

	if err != nil {
		logger.Printf("failed to start startup probe server, error %s", err)
	}
}
