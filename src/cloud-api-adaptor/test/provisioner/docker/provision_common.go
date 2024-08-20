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
	"github.com/containerd/containerd/reference"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// DockerProvisioner implements the CloudProvisioner interface for Docker.
type DockerProvisioner struct {
	conn *client.Client
	wd   string // docker's directory path on this repository
}

// DockerInstallOverlay implements the InstallOverlay interface
type DockerInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

type DockerProperties struct {
	DockerHost       string
	ApiVer           string
	ClusterName      string
	NetworkName      string
	PodvmImage       string
	CaaImage         string
	CaaImageTag      string
	KbsImage         string
	KbsImageTag      string
	ContainerRuntime string
}

var DockerProps = &DockerProperties{}

func initDockerProperties(properties map[string]string) error {

	DockerProps = &DockerProperties{
		DockerHost:       properties["DOCKER_HOST"],
		ApiVer:           properties["DOCKER_API_VERSION"],
		ClusterName:      properties["CLUSTER_NAME"],
		NetworkName:      properties["DOCKER_NETWORK_NAME"],
		PodvmImage:       properties["DOCKER_PODVM_IMAGE"],
		CaaImage:         properties["CAA_IMAGE"],
		CaaImageTag:      properties["CAA_IMAGE_TAG"],
		KbsImage:         properties["KBS_IMAGE"],
		KbsImageTag:      properties["KBS_IMAGE_TAG"],
		ContainerRuntime: properties["CONTAINER_RUNTIME"],
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

	conn, err := client.NewClientWithOpts(client.FromEnv)
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
	return map[string]string{
		"DOCKER_HOST":         DockerProps.DockerHost,
		"DOCKER_API_VERSION":  DockerProps.ApiVer,
		"CLUSTER_NAME":        DockerProps.ClusterName,
		"DOCKER_NETWORK_NAME": DockerProps.NetworkName,
		"DOCKER_PODVM_IMAGE":  DockerProps.PodvmImage,
		"CAA_IMAGE":           DockerProps.CaaImage,
		"CAA_IMAGE_TAG":       DockerProps.CaaImageTag,
		"KBS_IMAGE":           DockerProps.KbsImage,
		"KBS_IMAGE_TAG":       DockerProps.KbsImageTag,
		"CONTAINER_RUNTIME":   DockerProps.ContainerRuntime,
	}
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

func NewDockerInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		log.Errorf("Error creating the docker provider install overlay: %v", err)
		return nil, err
	}

	return &DockerInstallOverlay{
		Overlay: overlay,
	}, nil
}

func isDockerKustomizeConfigMapKey(key string) bool {
	switch key {
	case "CLOUD_PROVIDER", "DOCKER_HOST", "DOCKER_API_VERSION", "DOCKER_PODVM_IMAGE", "DOCKER_NETWORK_NAME", "GLOBAL_INITDATA":
		return true
	default:
		return false
	}
}

func (lio *DockerInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Apply(ctx, cfg)
}

func (lio *DockerInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Delete(ctx, cfg)
}

// Update install/overlays/docker/kustomization.yaml
func (lio *DockerInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {

	// If a custom image is defined then update it in the kustomization file.
	if DockerProps.CaaImage != "" {
		spec, err := reference.Parse(DockerProps.CaaImage)
		if err != nil {
			return fmt.Errorf("parsing image: %w", err)
		}

		log.Infof("Updating CAA image with %q", spec.Locator)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newName", spec.Locator); err != nil {
			return err
		}
	}

	if DockerProps.CaaImageTag != "" {
		spec, err := reference.Parse(DockerProps.CaaImageTag)
		if err != nil {
			return fmt.Errorf("parsing image tag: %w", err)
		}
		log.Infof("Updating CAA image tag with %q", spec.Locator)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", spec.Locator); err != nil {
			return err
		}
	}

	for k, v := range properties {
		// configMapGenerator
		if isDockerKustomizeConfigMapKey(k) {
			if err := lio.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", k, v); err != nil {
				return err
			}
		}
	}

	if err := lio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil

}
