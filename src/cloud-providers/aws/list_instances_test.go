// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

type listInstancesMockEC2Client struct {
	mockEC2Client
	instances []types.Instance
	lastInput *ec2.DescribeInstancesInput
	callCount int
}

func (m *listInstancesMockEC2Client) DescribeInstances(ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {

	m.lastInput = params
	m.callCount++
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{Instances: m.instances},
		},
	}, nil
}

func TestListInstances(t *testing.T) {
	tests := []struct {
		name       string
		clusterUID string
		instances  []types.Instance
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "empty cluster UID returns error",
			clusterUID: "",
			wantErr:    true,
		},
		{
			name:       "no instances found",
			clusterUID: "test-uid-123",
			instances:  nil,
			wantCount:  0,
		},
		{
			name:       "returns matching instances",
			clusterUID: "test-uid-123",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-aaa"),
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("vm-1")},
						{Key: aws.String(provider.ClusterUIDTagKey), Value: aws.String("test-uid-123")},
					},
				},
				{
					InstanceId: aws.String("i-bbb"),
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("vm-2")},
						{Key: aws.String(provider.ClusterUIDTagKey), Value: aws.String("test-uid-123")},
					},
				},
			},
			wantCount: 2,
		},
		{
			name:       "instance without Name tag",
			clusterUID: "test-uid-123",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-ccc"),
					Tags: []types.Tag{
						{Key: aws.String(provider.ClusterUIDTagKey), Value: aws.String("test-uid-123")},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:       "skips instance with nil InstanceId",
			clusterUID: "test-uid-123",
			instances: []types.Instance{
				{InstanceId: nil},
				{
					InstanceId: aws.String("i-ddd"),
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("vm-valid")},
					},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &awsProvider{
				ec2Client:     &listInstancesMockEC2Client{instances: tt.instances},
				serviceConfig: &Config{},
			}

			got, err := p.ListInstances(context.Background(), provider.ListInstancesInput{ClusterUID: tt.clusterUID})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListInstances() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ListInstances() returned %d instances, want %d", len(got), tt.wantCount)
			}

			for _, inst := range got {
				if inst.ID == "" {
					t.Error("ListInstances() returned instance with empty ID")
				}
			}
		})
	}
}

func TestListInstances_VerifyFilters(t *testing.T) {
	mockClient := &listInstancesMockEC2Client{}
	p := &awsProvider{
		ec2Client:     mockClient,
		serviceConfig: &Config{},
	}

	_, _ = p.ListInstances(context.Background(), provider.ListInstancesInput{ClusterUID: "uid-filter-test"})

	if mockClient.lastInput == nil {
		t.Fatal("DescribeInstances was not called")
	}

	filters := mockClient.lastInput.Filters
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}

	expectedTagFilter := "tag:" + provider.ClusterUIDTagKey
	if aws.ToString(filters[0].Name) != expectedTagFilter {
		t.Errorf("expected filter name %q, got %q", expectedTagFilter, aws.ToString(filters[0].Name))
	}
	if len(filters[0].Values) != 1 || filters[0].Values[0] != "uid-filter-test" {
		t.Errorf("expected filter value [uid-filter-test], got %v", filters[0].Values)
	}

	if aws.ToString(filters[1].Name) != "instance-state-name" {
		t.Errorf("expected filter name %q, got %q", "instance-state-name", aws.ToString(filters[1].Name))
	}
	expectedStates := map[string]bool{"pending": true, "running": true, "stopping": true, "stopped": true}
	for _, v := range filters[1].Values {
		if !expectedStates[v] {
			t.Errorf("unexpected instance state filter value: %s", v)
		}
	}
}

func TestListInstances_VerifyIDAndName(t *testing.T) {
	launchTime := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	mockClient := &listInstancesMockEC2Client{
		instances: []types.Instance{
			{
				InstanceId: aws.String("i-test123"),
				LaunchTime: aws.Time(launchTime),
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("my-pod-vm")},
					{Key: aws.String(provider.ClusterUIDTagKey), Value: aws.String("uid-abc")},
				},
			},
		},
	}

	p := &awsProvider{
		ec2Client:     mockClient,
		serviceConfig: &Config{},
	}

	instances, err := p.ListInstances(context.Background(), provider.ListInstancesInput{ClusterUID: "uid-abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	if instances[0].ID != "i-test123" {
		t.Errorf("expected ID i-test123, got %s", instances[0].ID)
	}
	if instances[0].Name != "my-pod-vm" {
		t.Errorf("expected Name my-pod-vm, got %s", instances[0].Name)
	}
	if !instances[0].CreatedAt.Equal(launchTime) {
		t.Errorf("expected CreatedAt %v, got %v", launchTime, instances[0].CreatedAt)
	}
}
