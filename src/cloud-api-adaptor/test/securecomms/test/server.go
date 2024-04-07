package test

import (
	"fmt"
	"net"
)

func Server(port string) {
	// Listen for incoming connections on port
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Accept incoming connections and handle them
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Handle the connection in a new goroutine
		go handleConnection(conn, port)
	}
}

func handleConnection(conn net.Conn, port string) {
	// Close the connection when we're done
	defer conn.Close()
	for {

		// Read incoming data
		buf := make([]byte, 1024)
		_, err := conn.Read(buf)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Print the incoming data
		fmt.Printf("Received: %s port %s\n", buf, port)
		_, err = conn.Write(buf)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Written: %s port %s\n", buf, port)
	}
}
