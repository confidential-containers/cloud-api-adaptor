// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"context"
	"errors"
	"testing"

	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

type imageRecordingECSClient struct {
	ecsClient
	images []string
}

func (c *imageRecordingECSClient) RunInstances(req *ecs.RunInstancesRequest) (*ecs.RunInstancesResponse, error) {
	c.images = append(c.images, tea.StringValue(req.ImageId))
	// Stop at the API boundary; this test only needs the submitted request.
	return nil, errors.New("stop after recording RunInstances")
}

func TestCreateInstanceImageOverride(t *testing.T) {
	for _, tc := range []struct {
		name           string
		systemDiskSize int
	}{
		{name: "default system disk"},
		{name: "configured system disk", systemDiskSize: 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				ImageID: "image-default", InstanceType: "ecs.g8i.xlarge",
				SystemDiskSize: tc.systemDiskSize,
			}
			client := &imageRecordingECSClient{}
			p := &alibabaCloudProvider{serviceConfig: config, ecsClient: client}

			for _, image := range []string{"image-custom", ""} {
				_, err := p.CreateInstance(context.Background(), "test-pod", "test-sandbox",
					&cloudinit.CloudConfig{}, provider.InstanceTypeSpec{Image: image})
				require.ErrorContains(t, err, "stop after recording RunInstances")
			}

			assert.Equal(t, []string{"image-custom", "image-default"}, client.images)
			assert.Equal(t, "image-default", config.ImageID)
		})
	}
}
