// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/aws"
)

// AWSAssert implements the CloudAssert interface.
type AWSAssert struct {
	Vpc *pv.Vpc
}

func NewAWSAssert() AWSAssert {
	return AWSAssert{
		Vpc: pv.AWSProps.Vpc,
	}
}

func (aa AWSAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (aa AWSAssert) HasPodVM(t *testing.T, podvmName string) {

	// validating if podvm exists with provided podvmName
	describeInstances, err := aa.Vpc.Client.DescribeInstances(context.TODO(),
		&ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String("subnet-id"),
					Values: []string{aa.Vpc.SubnetId},
				},
			},
		})
	if err != nil {
		t.Errorf("Podvm name=%s not found: %v", podvmName, err)
	}

	found := false
	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
			// Code == 48 (terminated)
			// Some podvm from previous tests might be on terminated stage
			// so let's ignore them.
			if instance.State.Code != aws.Int32(48) {
				for _, tag := range instance.Tags {
					if *tag.Key == "Name" && *tag.Value == podvmName {
						found = true
					}
				}
			}
		}
	}

	if !found {
		t.Errorf("Podvm name=%s not found", podvmName)
	}
}

func (aa AWSAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}

func (aa AWSAssert) VerifyPodvmConsole(t *testing.T, podvmName, expectedString string) {
	// Verify PodVM console output with provided expectedString
	// This is not implemented for AWS as of now.
	// So skipping this test.
	t.Log("Warning: console verification is not added for AWS")
}
