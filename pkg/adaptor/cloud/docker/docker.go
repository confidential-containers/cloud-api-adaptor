package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// The default podvm docker image to use
const defaultPodVMDockerImage = "quay.io/confidential-containers/podvm-docker-image"

// Method to create and start a container
// Returns the container ID and the IP address of the container
func createContainer(ctx context.Context, client *client.Client,
	instanceName string, volumeBinding []string) (string, string, error) {
	/*
			hostBinding := nat.PortBinding{
				HostIP:   "0.0.0.0",
				HostPort: "8000",
			}
		containerPort, err := nat.NewPort("tcp", "15150")
		if err != nil {
			return "", err
		}

		portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}
	*/

	// No need to bind the port to the host
	portBinding := nat.PortMap{}

	resp, err := client.ContainerCreate(
		ctx,
		&container.Config{
			Image: defaultPodVMDockerImage,
			ExposedPorts: nat.PortSet{
				"15150/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			PortBindings: portBinding,
			Binds:        volumeBinding,
		},
		nil, nil, instanceName,
	)
	if err != nil {
		return "", "", err
	}

	// Start the container

	if err := client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", "", err
	}

	// Get the IP address of the container

	inspect, err := client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return "", "", err
	}

	return resp.ID, inspect.NetworkSettings.IPAddress, nil

}

// Method to delete container given container id
func deleteContainer(ctx context.Context, client *client.Client, containerID string) error {
	return client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})
}
