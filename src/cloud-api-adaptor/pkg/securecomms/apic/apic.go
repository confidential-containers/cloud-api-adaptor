package apic

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

var logger = sshutil.Logger

type APIClient struct {
	servicePort uint16
	nsPath      string
}

// Create a client for API_SERVER_REST service
func NewAPIClient(servicePort uint16, nsPath string) *APIClient {
	return &APIClient{
		servicePort: servicePort,
		nsPath:      nsPath,
	}
}

// GetKey uses kbs-client to obtain keys such as pp-sid/privateKey, sshclient/publicKey
func (c *APIClient) GetKey(key string) (data []byte, err error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/cdh/resource/default/%s", c.servicePort, key)

	client := http.Client{
		Transport: &http.Transport{
			// Override the DialContext to enable connecting to a service at the pod namespace
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				runErr := netops.RunAsNsPath(c.nsPath, func() error {
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

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getKey %s client.Get err: %w", key, err)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getKey %s io.ReadAll err - %w", key, err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("getKey %s client.Get received error code of : %d - %s", key, resp.StatusCode, string(data))
	}

	logger.Printf("getKey %s statusCode %d success", key, resp.StatusCode)
	return
}
