// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ibmcloud"
	log "github.com/sirupsen/logrus"
)

func CreateConfidentialPodCheckIBMSECommands() []TestCommand {
	testCommands := []TestCommand{
		{
			Command:       []string{"cat", "/sys/firmware/uv/prot_virt_guest"},
			ContainerName: "fakename", //container name will be updated after pod is created.
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
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
			Command:       []string{"grep", "facilities", "/proc/cpuinfo"},
			ContainerName: "fakename", //container name will be updated after pod is created.
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
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
	return testCommands
}

// IBMCloudAssert implements the CloudAssert interface for ibmcloud.
type IBMCloudAssert struct {
	VPC *vpcv1.VpcV1
}

func (c IBMCloudAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (c IBMCloudAssert) HasPodVM(t *testing.T, podvmName string) {
	log.Infof("PodVM name: %s", podvmName)
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	for i, instance := range instances.Instances {
		name := *instance.Name
		log.Debugf("Instance number: %d, Instance id: %s, Instance name: %s", i, *instance.ID, name)
		if name == podvmName {
			return
		}
	}
	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}

func (c IBMCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		return "", err
	}
	for _, instance := range instances.Instances {
		name := *instance.Name
		if strings.HasPrefix(name, strings.Join([]string{"podvm", podName, ""}, "-")) {
			profile := instance.Profile.Name
			return *profile, nil
		}
	}
	return "", errors.New("Failed to Create PodVM Instance")
}

type IBMRollingUpdateAssert struct {
	VPC *vpcv1.VpcV1
	// cache Pod VM instance IDs for rolling update test
	InstanceIDs [2]string
}

func (c *IBMRollingUpdateAssert) CachePodVMIDs(t *testing.T, deploymentName string) {
	options := &vpcv1.ListInstancesOptions{
		VPCID: &pv.IBMCloudProps.VpcID,
	}
	instances, _, err := c.VPC.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	index := 0
	for i, instance := range instances.Instances {
		name := *instance.Name
		log.Debugf("Instance number: %d, Instance id: %s, Instance name: %s", i, *instance.ID, name)
		if strings.Contains(name, deploymentName) {
			c.InstanceIDs[index] = *instance.ID
			index++
		}
	}
}

func (c *IBMRollingUpdateAssert) VerifyOldVMDeleted(t *testing.T) {
	for _, id := range c.InstanceIDs {
		options := &vpcv1.GetInstanceOptions{
			ID: &id,
		}
		in, _, err := c.VPC.GetInstance(options)

		if err != nil {
			log.Printf("Instance %s has been deleted: %v", id, err)
		} else {
			if *in.Status == "deleting" {
				log.Printf("Instance %s is being deleting", id)
			} else {
				log.Printf("Instance %s current status: %s", id, *in.Status)
				t.Fatalf("Instance %s still exists", id)
			}
		}
	}
}
