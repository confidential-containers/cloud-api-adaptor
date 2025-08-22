package securecomms_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/ppssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
)

func PP() {
	test.HTTPServer("7131")

	// Forwarder Initialization
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	ppSecrets := ppssh.NewPpSecrets(ppssh.GetSecret(getKey))
	ppSecrets.AddKey(ppssh.WNPublicKey)
	ppSecrets.AddKey(ppssh.PPPrivateKey)

	sshServer := ppssh.NewSSHServer([]string{"BOTH_PHASES:KBS:7000", "KUBERNETES_PHASE:KUBEAPI:16443", "KUBERNETES_PHASE:DNS:9053"}, []string{"KUBERNETES_PHASE:KATAAGENT:127.0.0.1:7131"}, ppSecrets, sshutil.SSHPORT)
	_ = sshServer.Start(ctx)
	time.Sleep(1 * time.Minute)
	cancel()
}

// getKey uses kbs-client to obtain keys such as pp-sid/privateKey, sshclient/publicKey
func getKey(key string) (data []byte, err error) {
	url := fmt.Sprintf("http://127.0.0.1:7000/kbs/v0/resource/default/%s", key)

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				log.Printf("getKey client.DialContext() addr: %s", addr)
				return (&net.Dialer{}).DialContext(ctx, network, addr)
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

	log.Printf("getKey %s statusCode %d success", key, resp.StatusCode)
	return
}
