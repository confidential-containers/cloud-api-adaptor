//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"strings"
	"testing"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

func TestCreateSimplePod(t *testing.T) {
	assert := IBMCloudAssert{
		vpc: pv.IBMCloudProps.VPC,
	}
	doTestCreateSimplePod(t, assert)
}

// IBMCloudAssert implements the CloudAssert interface for ibmcloud.
type IBMCloudAssert struct {
	vpc *vpcv1.VpcV1
}

func (c IBMCloudAssert) HasPodVM(t *testing.T, id string) {
	fmt.Println("PodVM name: ", id)
	options := &vpcv1.ListInstancesOptions{}
	instances, _, err := c.vpc.ListInstances(options)

	if err != nil {
		t.Fatal(err)
	}

	for i, instance := range instances.Instances {
		name := *instance.Name
		fmt.Println("Instance number: ", i, " Instance id: ", *instance.ID, " Instance name: ", name)
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			return
		}
	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}
