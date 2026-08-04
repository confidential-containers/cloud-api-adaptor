// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/alibabacloud"
)

// AlibabaCloudAssert implements the CloudAssert interface.
type AlibabaCloudAssert struct {
	Vpc *pv.Vpc
}

func NewAlibabaCloudAssert() AlibabaCloudAssert {
	return AlibabaCloudAssert{
		Vpc: pv.AlibabaCloudProps.Vpc,
	}
}

func (aa AlibabaCloudAssert) DefaultTimeout() time.Duration {
	return 2 * time.Minute
}

func (aa AlibabaCloudAssert) ecsClient() (*ecs.Client, error) {
	cfg := &openapi.Config{
		AccessKeyId:     tea.String(pv.AlibabaCloudProps.AccessKeyID),
		AccessKeySecret: tea.String(pv.AlibabaCloudProps.AccessKeySecret),
		SecurityToken:   tea.String(pv.AlibabaCloudProps.SecurityToken),
		RegionId:        tea.String(pv.AlibabaCloudProps.Region),
	}
	return ecs.NewClient(cfg)
}

func (aa AlibabaCloudAssert) HasPodVM(t *testing.T, podvmName string) {
	client, err := aa.ecsClient()
	if err != nil {
		t.Errorf("failed to create ECS client: %v", err)
		return
	}

	result, err := client.DescribeInstances(&ecs.DescribeInstancesRequest{
		RegionId:  tea.String(pv.AlibabaCloudProps.Region),
		VSwitchId: tea.String(aa.Vpc.VSwitchID),
	})
	if err != nil {
		t.Errorf("Podvm name=%s not found: %v", podvmName, err)
		return
	}

	found := false
	if result.Body != nil && result.Body.Instances != nil {
		for _, instance := range result.Body.Instances.Instance {
			status := tea.StringValue(instance.Status)
			if status == "Terminated" {
				continue
			}
			if tea.StringValue(instance.InstanceName) == podvmName {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("Podvm name=%s not found", podvmName)
	}
}

func (aa AlibabaCloudAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	return "", nil
}
