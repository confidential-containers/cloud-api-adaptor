// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendUpgradeRequestErrorConnectServer(t *testing.T) {
	serverURL := "example.com"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: "httpinvalid", Host: serverURL, Path: "/upgrade"}, "test", WithLogger(log.Default()))
	assert.NotNil(t, err)
	if assert.NotNil(t, err) {
		expectedErrMsg := fmt.Sprintf("failed to connect to %s for http upgrade request: dial tcp: address %s: missing port in address", serverURL, serverURL)
		assert.EqualError(t, err, expectedErrMsg, "Verify failed to connect to server")
	}

}

func TestSendUpgradeRequestErrorRequest(t *testing.T) {
	serverURL := "ibm.com/"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: "httpinvalid", Host: serverURL, Path: "/upgrade"}, "test", WithLogger(log.Default()))
	assert.NotNil(t, err)
	if assert.NotNil(t, err) {
		expectedErrMsg := "failed to create an http upgrade request to test for"
		assert.Contains(t, err.Error(), expectedErrMsg, "Verify failed to create http request")
	}

}

func TestSendUpgradeRequestInvalidScheme(t *testing.T) {
	listener, error := net.Listen("tcp", "127.0.0.1:0")
	if error != nil {
		t.Fatalf("Expect no error, got %v", error)
	}
	defer listener.Close()
	addr := listener.Addr().String()
	invalidScheme := "htttttp"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: invalidScheme, Host: addr, Path: "/upgrade"}, "test", WithLogger(log.Default()))
	assert.NotNil(t, err)
	if assert.NotNil(t, err) {

		expectedErrMsg := fmt.Sprintf("unknown scheme is specified for http upgrade request: %s", invalidScheme)
		assert.EqualError(t, err, expectedErrMsg, "Verify invalid scheme of http request")
	}
}

func TestSendUpgradeRequestHttpsError(t *testing.T) {
	listener, error := net.Listen("tcp", "127.0.0.1:0")
	if error != nil {
		t.Fatalf("Expect no error, got %v", error)
	}
	defer listener.Close()
	addr := listener.Addr().String()
	scheme := "https"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: scheme, Host: addr, Path: "/upgrade"}, "test", WithLogger(log.Default()))
	assert.NotNil(t, err)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "failed to handshake TLS protocol with 127.0.0.1", "verify failed to handshake")
	}
}

func TestWithDialerFunc(t *testing.T) {
	rs := WithDialer(nil)
	assert.Equalf(t, "upgrader.Option", reflect.TypeOf(rs).String(), "verify the withTLSConfig func")
}

func TestWithTLSConfigFunc(t *testing.T) {
	rs := WithTLSConfig(nil)
	assert.Equalf(t, "upgrader.Option", reflect.TypeOf(rs).String(), "verify the withTLSConfig func")
}

func TestSendUpgradeRequestErrorResp(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "This is unit test.")
	}))
	defer ts.Close()
	tsHost := ts.Listener.Addr().String()
	scheme := "http"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: scheme, Host: tsHost, Path: "/upgrade"}, "test")

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "error in a response for an http upgrade request to test", "verify error response")
	}
}
func TestSendUpgradeRequestWithConfig(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %s", r.Proto)
	}))
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()
	tsHost := ts.Listener.Addr().String()
	scheme := "http"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: scheme, Host: tsHost, Path: "/upgrade"}, "test", WithTLSConfig(nil))
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "error in a response for an http upgrade request to", "verify withTLSConfig")
	}
}

type mockDialer struct {
}

func (d *mockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	return conn, err
}

func TestSendUpgradeRequestNoResp(t *testing.T) {

	c := &client{
		dialer: &mockDialer{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCodeLessThan100 := 10
		w.WriteHeader(statusCodeLessThan100)
		fmt.Fprintln(w, "Mocked test with incorrect header.")

	}))
	defer ts.Close()

	tsHost := ts.Listener.Addr().String()

	upgradeDialer := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return c.dialer.DialContext(context.Background(), "tcp", tsHost)
	}
	scheme := "http"
	_, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: scheme, Host: tsHost, Path: "/upgrade"}, "test", WithDialer(upgradeDialer))
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "failed to receive a response for an http upgrade request", "verify response error")
	}
}
