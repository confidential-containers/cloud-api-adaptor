package test

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

type myport string

func (p myport) getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("HttpServer port %s got request %s\n", p, r.URL)

	if _, err := io.WriteString(w, fmt.Sprintf("port %s - this is my website!\n", p)); err != nil {
		fmt.Printf("HttpServer failed writing response: %v\n", err)
	}
}

func HTTPServer(port string) *http.Server {
	p := myport(port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.getRoot)
	s := &http.Server{
		Addr:    "127.0.0.1:" + port,
		Handler: mux,
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Printf("Listen Error %v\n", err)
		return nil
	}

	go func() {
		err = s.Serve(ln)
		if err != http.ErrServerClosed { // graceful shutdown
			fmt.Printf("Serve Error %v\n", err)
		}
	}()
	return s
}
