//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"strings"
	"testing"
)

func TestCreateSimplePod(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreateSimplePod(t, assert)
}

func TestCreateConfidentialPod(t *testing.T) {
	instanceProfile := pv.IBMCloudProps.InstanceProfile
	if strings.HasPrefix(instanceProfile, "bz2e") {
		log.Infof("Test SE pod")
		assert := IBMCloudAssert{
			vpc: pv.IBMCloudProps.VPC,
		}

		testCommands := []testCommand{
			{
				command:       []string{"cat", "/sys/firmware/uv/prot_virt_guest"},
				containerName: "fakename", //container name will be updated after pod is created.
				testCommandStdoutFn: func(stdout bytes.Buffer) bool {
					trimmedStdout := strings.Trim(stdout.String(), "\n")
					if trimmedStdout == "1" {
						log.Infof("The pod is SE pod based on content of prot_virt_guest file: %s", stdout.String())
						return true
					} else {
						log.Infof("The pod is non SE pod based on content of prot_virt_guest file: %s", stdout.String())
						return false
					}
				},
			},
			{
				command:       []string{"grep", "facilities", "/proc/cpuinfo"},
				containerName: "fakename", //container name will be updated after pod is created.
				testCommandStdoutFn: func(stdout bytes.Buffer) bool {
					if strings.Contains(stdout.String(), "158") {
						log.Infof("The pod is SE pod based on facilities of /proc/cpuinfo file: %s", stdout.String())
						return true
					} else {
						log.Infof("The pod is non SE pod based on facilities of /proc/cpuinfo file: %s", stdout.String())
						return false
					}
				},
			},
		}
		doTestCreateConfidentialPod(t, assert, testCommands)
	} else {
		log.Infof("Ignore SE test for simple pod")
	}

}

func TestCreatePodWithConfigMap(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePodWithConfigMap(t, assert)
}

func TestCreatePodWithSecret(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePodWithSecret(t, assert)
}

func TestCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePeerPodContainerWithExternalIPAccess(t, assert)
}
func TestCreatePeerPodWithJob(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePeerPodWithJob(t, assert)
}

func TestCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePeerPodAndCheckUserLogs(t, assert)
}

func TestCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreatePeerPodAndCheckWorkDirLogs(t, assert)
}

// IBMCloudAssert implements the CloudAssert interface for ibmcloud.
type IBMCloudAssert struct {
	vpc *vpcv1.VpcV1
}

func (c IBMCloudAssert) HasPodVM(t *testing.T, id string) {
	log.Infof("PodVM name: %s", id)
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.vpc.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	for i, instance := range instances.Instances {
		name := *instance.Name
		log.Debugf("Instance number: %d, Instance id: %s, Instance name: %s", i, *instance.ID, name)
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			return
		}
	}
	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}
