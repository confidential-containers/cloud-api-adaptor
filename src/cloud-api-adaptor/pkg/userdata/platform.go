// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package userdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PlatformEnvPath holds the TEE and cloud provider the pod VM runs on, for
// systemd units to load with EnvironmentFile=.
const PlatformEnvPath = "/run/peerpod/platform.env"

// Guest devices created by the kernel's TEE drivers once they are loaded.
var teeDevices = []struct{ tee, path string }{
	{"tdx", "dev/tdx_guest"},
	{"sev-snp", "dev/sev-guest"},
}

// detectTEE returns the TEE the VM runs in, or "" if no TEE guest device exists.
// Azure CVMs reach the TEE through a paravisor and vTPM instead, so they report "".
func detectTEE(root string) (string, error) {
	for _, d := range teeDevices {
		_, err := os.Stat(filepath.Join(root, d.path))
		if err == nil {
			return d.tee, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", d.path, err)
		}
	}
	return "", nil
}

// WritePlatformEnv detects the TEE and cloud provider and writes them to path.
func WritePlatformEnv(path string) error {
	return writePlatformEnv("/", path)
}

func writePlatformEnv(root, path string) error {
	tee, err := detectTEE(root)
	if err != nil {
		return err
	}
	csp, err := detectCSP(root)
	if err != nil {
		return err
	}
	logger.Printf("Detected TEE=%q CSP=%q\n", tee, csp)
	return writeFile(path, []byte(fmt.Sprintf("TEE=%s\nCSP=%s\n", tee, csp)))
}
