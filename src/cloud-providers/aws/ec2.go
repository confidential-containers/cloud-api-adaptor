// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func NewEC2Client(cloudCfg Config) (*ec2.Client, error) {

	var cfg aws.Config
	var err error

	if cloudCfg.AccessKeyID != "" && cloudCfg.SecretKey != "" {
		logger.Printf("using static credentials")
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cloudCfg.AccessKeyID, cloudCfg.SecretKey, cloudCfg.SessionToken)),
			config.WithRegion(cloudCfg.Region))
	} else if cloudCfg.LoginProfile != "" {
		logger.Printf("using shared profile: %s", cloudCfg.LoginProfile)
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cloudCfg.Region),
			config.WithSharedConfigProfile(cloudCfg.LoginProfile))
	} else {
		logger.Printf("using default credential chain (supports IRSA)")
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cloudCfg.Region))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	return client, nil
}
