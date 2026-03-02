// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// DockerProvisioner implements the CloudProvisioner interface for Docker.
type DockerProvisioner struct {
	conn *client.Client
	wd   string // docker's directory path on this repository
}

// DockerInstallChart implements the InstallChart interface
type DockerInstallChart struct {
	Helm *pv.Helm
}

type DockerProperties struct {
	DockerHost       string
	APIVer           string
	ClusterName      string
	NetworkName      string
	PodvmImage       string
	CaaImage         string
	CaaImageTag      string
	ContainerRuntime string
	TunnelType       string
	VxlanPort        string
}

var DockerProps = &DockerProperties{}

func initDockerProperties(properties map[string]string) error {

	DockerProps = &DockerProperties{
		DockerHost:       properties["DOCKER_HOST"],
		APIVer:           properties["DOCKER_API_VERSION"],
		ClusterName:      properties["CLUSTER_NAME"],
		NetworkName:      properties["DOCKER_NETWORK_NAME"],
		PodvmImage:       properties["DOCKER_PODVM_IMAGE"],
		CaaImage:         properties["CAA_IMAGE"],
		CaaImageTag:      properties["CAA_IMAGE_TAG"],
		ContainerRuntime: properties["CONTAINER_RUNTIME"],
		TunnelType:       properties["TUNNEL_TYPE"],
		VxlanPort:        properties["VXLAN_PORT"],
	}
	return nil
}

func NewDockerProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	wd, err := filepath.Abs(filepath.Join("..", "..", "docker"))
	if err != nil {
		log.Errorf("Error getting the absolute path of the docker directory: %v", err)
		return nil, err
	}

	if err := initDockerProperties(properties); err != nil {
		return nil, err
	}

	// set environment variables
	os.Setenv("DOCKER_HOST", DockerProps.DockerHost)
	if DockerProps.APIVer != "" {
		os.Setenv("DOCKER_API_VERSION", DockerProps.APIVer)
	}

	conn, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Errorf("Error creating the Docker client: %v", err)
		return nil, err
	}

	return &DockerProvisioner{
		conn: conn,
		wd:   wd,
	}, nil
}

func (l *DockerProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {

	// Create kind cluster
	if err := createKindCluster(l.wd); err != nil {
		log.Errorf("Error creating Kind cluster: %v", err)
		return err
	}

	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube/config")
	cfg.WithKubeconfigFile(kubeconfig)

	if err := pv.AddNodeRoleWorkerLabel(ctx, DockerProps.ClusterName, cfg); err != nil {

		return fmt.Errorf("labeling nodes: %w", err)
	}

	return nil
}

func (l *DockerProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {

	if err := deleteKindCluster(l.wd); err != nil {
		log.Errorf("Error deleting Kind cluster: %v", err)
		return err
	}
	return nil
}

func (l *DockerProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	// TODO: delete the resources created on CreateVPC() that currently only checks
	// the Docker's storage and network exist.
	return nil
}

func (l *DockerProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	// TODO: delete the resources created on CreateVPC() that currently only checks
	// the Docker's storage and network exist.
	return nil
}

func (l *DockerProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	allProps := map[string]string{
		"DOCKER_HOST":         DockerProps.DockerHost,
		"DOCKER_API_VERSION":  DockerProps.APIVer,
		"CLUSTER_NAME":        DockerProps.ClusterName,
		"DOCKER_NETWORK_NAME": DockerProps.NetworkName,
		"DOCKER_PODVM_IMAGE":  DockerProps.PodvmImage,
		"CAA_IMAGE":           DockerProps.CaaImage,
		"CAA_IMAGE_TAG":       DockerProps.CaaImageTag,
		"CONTAINER_RUNTIME":   DockerProps.ContainerRuntime,
		"TUNNEL_TYPE":         DockerProps.TunnelType,
		"VXLAN_PORT":          DockerProps.VxlanPort,
	}

	// Filter out empty values to avoid overriding defaults
	props := make(map[string]string)
	for k, v := range allProps {
		if v != "" {
			props[k] = v
		}
	}

	return props
}

func (l *DockerProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {

	// Download the podvm image from the registry by using docker pull
	cmd := exec.Command("/bin/bash", "-c", "docker pull "+DockerProps.PodvmImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Errorf("Error pulling the podvm image: %v", err)
		return err
	}

	return nil
}

func createKindCluster(workingDir string) error {
	// Create kind cluster by executing the script on the node
	cmd := exec.Command("/bin/bash", "-c", "./kind_cluster.sh create")
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	// Set CLUSTER_NAME and CONTAINER_RUNTIME if available. Also unset KUBECONFIG so that the default path is used.
	cmd.Env = append(cmd.Env, "CLUSTER_NAME="+DockerProps.ClusterName, "KUBECONFIG=", "CONTAINER_RUNTIME="+DockerProps.ContainerRuntime)
	err := cmd.Run()
	if err != nil {
		log.Errorf("Error creating Kind cluster: %v", err)
		return err
	}
	return nil
}

func deleteKindCluster(workingDir string) error {
	// Delete kind cluster by executing the script on the node
	cmd := exec.Command("/bin/bash", "-c", "./kind_cluster.sh delete")
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	// TODO: better handle stderr. Messages getting out of order.
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Errorf("Error deleting Kind cluster: %v", err)
		return err
	}
	return nil
}

func NewDockerInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &DockerInstallChart{
		Helm: helm,
	}, nil
}

func (d *DockerInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return d.Helm.Install(ctx, cfg)
}

func (d *DockerInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return d.Helm.Uninstall(ctx, cfg)
}

func (d *DockerInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Handle CAA image - already split into CAA_IMAGE and CAA_IMAGE_TAG
	if properties["CAA_IMAGE"] != "" {
		d.Helm.OverrideValues["image.name"] = properties["CAA_IMAGE"]
	}
	if properties["CAA_IMAGE_TAG"] != "" {
		d.Helm.OverrideValues["image.tag"] = properties["CAA_IMAGE_TAG"]
	}

	// Mapping the internal properties to Helm chart values.
	mapProps := map[string]string{
		"DOCKER_HOST":         "DOCKER_HOST",
		"DOCKER_API_VERSION":  "DOCKER_API_VERSION",
		"DOCKER_PODVM_IMAGE":  "DOCKER_PODVM_IMAGE",
		"DOCKER_NETWORK_NAME": "DOCKER_NETWORK_NAME",
		"TUNNEL_TYPE":         "TUNNEL_TYPE",
		"VXLAN_PORT":          "VXLAN_PORT",
		"INITDATA":            "INITDATA",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			d.Helm.OverrideProviderValues[v] = properties[k]
		}
	}

	return nil
}
