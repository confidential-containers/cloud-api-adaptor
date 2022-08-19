//go:build aws

package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type EC2CreateInstanceAPI interface {
	//Create instances
	//https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html
	RunInstances(ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)

	CreateTags(ctx context.Context,
		params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type EC2TerminateInstancesAPI interface {
	TerminateInstances(ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}

func GetInstance(c context.Context, api EC2DescribeInstancesAPI, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return api.DescribeInstances(c, input)
}

func CreateInstance(c context.Context, api EC2CreateInstanceAPI, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return api.RunInstances(c, input)
}

func DeleteInstance(c context.Context, api EC2TerminateInstancesAPI, input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return api.TerminateInstances(c, input)
}

func MakeTags(c context.Context, api EC2CreateInstanceAPI, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return api.CreateTags(c, input)
}

//TODO: Use IAM role
func NewEC2Client(cloudCfg Config) (*ec2.Client, error) {

	var cfg aws.Config
	var err error

	if cloudCfg.AccessKeyId != "" && cloudCfg.SecretKey != "" {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cloudCfg.AccessKeyId, cloudCfg.SecretKey, "")), config.WithRegion(cloudCfg.Region))
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
