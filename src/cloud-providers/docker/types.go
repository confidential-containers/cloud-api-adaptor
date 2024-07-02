// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

type Config struct {
	DockerHost       string
	DockerAPIVersion string
	DockerCertPath   string
	DockerTLSVerify  bool
	DataDir          string
	PodVMDockerImage string
	NetworkName      string
}
