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

// TODO: Use IAM role
func NewEC2Client(cloudCfg Config) (*ec2.Client, error) {

	var cfg aws.Config
	var err error

	if cloudCfg.AccessKeyId != "" && cloudCfg.SecretKey != "" {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cloudCfg.AccessKeyId, cloudCfg.SecretKey, cloudCfg.SessionToken)), config.WithRegion(cloudCfg.Region))
		if err != nil {
			return nil, fmt.Errorf("configuration error when using creds: %s", err)
		}

	} else {

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cloudCfg.Region),
			config.WithSharedConfigProfile(cloudCfg.LoginProfile))
		if err != nil {
			return nil, fmt.Errorf("configuration error when using shared profile: %s", err)
		}
	}
	client := ec2.NewFromConfig(cfg)
	return client, nil
}
