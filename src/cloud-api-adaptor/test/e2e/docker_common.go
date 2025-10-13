//go:build docker

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerAssert implements the CloudAssert interface for Docker.
type DockerAssert struct {
	// TODO: create the connection once on the initializer.
	//conn client.Connect
}

func (c DockerAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (l DockerAssert) HasPodVM(t *testing.T, podvmName string) {
	conn, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}

	// Check if the container is running
	containers, err := conn.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for _, container := range containers {
		if strings.Contains(container.Names[0], podvmName) {
			return
		}
	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}

func (l DockerAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}
