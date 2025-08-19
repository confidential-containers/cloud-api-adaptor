package apic

import (
	"context"
	"fmt"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tuntest"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
)

func TestApiC(t *testing.T) {
	testNs, _ := tuntest.NewNamedNS(t, "test-TestApiC")
	defer tuntest.DeleteNamedNS(t, testNs)

	apic := NewAPIClient(7700, testNs.Path())

	s := test.NamespacedHTTPServer(7700, testNs.Path())
	data, err := apic.GetKey("default/sshclient/publicKey")
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	fmt.Println(string(data))
}
