//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func init() {
	newInstallOverlayFunctions["ibmcloud"] = NewIBMCloudInstallOverlay
}

// IBMCloudInstallOverlay implements the InstallOverlay interface
type IBMCloudInstallOverlay struct {
	overlay *KustomizeOverlay
}

func isKustomizeConfigMapKey(key string) bool {
	switch key {
	case "CLOUD_PROVIDER":
		return true
	case "IBMCLOUD_VPC_ENDPOINT":
		return true
	case "IBMCLOUD_RESOURCE_GROUP_ID":
		return true
	case "IBMCLOUD_SSH_KEY_ID":
		return true
	case "IBMCLOUD_PODVM_IMAGE_ID":
		return true
	case "IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME":
		return true
	case "IBMCLOUD_ZONE":
		return true
	case "IBMCLOUD_VPC_SUBNET_ID":
		return true
	case "IBMCLOUD_VPC_SG_ID":
		return true
	case "IBMCLOUD_VPC_ID":
		return true
	case "CRI_RUNTIME_ENDPOINT":
		return true
	default:
		return false
	}
}

func isKustomizeSecretKey(key string) bool {
	switch key {
	case "IBMCLOUD_API_KEY":
		return true
	case "IBMCLOUD_IAM_ENDPOINT":
		return true
	case "IBMCLOUD_ZONE":
		return true
	default:
		return false
	}
}

func getCaaNewTagFromCommit() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Errorf(err.Error())
		return ""
	}
	repoDir, err := filepath.Abs(path.Join(dir, "../../"))
	if err != nil {
		log.Errorf(err.Error())
		return ""
	}
	log.Infof("getCaaNewTagFromCommit running in dir: %s", repoDir)
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		log.Errorf(err.Error())
		return ""
	}
	log.Infof("getCaaNewTagFromCommit opened repository in dir: %s", repoDir)
	ref, err := repo.Head()
	if err != nil {
		log.Errorf(err.Error())
		return ""
	}
	log.Infof("getCaaNewTagFromCommit read head successfully.")

	branch := ref.Name()
	log.Infof("Branch name: %s", branch)
	if branch == "refs/heads/staging" {
		cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			log.Errorf(err.Error())
			return ""
		}
		commit, _ := cIter.Next()
		if commit != nil {
			cStr := commit.Hash.String()
			log.Infof("Latest commit hash is: %s", cStr)
			return cStr
		}
	}
	return ""
}

func NewIBMCloudInstallOverlay() (InstallOverlay, error) {
	overlay, err := NewKustomizeOverlay("../../install/overlays/ibmcloud")
	if err != nil {
		return nil, err
	}

	return &IBMCloudInstallOverlay{
		overlay: overlay,
	}, nil
}

func (lio *IBMCloudInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Apply(ctx, cfg)
}

func (lio *IBMCloudInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Delete(ctx, cfg)
}

// Update install/overlays/ibmcloud/kustomization.yaml
func (lio *IBMCloudInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	log.Debugf("%+v", properties)
	var err error

	// image
	var newTag string
	if IBMCloudProps.CaaImageTag != "" {
		newTag = IBMCloudProps.CaaImageTag
	} else {
		newTag = getCaaNewTagFromCommit()
	}
	if newTag != "" {
		log.Infof("Updating caa image tag with %s", newTag)
		if err = lio.overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", newTag); err != nil {
			return err
		}
	}

	for k, v := range properties {
		// configMapGenerator
		if isKustomizeConfigMapKey(k) {
			if err = lio.overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", k, v); err != nil {
				return err
			}
		}
		// secretGenerator
		if isKustomizeSecretKey(k) {
			if err = lio.overlay.SetKustomizeSecretGeneratorLiteral("peer-pods-secret", k, v); err != nil {
				return err
			}
		}
	}

	if err = lio.overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
