//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go"
)

func init() {
	registerCloudDiskVerifier("aws", newAWSDiskVerifier)
}

type awsDiskVerifier struct {
	client *ec2.Client
}

func newAWSDiskVerifier() (CloudDiskVerifier, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return nil, fmt.Errorf("AWS_REGION must be set for cloud-side verification")
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return &awsDiskVerifier{client: ec2.NewFromConfig(cfg)}, nil
}

func (v *awsDiskVerifier) DiskExists(ctx context.Context, diskID string) (bool, error) {
	result, err := v.client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{diskID},
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidVolume.NotFound" {
			return false, nil
		}
		return false, err
	}
	return len(result.Volumes) > 0, nil
}

func (v *awsDiskVerifier) DiskState(ctx context.Context, diskID string) (string, error) {
	result, err := v.client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{diskID},
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidVolume.NotFound" {
			return "deleted", nil
		}
		return "", err
	}
	if len(result.Volumes) == 0 {
		return "deleted", nil
	}
	return string(result.Volumes[0].State), nil
}
