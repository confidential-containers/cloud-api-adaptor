// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package userdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// PlatformEnvPath holds facts about the platform the pod VM runs on, for
// systemd units to load with EnvironmentFile=.
const PlatformEnvPath = "/run/peerpod/platform.env"

const (
	tsmReportDir = "sys/kernel/config/tsm/report"
	// AA's fallback for TDX guests without configfs-tsm.
	tdxGuestDevice = "dev/tdx_guest"
)

// tsmProvider returns the TEE driver registered with configfs-tsm, or "" if
// there is none.
func tsmProvider(root string) (string, error) {
	dir, err := os.MkdirTemp(filepath.Join(root, tsmReportDir), "probe")
	// ENOENT: no configfs-tsm. ENXIO: no TEE driver registered.
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENXIO) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("create tsm report: %w", err)
	}
	defer os.Remove(dir)
	data, err := os.ReadFile(filepath.Join(dir, "provider"))
	if err != nil {
		return "", fmt.Errorf("read tsm provider: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// detectTDX reports whether the guest sees TDX, using the same checks as AA's
// tdx attester. Azure CVMs reach TDX through a paravisor, so they report false.
func detectTDX(root string) (bool, error) {
	provider, err := tsmProvider(root)
	if err != nil {
		return false, err
	}
	if provider == "tdx_guest" {
		return true, nil
	}
	_, err = os.Stat(filepath.Join(root, tdxGuestDevice))
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", tdxGuestDevice, err)
	}
	return false, nil
}

// supportsInitdataBinding reports whether AA is known to bind initdata here:
// TDX records it in RTMR3 when the host left MRCONFIGID unset, and Azure CVMs
// extend a vTPM PCR. Other attesters either can't bind it or, like SNP
// without HOSTDATA, exit when asked to.
func supportsInitdataBinding(tdx bool, csp string) bool {
	return tdx || csp == "azure"
}

// WritePlatformEnv detects the platform and writes the result to path.
func WritePlatformEnv(path string) error {
	return writePlatformEnv("/", path)
}

func writePlatformEnv(root, path string) error {
	tdx, err := detectTDX(root)
	if err != nil {
		return err
	}
	csp, err := detectCSP(root)
	if err != nil {
		return err
	}
	binding := "no"
	if supportsInitdataBinding(tdx, csp) {
		binding = "yes"
	}
	logger.Printf("Detected TDX=%t CSP=%q, INITDATA_BINDING=%s\n", tdx, csp, binding)
	env := "INITDATA_BINDING=" + binding + "\n"
	if csp != "" {
		env = "CSP=" + csp + "\n" + env
	}
	return writeFile(path, []byte(env))
}
