//go:build aws

package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
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

func (aa AWSAssert) HasPodVM(t *testing.T, id string) {
	// The `id` parameter is not the instance ID but rather the pod's name, so
	// it will need to scan all running pods on the subnet to find one that
	// starts with the prefix.
	podvmPrefix := "podvm-" + id

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
		t.Errorf("Podvm name=%s not found: %v", id, err)
	}

	found := false
	for _, reservation := range describeInstances.Reservations {
		for _, instance := range reservation.Instances {
			// Code == 48 (terminated)
			// Some podvm from previous tests might be on terminated stage
			// so let's ignore them.
			if instance.State.Code != aws.Int32(48) {
				for _, tag := range instance.Tags {
					if *tag.Key == "Name" &&
						strings.HasPrefix(*tag.Value, podvmPrefix) {
						found = true
					}
				}
			}
		}
	}

	if !found {
		t.Errorf("Podvm name=%s not found", id)
	}
}

func (aa AWSAssert) getInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}

func TestAwsCreateSimplePod(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreateSimplePod(t, assert)
}

func TestAwsCreatePodWithConfigMap(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	doTestCreatePodWithConfigMap(t, assert)
}

func TestAwsCreatePodWithSecret(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	doTestCreatePodWithSecret(t, assert)
}

func TestAwsCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	doTestCreatePeerPodContainerWithExternalIPAccess(t, assert)
}

func TestAwsCreatePeerPodWithJob(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodWithJob(t, assert)
}

func TestAwsCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodAndCheckUserLogs(t, assert)
}

func TestAwsCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodAndCheckWorkDirLogs(t, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, assert)
}

func TestAwsCreatePeerPodWithLargeImage(t *testing.T) {
	assert := NewAWSAssert()

	doTestCreatePeerPodWithLargeImage(t, assert)
}

func TestAwsCreatePeerPodWithPVC(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsDeletePod(t *testing.T) {
	assert := NewAWSAssert()
	doTestDeleteSimplePod(t, assert)
}
