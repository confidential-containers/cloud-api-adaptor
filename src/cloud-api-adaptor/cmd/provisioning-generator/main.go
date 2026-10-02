// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const sourceDropInFile = "process-user-data.service.d/20-provisioning-source.conf"

const configDriveUnit = `[Unit]
Description=PodVM provisioning config drive
DefaultDependencies=no

[Mount]
What=/dev/disk/by-label/cidata
Where=/media/cidata
Type=iso9660
Options=ro
TimeoutSec=30s
`

const cidataDropIn = `[Unit]
JobTimeoutSec=30s
`

const configDriveDependencies = `Requires=media-cidata.mount
After=media-cidata.mount
`

const imdsDependencies = `Wants=network-online.target
After=network-online.target
`

const envUnitTmpl = `[Unit]
%s

[Service]
Environment=PODVM_PROVISIONING_SOURCE=%s
`

const blockProvisioningDropIn = `[Service]
ExecStartPre=/usr/bin/echo PodVM provisioning source detection failed; see provisioning-generator diagnostics
ExecStartPre=/usr/bin/false
`

func main() {
	logger := log.New(os.Stderr, "[provisioning-generator] ", 0)
	if len(os.Args) != 4 {
		logger.Print("expected normal, early and late generator output directories")
		os.Exit(1)
	}
	if err := generate(os.Args[1], "/", runtime.GOARCH); err != nil {
		logger.Print(err)
		os.Exit(1)
	}
}

func generate(outputDir, root, arch string) error {
	if arch != "amd64" {
		return nil
	}
	for _, marker := range []string{
		"etc/initrd-release",
		"etc/systemd/system/multi-user.target.wants/process-user-data.path",
	} {
		_, err := os.Lstat(filepath.Join(root, marker))
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return blockProvisioning(outputDir, fmt.Errorf("inspect %s: %w", marker, err))
		}
	}

	csp, err := detectCSP(root)
	if err != nil {
		return blockProvisioning(outputDir, err)
	}

	var source string
	if csp == "" {
		source = "config-drive"
	} else {
		source = fmt.Sprintf("imds-%s", csp)
	}

	var dependencies string
	if source == "config-drive" {
		if err := writeUnit(outputDir, "media-cidata.mount", configDriveUnit); err != nil {
			return blockProvisioning(outputDir, err)
		}
		if err := writeUnit(outputDir, `dev-disk-by\x2dlabel-cidata.device.d/20-provisioning-source.conf`, cidataDropIn); err != nil {
			return blockProvisioning(outputDir, err)
		}
		dependencies = "Requires=media-cidata.mount\nAfter=media-cidata.mount\n"
	} else {
		dependencies = "Wants=network-online.target\nAfter=network-online.target\n"
	}

	sourceDropIn := fmt.Sprintf(envUnitTmpl, dependencies, source)
	return writeUnit(outputDir, sourceDropInFile, sourceDropIn)
}

func detectCSP(root string) (string, error) {
	vendor, err := readDMI(root, "sys_vendor")
	if err != nil {
		return "", err
	}
	product, err := readDMI(root, "product_name")
	if err != nil {
		return "", err
	}
	switch {
	case vendor == "Amazon EC2":
		return "aws", nil
	case strings.HasPrefix(product, "Google Compute Engine"):
		return "gcp", nil
	case vendor == "Alibaba Cloud":
		return "alibaba", nil
	case vendor == "Microsoft Corporation" && (product == "Virtual Machine" || strings.HasPrefix(product, "Hyper-V")):
		return "azure", nil
	default:
		return "", nil
	}
}

func readDMI(root, field string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "sys/class/dmi/id", field))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read DMI %s: %w", field, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func blockProvisioning(outputDir string, cause error) error {
	// A failed generator alone does not prevent systemd from starting the service.
	err := writeUnit(outputDir, sourceDropInFile, blockProvisioningDropIn)
	return errors.Join(cause, err)
}

func writeUnit(outputDir, name, contents string) error {
	path := filepath.Join(outputDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", name, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
