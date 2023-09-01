//go:build libvirt && cgo

package e2e

import (
	libvirtAdaptor "github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/libvirt"
	log "github.com/sirupsen/logrus"

	"bytes"
	"libvirt.org/go/libvirt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLibvirtCreateSimplePod(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreateSimplePod(t, assert)
}

func TestLibvirtCreateConfidentialPod(t *testing.T) {
	assert := LibvirtAssert{}

	// Check DISABLECVM
	cmd := exec.Command("kubectl", "describe", "configMap/peer-pods-cm", "-n", "confidential-containers-system")
	cmd.Env = os.Environ()
	out, err := cmd.Output()

	if err != nil {
		t.Errorf("Unable to determine parse configMap to determine DISABLECVM: [%+v]", err)
	}

	outStr := string(out)
	startStr := "DISABLECVM:\n----\n"
	endStr := "\n"

	start := strings.Index(outStr, startStr)
	start += len(startStr)

	if start == -1 {
		t.Skip("DISABLECVM not found. Skipping testing CVM")
	}

	end := strings.Index(outStr[start:], endStr)
	end = start + end

	outStr = outStr[start:end]

	if outStr == "true" {
		t.Skip("DISABLECVM is true. Skipping testing CVM")
	}

	launchSecurity, err := libvirtAdaptor.GetLaunchSecurityType("qemu:///system")
	if err != nil {
		t.Errorf("Unable to determine machine confidentiality capabilities: [%v]", err)
	}
	switch launchSecurity {
	case libvirtAdaptor.SEV:
		// Cannot do Attestation on host hypervisor (since we don't know if compromised)
		// So can only test if kernel ring buffer messages contain active
		testCommands := []testCommand{
			{
				command:       []string{"bash", "-c", "dmesg | grep \"AMD Secure Encrypted Virtualization (SEV) active\""},
				containerName: "fakename",
				testCommandStdoutFn: func(stdout bytes.Buffer) bool {
					if stdout.String() != "" {
						log.Infof("SEV is enabled based on kernel ring buffer messages")
						return true
					} else {
						log.Infof("SEV is not enabled based on kernel ring buffer messages")
						return false
					}
				},
			},
		}

		doTestCreateConfidentialPod(t, assert, testCommands)
	case libvirtAdaptor.S390PV:
		t.Skip("Unimplemented")
	default:
		t.Skip("No confidential hardware detected on the machine")
	}

}

func TestLibvirtCreatePodWithConfigMap(t *testing.T) {
	skipTestOnCI(t)
	assert := LibvirtAssert{}
	doTestCreatePodWithConfigMap(t, assert)
}

func TestLibvirtCreatePodWithSecret(t *testing.T) {
	assert := LibvirtAssert{}
	doTestCreatePodWithSecret(t, assert)
}

func TestLibvirtCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	skipTestOnCI(t)
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
