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
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Helm represents a generic Helm chart installer
type Helm struct {
	ChartPath   string
	Namespace   string
	ReleaseName string
	Debug       bool
	Values      map[string]interface{} // Complete helm values structure
}

// NewHelm creates a new Helm instance and builds chart dependencies
func NewHelm(chartPath, namespace, releaseName string, debug bool) (*Helm, error) {
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
		ChartPath:   chartPath,
		Namespace:   namespace,
		ReleaseName: releaseName,
		Debug:       debug,
		Values:      make(map[string]interface{}),
	}, nil
}

// LoadFromFile reads a YAML file and deep merges its content into the Values map
func (h *Helm) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read values file %s: %w", path, err)
	}

	var fileValues map[string]interface{}
	if err := yaml.Unmarshal(data, &fileValues); err != nil {
		return fmt.Errorf("failed to parse YAML from %s: %w", path, err)
	}

	// Deep merge file values into existing Values map
	h.Values = deepMerge(h.Values, fileValues)

	log.Infof("Loaded helm values from: %s", path)
	return nil
}

// deepMerge recursively merges src into dst. Values in src take precedence.
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = make(map[string]interface{})
	}

	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal
			continue
		}

		// If both are maps, merge recursively
		srcMap, srcIsMap := srcVal.(map[string]interface{})
		dstMap, dstIsMap := dstVal.(map[string]interface{})
		if srcIsMap && dstIsMap {
			dst[key] = deepMerge(dstMap, srcMap)
		} else {
			// Otherwise, src wins
			dst[key] = srcVal
		}
	}

	return dst
}

// ErrDryRun is returned when dry-run mode completes successfully
var ErrDryRun = fmt.Errorf("dry-run completed")

// Install installs the Helm chart
func (h *Helm) Install(ctx context.Context, cfg *envconf.Config) error {
	// Create temporary values file
	tmpFile, err := os.CreateTemp("", "helm-values-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp values file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write Values to temp file
	data, err := yaml.Marshal(h.Values)
	if err != nil {
		return fmt.Errorf("failed to marshal values to YAML: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write values to temp file: %w", err)
	}
	tmpFile.Close()

	// Check for dry-run mode
	dryRun := os.Getenv("HELM_DRY_RUN") == "true"

	var args []string
	if dryRun {
		// Use helm template for dry-run
		args = []string{
			"template", h.ReleaseName, h.ChartPath,
			"--namespace", h.Namespace,
			"-f", tmpPath,
		}
	} else {
		// Build helm install command
		args = []string{
			"install", h.ReleaseName, h.ChartPath,
			"--namespace", h.Namespace,
			"--create-namespace",
			"--wait", "--timeout", "15m",
			"--kubeconfig", cfg.KubeconfigFile(),
			"-f", tmpPath,
		}
	}

	if h.Debug {
		args = append(args, "--debug")
	}

	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	log.Infof("Executing helm command: helm %s", strings.Join(args, " "))
	output, err := cmd.CombinedOutput()
	log.Info("Helm output:")
	fmt.Printf("%s", output)
	if err != nil {
		return fmt.Errorf("failed to run helm: %w, output: %s", err, output)
	}

	if dryRun {
		return ErrDryRun
	}
	return nil
}

// Uninstall uninstalls the Helm chart
func (h *Helm) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	args := []string{
		"uninstall", h.ReleaseName,
		"--namespace", h.Namespace,
		"--kubeconfig", cfg.KubeconfigFile(),
	}

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
