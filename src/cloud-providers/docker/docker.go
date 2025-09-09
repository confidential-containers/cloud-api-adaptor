// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"

	// Ensure you explicitly get the specific docker module version
	// to avoid incompatibility with the opentelemetry packages that
	// is included implicitly.
	// Refer to docker module specific vendor.mod for the versions
	// eg. - https://github.com/moby/moby/blob/v25.0.5/vendor.mod

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// The default podvm docker image to use
const defaultPodVMDockerImage = "quay.io/confidential-containers/podvm-docker-image"

// Default docker network name to connect to
const defaultDockerNetworkName = "bridge"

// dockerClient defines the interface for Docker operations
type dockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	Close() error
}

// Method to create and start a container
// Returns the container ID and the IP address of the container
func createContainer(ctx context.Context, client dockerClient,
	instanceName string, volumeBinding []string,
	podvmImage string, networkName string) (string, string, error) {

	// No need to bind the port to the host
	portBinding := nat.PortMap{}

	// Create a privileged container as it's required due to systemd
	resp, err := client.ContainerCreate(
		ctx,
		&container.Config{
			Image: podvmImage,
			ExposedPorts: nat.PortSet{
				"15150/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			PortBindings: portBinding,
			Binds:        volumeBinding,
			Privileged:   true, // This line is added to create a privileged container
		},
		// Connect to specific network name
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},

		nil, instanceName,
	)
	if err != nil {
		return "", "", err
	}

	// Start the container

	if err := client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", "", err
	}

	// Get the IP address of the container

	inspect, err := client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return "", "", err
	}

	// Get the IP address of the container from the network settings
	// networks: map[network-name: {IPAddress: ip-address}]
	// The network name is the key in the networks map

	return resp.ID, inspect.NetworkSettings.Networks[networkName].IPAddress, nil

}

// Method to delete container given container id
func deleteContainer(ctx context.Context, client dockerClient, containerID string) error {
	return client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}
