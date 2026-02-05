// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Helm represents a Helm chart installer
type Helm struct {
	ChartPath               string            // path to the chart directory
	Namespace               string            // namespace where the chart will be installed
	ReleaseName             string            // name of the Helm release
	Provider                string            // cloud provider name
	Debug                   bool              // enable debug mode for helm commands
	OverrideValues          map[string]string // key-value map for overriding chart values
	OverrideProviderValues  map[string]string // key-value map for overriding provider-specific chart values
	OverrideProviderSecrets map[string]string // key-value map for overriding provider-specific chart secrets
}

// NewHelm creates a new Helm instance and builds chart dependencies
func NewHelm(chartPath, namespace, releaseName, provider string, debug bool) (*Helm, error) {
	// Build chart dependencies
	args := []string{"dependency", "build", chartPath}
	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to build helm chart dependencies: %w, output: %s", err, string(output))
	}

	return &Helm{
		ChartPath:               chartPath,
		Namespace:               namespace,
		ReleaseName:             releaseName,
		Provider:                provider,
		Debug:                   debug,
		OverrideValues:          make(map[string]string),
		OverrideProviderValues:  make(map[string]string),
		OverrideProviderSecrets: make(map[string]string),
	}, nil
}

// Install installs the Helm chart. Equivalent to the `helm install` command
func (h *Helm) Install(ctx context.Context, cfg *envconf.Config) error {
	providerValuesFile := fmt.Sprintf("%s/providers/%s.yaml", h.ChartPath, h.Provider)

	args := []string{"install", h.ReleaseName, h.ChartPath,
		"--namespace", h.Namespace,
		"--create-namespace",
		"--wait", "--timeout", "15m",
		"--kubeconfig", cfg.KubeconfigFile(),
		"-f", providerValuesFile}

	// Add --debug flag if Debug is enabled
	if h.Debug {
		args = append(args, "--debug")
	}

	// Add --set-literal flags for OverrideValues if not empty (passed as-is)
	if len(h.OverrideValues) > 0 {
		for key, value := range h.OverrideValues {
			setArg := fmt.Sprintf("%s=%s", key, value)
			args = append(args, "--set-literal", setArg)
		}
	}

	// Add --set-literal flags for OverrideProviderValues if not empty
	if len(h.OverrideProviderValues) > 0 {
		for key, value := range h.OverrideProviderValues {
			setArg := fmt.Sprintf("providerConfigs.%s.%s=%s", h.Provider, key, value)
			args = append(args, "--set-literal", setArg)
		}
	}

	// Add --set flags for OverrideProviderSecrets if not empty
	if len(h.OverrideProviderSecrets) > 0 {
		for key, value := range h.OverrideProviderSecrets {
			setArg := fmt.Sprintf("providerSecrets.%s.%s=%s", h.Provider, key, value)
			args = append(args, "--set-literal", setArg)
		}
	}

	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	log.Info("Helm install output:")
	fmt.Printf("%s", output)
	if err != nil {
		return fmt.Errorf("failed to install helm chart: %w, output: %s", err, output)
	}
	return nil
}

// Uninstall uninstalls the Helm chart. Equivalent to the `helm uninstall` command
func (h *Helm) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	args := []string{"uninstall", h.ReleaseName,
		"--namespace", h.Namespace,
		"--kubeconfig", cfg.KubeconfigFile()}

	// Add --debug flag if Debug is enabled
	if h.Debug {
		args = append(args, "--debug")
	}

	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall helm chart: %w, output: %s", err, string(output))
	}
	return nil
}
