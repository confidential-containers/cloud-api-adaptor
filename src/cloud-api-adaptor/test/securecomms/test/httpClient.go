package test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

func HTTPClient(dest string) bool {
	fmt.Printf("HttpClient start : %s\n", dest)

	fmt.Printf("HttpClient sending req: %s\n", dest)
	c := http.Client{}
	resp, err := c.Get(dest)
	if err != nil {
		fmt.Printf("HttpClient %s Error %s\n", dest, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("HttpClient %s ReadAll Error %s\n", dest, err)
		return false
	}
	fmt.Printf("HttpClient %s Body : %s\n", dest, body)
	return (resp.StatusCode == 200)
}

func HTTPClientInNamespace(dest string, nsPath string) bool {
	fmt.Printf("HttpClient start : %s in namepspace: %s\n", dest, nsPath)

	c := http.Client{
		Transport: &http.Transport{
			// Override the DialContext to enable connecting to a service at the pod namespace
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				runErr := netops.RunAsNsPath(nsPath, func() error {
					fmt.Printf("HttpClient dialing req: %s in namepspace: %s\n", addr, nsPath)
					conn, err = (&net.Dialer{}).DialContext(ctx, network, addr)
					return nil
				})
				if runErr != nil {
					return nil, runErr
				}
				return
			},
		},
	}

	resp, err := c.Get(dest)
	if err != nil {
		fmt.Printf("HttpClient %s Get Error %s\n", dest, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("HttpClient %s ReadAll Error %s\n", dest, err)
		return false
	}
	fmt.Printf("HttpClient %s StatusCode %d Body : %s\n", dest, resp.StatusCode, body)
	return (resp.StatusCode == 200)
}
