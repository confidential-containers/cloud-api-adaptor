package apic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/securecomms/test"
	"github.com/google/uuid"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func TestApiC(t *testing.T) {
	namespace := uuid.NewString()
	nsPath := "/run/netns/" + namespace
	// Create a new network namespace
	newns, err := netns.NewNamed(namespace)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		if errors.Is(err, fs.ErrPermission) {
			t.Skip("Skip due to missing permissions - run privileged!")
		}
		t.Errorf("netns.NewNamed(%s) returned err %s", namespace, err.Error())
	}
	defer func() {
		newns.Close()
		if err := netns.DeleteNamed(namespace); err != nil {
			t.Errorf("failed to delete a named network namespace %s: %v", namespace, err)
		}
	}()

	link, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	apic := NewApiClient(7700, nsPath)

	s := test.NamespacedHttpServer(7700, nsPath)
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
