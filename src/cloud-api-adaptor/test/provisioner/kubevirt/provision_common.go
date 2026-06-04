// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var KubeVirtProvs = &KubeVirtProvisioner{}

type Virtclient struct {
	client kubecli.KubevirtClient
}

type KubeVirtProvisioner struct {
	kubevirtClient *Virtclient
	Properties     map[string]string
	serviceConfig  string
	CaaImage       string
}

func expandUser(filePath string) (expandedPath string, err error) {
	if strings.HasPrefix(filePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, filePath[2:]), nil
	}
	return filePath, nil
}

// NewKubeVirtProvisioner creates a new instance of KubeVirtProvisioner with the provided properties.
func newKubeVirtClient(kubeconfigPath string) (*Virtclient, error) {

	if kubeconfigPath == "" {
		return nil, fmt.Errorf("path_to_kubeconfig is not set")
	}

	virtClient, err := kubecli.GetKubevirtClientFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create KubeVirt client: %w", err)
	}

	return &Virtclient{client: virtClient}, nil
}

func NewKubeVirtProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {

	kubeconfigPath, err := expandUser(properties["path_to_kubeconfig"])
	if err != nil {
		log.Infof("path_to_kubeconfig Path was not found")
		return nil, err
	}
	properties["path_to_kubeconfig"] = kubeconfigPath

	proPath, err := expandUser(properties["path_to_vmconfig"])
	if err != nil {
		log.Infof("path_to_vmconfig Path was not found")
		return nil, err
	}
	properties["path_to_vmconfig"] = proPath

	serPath, err := expandUser(properties["path_to_serviceconfig"])
	if err != nil {
		log.Infof("path_to_serviceconfig Path was not found")
		return nil, nil
	}
	properties["path_to_serviceconfig"] = serPath

	client, err := newKubeVirtClient(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	KubeVirtProvs = &KubeVirtProvisioner{
		kubevirtClient: client,
		Properties:     properties,
		serviceConfig:  properties["path_to_serviceconfig"],
		CaaImage:       properties["CAA_IMAGE"],
	}
	return KubeVirtProvs, nil
}

func (p *KubeVirtProvisioner) KubevirtClient() kubecli.KubevirtClient {
	if p == nil || p.kubevirtClient == nil {
		return nil
	}
	return p.kubevirtClient.client
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

func (p *KubeVirtProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *KubeVirtProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *KubeVirtProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *KubeVirtProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *KubeVirtProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	props := make(map[string]string)
	props["CAA_IMAGE"] = p.CaaImage

	for k, v := range p.Properties {
		props[k] = v
	}
	return props
}

func (p *KubeVirtProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

type KubeVirtInstallChart struct {
	Helm       *pv.Helm
	installDir string
}

func NewKubeVirtInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &KubeVirtInstallChart{
		Helm:       helm,
		installDir: installDir,
	}, nil
}

func (o *KubeVirtInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return o.Helm.Install(ctx, cfg)
}

func (o *KubeVirtInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return o.Helm.Uninstall(ctx, cfg)
}

func (o *KubeVirtInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	if KubeVirtProvs.CaaImage != "" {
		// Split the CAA image into name and tag using the last colon as the separator
		lastColonIndex := strings.LastIndex(KubeVirtProvs.CaaImage, ":")
		if lastColonIndex == -1 {
			o.Helm.OverrideValues["image.name"] = KubeVirtProvs.CaaImage
		} else if lastColonIndex == 0 || lastColonIndex == len(KubeVirtProvs.CaaImage)-1 {
			return fmt.Errorf("Invalid CAA image format: %s", KubeVirtProvs.CaaImage)
		} else {
			o.Helm.OverrideValues["image.name"] = KubeVirtProvs.CaaImage[:lastColonIndex]
			o.Helm.OverrideValues["image.tag"] = KubeVirtProvs.CaaImage[lastColonIndex+1:]
		}
	}

	// Override provider values
	for k, v := range properties {
		if v != "" {
			o.Helm.OverrideProviderValues[k] = v
		}
	}

	dstDir := filepath.Join(o.installDir, "charts", "peerpods", "providers", "kubevirt")
	if _, err := os.Stat(dstDir); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(dstDir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dstDir, err)
			}
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dstDir, err)
		}
	}

	// Copy the kubeconfig and vmconfig to the destination directory
	kubeconfigSrc := properties["path_to_kubeconfig"]
	if kubeconfigSrc == "" {
		return fmt.Errorf("path_to_kubeconfig is not set in properties")
	}
	dstPath := filepath.Join(dstDir, filepath.Base(kubeconfigSrc))
	if err := copyFile(kubeconfigSrc, dstPath); err != nil {
		return fmt.Errorf("failed to copy path_to_kubeconfig from %s to %s: %w", kubeconfigSrc, dstPath, err)
	}

	vmconfigSrc := properties["path_to_vmconfig"]
	if vmconfigSrc == "" {
		return fmt.Errorf("path_to_vmconfig is not set in properties")
	}
	dstPath = filepath.Join(dstDir, filepath.Base(vmconfigSrc))
	if err := copyFile(vmconfigSrc, dstPath); err != nil {
		return fmt.Errorf("failed to copy path_to_vmconfig from %s to %s: %w", vmconfigSrc, dstPath, err)
	}

	if serviceconfigSrc := properties["path_to_serviceconfig"]; serviceconfigSrc != "" {
		dstPath := filepath.Join(dstDir, filepath.Base(serviceconfigSrc))
		if err := copyFile(serviceconfigSrc, dstPath); err != nil {
			return fmt.Errorf("failed to copy path_to_serviceconfig from %s to %s: %w", serviceconfigSrc, dstPath, err)
		}
	}

	return nil
}
