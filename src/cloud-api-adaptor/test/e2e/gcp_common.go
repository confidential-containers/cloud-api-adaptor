// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
)

// GCPAssert implements the CloudAssert interface.
type GCPAssert struct {
	Vpc *pv.GCPVPC
}

func NewGCPAssert() GCPAssert {
	return GCPAssert{
		Vpc: pv.GCPProps.GcpVPC,
	}
}

func (aa GCPAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (aa GCPAssert) HasPodVM(t *testing.T, id string) {
	// // The `id` parameter is not the instance ID but rather the pod's name, so
	// // it will need to scan all running pods on the subnet to find one that
	// // starts with the prefix.
	// podvmPrefix := "podvm-" + id

	// describeInstances, err := aa.Vpc.Client.DescribeInstances(context.TODO(),
	// 	&ec2.DescribeInstancesInput{
	// 		Filters: []ec2types.Filter{
	// 			{
	// 				Name:   aws.String("subnet-id"),
	// 				Values: []string{aa.Vpc.SubnetId},
	// 			},
	// 		},
	// 	})
	// if err != nil {
	// 	t.Errorf("Podvm name=%s not found: %v", id, err)
	// }

	// found := false
	// for _, reservation := range describeInstances.Reservations {
	// 	for _, instance := range reservation.Instances {
	// 		// Code == 48 (terminated)
	// 		// Some podvm from previous tests might be on terminated stage
	// 		// so let's ignore them.
	// 		if instance.State.Code != aws.Int32(48) {
	// 			for _, tag := range instance.Tags {
	// 				if *tag.Key == "Name" &&
	// 					strings.HasPrefix(*tag.Value, podvmPrefix) {
	// 					found = true
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	// if !found {
	// 	t.Errorf("Podvm name=%s not found", id)
	// }
}

func (aa GCPAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	return "", nil
}
