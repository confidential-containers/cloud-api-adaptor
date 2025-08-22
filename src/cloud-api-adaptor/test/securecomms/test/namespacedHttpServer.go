package test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

func listen(nsPath string, port uint16) (tcpListener *net.TCPListener, err error) {
	addr, err := netip.ParseAddr("127.0.0.1")
	addPort := netip.AddrPortFrom(addr, port)
	tcpAddr := net.TCPAddrFromAddrPort(addPort)

	runErr := netops.RunAsNsPath(nsPath, func() error {
		tcpListener, err = net.ListenTCP("tcp", tcpAddr)
		return nil
	})
	if runErr != nil {
		return nil, runErr
	}
	return
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("HttpServer got request %s\n", r.URL)

	if _, err := io.WriteString(w, "this is my website!\n"); err != nil {
		fmt.Printf("HttpServer failed writing response: %v\n", err)
	}
}

func NamespacedHTTPServer(port uint16, nsPath string) *http.Server {
	tcpListener, err := listen(nsPath, port)
	if err != nil {
		log.Printf("failed to listen to namespace %s port %d, err: %v", nsPath, port, err)
		return nil
	}
	_, retPort, err := net.SplitHostPort(tcpListener.Addr().String())
	if err != nil {
		panic(err)
	}
	log.Printf("port %s", retPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/", getRoot)

	s := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%s", retPort),
		Handler: mux,
	}

	go func() {
		err := http.Serve(tcpListener, mux)
		if err != http.ErrServerClosed { // graceful shutdown
			fmt.Printf("Serve Error %v\n", err)
		}
	}()
	return s
}
