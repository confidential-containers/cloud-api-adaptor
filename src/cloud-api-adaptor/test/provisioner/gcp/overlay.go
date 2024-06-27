// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type GCPInstallOverlay struct {
	Overlay  *pv.KustomizeOverlay
	CaaImage string
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

	// Mapping the internal properties to ConfigMapGenerator properties.
	mapProps := map[string]string{
		"pause_image":      "PAUSE_IMAGE",
		"podvm_image_name": "PODVM_IMAGE_NAME",
		"machine_type":     "GCP_MACHINE_TYPE",
		"project_id":       "GCP_PROJECT_ID",
		"zone":             "GCP_ZONE",
		"network":          "GCP_NETWORK",
		"vxlan_port":       "VXLAN_PORT",
	}

	if value, ok := properties["caa_image_name"]; ok {
		if value != "" {
			log.Infof("Updating caa image with %s", value)
			if err = a.Overlay.SetKustomizeImage("cloud-api-adaptor", "newImage", value); err != nil {
				return err
			}
		}
	}

	for k, v := range mapProps {
		if properties[k] != "" {
			if err = a.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm",
				v, properties[k]); err != nil {
				return err
			}
		}
	}

	// Mapping the internal properties to SecretGenerator properties.
	mapProps = map[string]string{
		"credentials": "GCP_CREDENTIALS",
	}
	for k, _ := range mapProps {
		if properties[k] != "" {
			log.Info(properties[k])
			if err = a.Overlay.SetKustomizeSecretGeneratorFile("peer-pods-secret",
				properties[k]); err != nil {
				return err
			}
		}
	}

	if err = a.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
