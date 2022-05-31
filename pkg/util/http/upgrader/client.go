// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type client struct {
	dialer interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
	tlsConfig *tls.Config
	logger    *log.Logger
}

type Option func(*client)

type DialerFunc func(context.Context, string, string) (net.Conn, error)

func (dialer DialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialer(ctx, network, address)
}

func WithDialer(dialer DialerFunc) Option {
	return func(c *client) {
		c.dialer = dialer
	}
}

func WithTLSConfig(config *tls.Config) Option {
	return func(c *client) {
		c.tlsConfig = config
	}
}

func WithLogger(logger *log.Logger) Option {
	return func(c *client) {
		c.logger = logger
	}
}

func (c *client) printf(format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, v...)
	}
}

func SendUpgradeRequest(ctx context.Context, serverURL *url.URL, protocol string, opts ...Option) (Conn, error) {

	c := &client{
		dialer: &net.Dialer{},
	}

	for _, apply := range opts {
		apply(c)
	}

	c.printf("[http/upgrader] connecting to a server: %s", serverURL.String())

	req, err := http.NewRequest("GET", serverURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create an http upgrade request to %s for %s: %w", protocol, serverURL.String(), err)
	}
	req.Header.Add("Connection", "Upgrade")
	req.Header.Add("Upgrade", protocol)

	c.printf("[http/upgrader] dialing to a server: %s", serverURL.String())

	tcpConn, err := c.dialer.DialContext(ctx, "tcp", serverURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s for http upgrade request: %w", serverURL.Host, err)
	}

	var rawConn net.Conn
	switch serverURL.Scheme {
	case "http":
		rawConn = tcpConn
	case "https":
		tlsConn := tls.Client(tcpConn, c.tlsConfig)

		if err = tlsConn.Handshake(); err != nil {
			return nil, fmt.Errorf("failed to handshake TLS protocol with %s for http upgrade request: %w", serverURL.Host, err)
		}
		rawConn = tlsConn
	default:
		return nil, fmt.Errorf("unknown scheme is specified for http upgrade request: %s", serverURL.Scheme)
	}

	conn := &upgradedConn{
		Conn:   rawConn,
		buf:    bufio.NewReader(rawConn),
		waitCh: make(chan struct{}),
	}

	c.printf("[http/upgrader] connected to a server: %s", serverURL.String())

	if err = req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send an http upgrade request to %s: %w", serverURL.Host, err)
	}

	c.printf("[http/upgrader] sent an http upgrade request to %s: %s %s %s", serverURL.String(), req.Method, req.URL.Path, req.Proto)

	res, err := http.ReadResponse(conn.buf, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to receive a response for an http upgrade request to %s from %s: %w", protocol, serverURL.Host, err)
	}

	if res.StatusCode != http.StatusSwitchingProtocols || !strings.EqualFold(res.Header.Get("Upgrade"), protocol) || !strings.EqualFold(res.Header.Get("Connection"), "upgrade") {
		conn.Close()
		return nil, fmt.Errorf("error in a response for an http upgrade request to %s from %s: %v", protocol, serverURL.Host, res)
	}

	c.printf("[http/upgrader] connection upgraded to %s: %s %s", protocol, res.Status, res.Proto)

	return conn, nil
}
