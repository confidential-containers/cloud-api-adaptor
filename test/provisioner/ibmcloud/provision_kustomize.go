// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// IBMCloudInstallOverlay implements the InstallOverlay interface
type IBMCloudInstallOverlay struct {
	Overlay *pv.KustomizeOverlay
}

type QuayTagsResponse struct {
	Tags []struct {
		Name     string `json:"name"`
		Manifest bool   `json:"is_manifest_list"`
	} `json:"tags"`
	Others map[string]interface{} `json:"-"`
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
	case "IBMCLOUD_PODVM_INSTANCE_PROFILE_LIST":
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
	case "IBMCLOUD_IAM_PROFILE_ID":
		return true
	case "IBMCLOUD_IAM_ENDPOINT":
		return true
	case "IBMCLOUD_ZONE":
		return true
	default:
		return false
	}
}

func isWorkerS390xFlavors() bool {
	if strings.HasPrefix(IBMCloudProps.WorkerFlavor, "bz") ||
		strings.HasPrefix(IBMCloudProps.WorkerFlavor, "cz") ||
		strings.HasPrefix(IBMCloudProps.WorkerFlavor, "mz") {
		return true
	}

	return false
}

func getCaaLatestCommitTag() string {
	resp, err := http.Get("https://quay.io/api/v1/repository/confidential-containers/cloud-api-adaptor/tag/")
	if err != nil {
		log.Errorf(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf(err.Error())
	}

	var result QuayTagsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Errorf(err.Error())
		return ""
	}

	for _, tag := range result.Tags {
		if tag.Manifest && len(tag.Name) == 40 { // the latest git commit hash tag
			return tag.Name
		}
	}

	return ""
}

func NewIBMCloudInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		return nil, err
	}

	return &IBMCloudInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (lio *IBMCloudInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Apply(ctx, cfg)
}

func (lio *IBMCloudInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Delete(ctx, cfg)
}

// Update install/overlays/ibmcloud/kustomization.yaml
func (lio *IBMCloudInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	log.Debugf("%+v", properties)
	var err error

	// image
	var newTag string
	if IBMCloudProps.CaaImageTag != "" {
		newTag = IBMCloudProps.CaaImageTag
	} else if isWorkerS390xFlavors() {
		newTag = getCaaLatestCommitTag()
	}
	if newTag != "" {
		log.Infof("Updating caa image tag with %s", newTag)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", newTag); err != nil {
			return err
		}
	}

	for k, v := range properties {
		// configMapGenerator
		if isKustomizeConfigMapKey(k) {
			if err = lio.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", k, v); err != nil {
				return err
			}
		}
		// secretGenerator
		if isKustomizeSecretKey(k) {
			if err = lio.Overlay.SetKustomizeSecretGeneratorLiteral("peer-pods-secret", k, v); err != nil {
				return err
			}
		}
	}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" {
		client, err := cfg.NewClient()
		if err != nil {
			return err
		}
		clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
		if err != nil {
			return err
		}
		_, err = clientSet.CoreV1().Secrets("confidential-containers-system").Get(ctx, "auth-json-secret", metav1.GetOptions{})
		if err == nil {
			log.Info("Deleting pre-existing auth-json-secret...")
			err = clientSet.CoreV1().Secrets("confidential-containers-system").Delete(ctx, "auth-json-secret", metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}

		log.Info("Setting up auth.json")
		data := map[string]interface{}{
			"auths": map[string]interface{}{
				"quay.io": map[string]interface{}{
					"auth": os.Getenv("REGISTRY_CREDENTIAL_ENCODED"),
				},
			},
		}
		jsondata, err := json.MarshalIndent(data, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(lio.Overlay.ConfigDir, "auth.json"), jsondata, 0644); err != nil {
			return err
		}
		if err = lio.Overlay.SetKustomizeSecretGeneratorFile("auth-json-secret", "auth.json"); err != nil {
			return err
		}
	}
	if err = lio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
