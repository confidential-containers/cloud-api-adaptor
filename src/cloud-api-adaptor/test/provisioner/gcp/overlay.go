// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"strings"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type GCPInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

func NewGCPInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		return nil, err
	}

	return &GCPInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (a *GCPInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return a.Overlay.Apply(ctx, cfg)
}

func (a *GCPInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return a.Overlay.Delete(ctx, cfg)
}

func (a *GCPInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error

	image := strings.Split(properties["caa_image"], ":")[0]
	tag := strings.Split(properties["caa_image"], ":")[1]
	log.Infof("Updating caa image with %s", image)
	if image != "" {
		err = a.Overlay.SetKustomizeImage("cloud-api-adaptor", "newName", image)
		if err != nil {
			return err
		}
	}
	if tag != "" {
		err = a.Overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", tag)
		if err != nil {
			return err
		}
	}

	// Mapping the internal properties to ConfigMapGenerator properties.
	mapProps := map[string]string{
		"podvm_image_name":   "PODVM_IMAGE_NAME",
		"podvm_machine_type": "GCP_MACHINE_TYPE", // Different from cluster_machine_type
		"project_id":         "GCP_PROJECT_ID",
		"zone":               "GCP_ZONE",
		"vpc_name":           "GCP_NETWORK",
		// TODO: Check those
		// "vxlan_port":       "VXLAN_PORT",
		// "pause_image":      "PAUSE_IMAGE",
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			if err = a.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v, properties[k]); err != nil {
				return err
			}
		}
	}

	credsPath := properties["credentials"]
	if credsPath != "" {
		err := copyFile(credsPath, filepath.Join(a.Overlay.ConfigDir, "GCP_CREDENTIALS"))
		if err != nil {
			log.Fatalf("failed to copy file: %v", err)
		}

		// basePath := filepath.Base(credsPath)
		if err = a.Overlay.SetKustomizeSecretGeneratorFile("peer-pods-secret",
			"GCP_CREDENTIALS"); err != nil {
			return err
		}
	}

	if err = a.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
