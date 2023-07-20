//go:build libvirt && cgo

package e2e

import (
	"libvirt.org/go/libvirt"
	"strings"
	"testing"
)

func TestLibvirtCreateSimplePod(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreateSimplePod(t, assert)
}

func TestLibvirtCreatePodWithConfigMap(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePodWithConfigMap(t, assert)
}

func TestLibvirtCreatePodWithSecret(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePodWithSecret(t, assert)
}

func TestLibvirtCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodContainerWithExternalIPAccess(t, assert)

}

func TestLibvirtCreatePeerPodWithJob(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodWithJob(t, assert)
}

func TestLibvirtCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodAndCheckUserLogs(t, assert)
}

func TestLibvirtCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodAndCheckWorkDirLogs(t, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, assert)
}

/*
Failing due to issues will pulling image (ErrImagePull)
func TestLibvirtCreatePeerPodWithLargeImage(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePeerPodWithLargeImage(t, assert)
}
*/

// LibvirtAssert implements the CloudAssert interface for Libvirt.
type LibvirtAssert struct {
	// TODO: create the connection once on the initializer.
	//conn libvirt.Connect
}

func (l LibvirtAssert) HasPodVM(t *testing.T, id string) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		t.Fatal(err)
	}

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	for _, dom := range domains {
		name, _ := dom.GetName()
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			return
		}
	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}
