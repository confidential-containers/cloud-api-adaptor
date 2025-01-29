// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// defaultTagSpecifications returns a tag specifications array with only one element and one "Name" tag.
func defaultTagSpecifications(name string, resourceType ec2types.ResourceType) []ec2types.TagSpecification {
	return []ec2types.TagSpecification{
		{
			ResourceType: resourceType,
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	}
}
