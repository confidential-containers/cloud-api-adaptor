// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"flag"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var dockerCfg Config

type Manager struct{}

const defaultDataDir = "/var/lib/docker/peerpods"

func init() {
	provider.AddCloudProvider("docker", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	reg := provider.NewFlagRegistrar(flags)

	// Flags with environment variable support
	reg.StringWithEnv(&dockerCfg.DockerHost, "docker-host", "unix:///var/run/docker.sock", "DOCKER_HOST", "Docker host")
	reg.StringWithEnv(&dockerCfg.DockerAPIVersion, "docker-api-version", "1.44", "DOCKER_API_VERSION", "Docker API version")
	reg.StringWithEnv(&dockerCfg.DockerCertPath, "docker-cert-path", "", "DOCKER_CERT_PATH", "Path to directory with Docker TLS certificates")
	reg.BoolWithEnv(&dockerCfg.DockerTLSVerify, "docker-tls-verify", false, "DOCKER_TLS_VERIFY", "Use TLS and verify the remote server certificate")
	reg.StringWithEnv(&dockerCfg.PodVMDockerImage, "podvm-docker-image", defaultPodVMDockerImage, "DOCKER_PODVM_IMAGE", "Docker image to use for podvm")
	reg.StringWithEnv(&dockerCfg.NetworkName, "docker-network-name", defaultDockerNetworkName, "DOCKER_NETWORK_NAME", "Docker network name to connect to")

	// Flags without environment variable support (pass empty string for envVarName)
	reg.StringWithEnv(&dockerCfg.DataDir, "data-dir", defaultDataDir, "", "docker storage dir")
}

func (m *Manager) LoadEnv() {
	// No longer needed - environment variables are handled in ParseCmd
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&dockerCfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &dockerCfg
}
