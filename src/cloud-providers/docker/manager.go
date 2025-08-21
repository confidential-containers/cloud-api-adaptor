// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"flag"
	"os"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var dockerCfg Config

type Manager struct{}

const defaultDataDir = "/var/lib/docker/peerpods"

func init() {
	provider.AddCloudProvider("docker", &Manager{})
}

func (_ *Manager) ParseCmd(flags *flag.FlagSet) {
	flags.StringVar(&dockerCfg.DockerHost, "docker-host", "unix:///var/run/docker.sock", "Docker host, defaults to `unix:///var/run/docker.sock`")
	flags.StringVar(&dockerCfg.DockerAPIVersion, "docker-api-version", "1.40", "Docker API version")
	flags.StringVar(&dockerCfg.DockerCertPath, "docker-cert-path", "", "Path to directory with Docker TLS certificates")
	flags.BoolVar(&dockerCfg.DockerTLSVerify, "docker-tls-verify", false, "Use TLS and verify the remote server certificate")
	flags.StringVar(&dockerCfg.DataDir, "data-dir", defaultDataDir, "docker storage dir")
	flags.StringVar(&dockerCfg.PodVMDockerImage, "podvm-docker-image", defaultPodVMDockerImage, "Docker image to use for podvm")
	// Docker network name to connect to
	flags.StringVar(&dockerCfg.NetworkName, "docker-network-name", defaultDockerNetworkName, "Docker network name to connect to")
}

func (m *Manager) LoadEnv() {
	provider.DefaultToEnv(&dockerCfg.DockerHost, "DOCKER_HOST", "unix:///var/run/docker.sock")
	provider.DefaultToEnv(&dockerCfg.DockerAPIVersion, "DOCKER_API_VERSION", "1.44")
	provider.DefaultToEnv(&dockerCfg.DockerCertPath, "DOCKER_CERT_PATH", "")
	dockerTLSVerify := os.Getenv("DOCKER_TLS_VERIFY")
	if dockerTLSVerify == "1" || dockerTLSVerify == "true" {
		dockerCfg.DockerTLSVerify = true
	} else {
		dockerCfg.DockerTLSVerify = false
	}
}

func (_ *Manager) NewProvider() (provider.Provider, error) {
	return NewProvider(&dockerCfg)
}

func (_ *Manager) GetConfig() (config *Config) {
	return &dockerCfg
}
