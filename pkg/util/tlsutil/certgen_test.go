// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCertificate(t *testing.T) {

	serverName := "server1"

	caService, err := NewCAService("agent-protocol-forwarder")
	assert.NoError(t, err)

	serverCACertPEM := caService.RootCertificate()
	assert.NotNil(t, serverCACertPEM)

	serverCertPEM, serverKeyPEM, err := caService.Issue(serverName)
	assert.NoError(t, err)

	clientCertPEM, clientKeyPEM, err := NewClientCertificate("cloud-api-adaptor")
	assert.NoError(t, err)

	serverConfig, err := GetTLSConfigFor(&TLSConfig{CAData: clientCertPEM, CertData: serverCertPEM, KeyData: serverKeyPEM})
	assert.NoError(t, err)

	clientConfig, err := GetTLSConfigFor(&TLSConfig{CAData: serverCACertPEM, CertData: clientCertPEM, KeyData: clientKeyPEM})
	assert.NoError(t, err)

	clientConfig.ServerName = serverName

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	require.NoError(t, err)

	addr := listener.Addr().String()

	recvCh := make(chan string)

	go func() {
		defer close(recvCh)

		conn, err := listener.Accept()
		require.NoError(t, err)

		var buf = make([]byte, 64)
		n, err := conn.Read(buf)
		assert.NoError(t, err)

		recvCh <- string(buf[:n])
	}()

	dialer := tls.Dialer{
		Config: clientConfig,
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	require.NoError(t, err)

	defer conn.Close()

	msg := "Hello!"

	n, err := conn.Write([]byte(msg))
	assert.NoError(t, err)
	assert.Equal(t, len(msg), n)

	recv := <-recvCh

	assert.Equal(t, recv, msg)
}
