// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package userdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupportsInitdataBinding(t *testing.T) {
	tests := []struct {
		name string
		tdx  bool
		csp  string
		want bool
	}{
		{"TDX on GCP", true, "gcp", true},
		{"TDX, unknown CSP", true, "", true},
		{"Azure CVM", false, "azure", true},
		{"no TDX on GCP", false, "gcp", false},
		{"no TDX on AWS", false, "aws", false},
		{"no TDX, unknown CSP", false, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsInitdataBinding(tt.tdx, tt.csp); got != tt.want {
				t.Fatalf("supportsInitdataBinding(%t, %q) = %t; want %t", tt.tdx, tt.csp, got, tt.want)
			}
		})
	}
}

func TestDetectTDX(t *testing.T) {
	tests := []struct {
		name   string
		device string
		tdx    bool
	}{
		{"guest device", tdxGuestDevice, true},
		{"no TEE", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.device != "" {
				writeFixture(t, root, tt.device, "")
			}
			tdx, err := detectTDX(root)
			if err != nil || tdx != tt.tdx {
				t.Fatalf("TDX = %t, error = %v; want %t", tdx, err, tt.tdx)
			}
		})
	}
}

func TestDetectTDXProviderError(t *testing.T) {
	root := t.TempDir()
	// A plain directory accepts the probe but, unlike configfs, has no provider.
	reportDir := filepath.Join(root, tsmReportDir)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if tdx, err := detectTDX(root); err == nil {
		t.Fatalf("TDX = %t, want an error", tdx)
	}
	entries, err := os.ReadDir(reportDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe not removed: %v, %v", entries, err)
	}
}

func TestDetectTDXStatError(t *testing.T) {
	root := t.TempDir()
	// A file where the dev directory should be makes stat fail with ENOTDIR.
	writeFixture(t, root, "dev", "")
	if tdx, err := detectTDX(root); err == nil {
		t.Fatalf("TDX = %t, want an error", tdx)
	}
}

func TestWritePlatformEnv(t *testing.T) {
	tests := []struct {
		name, vendor, product, device, want string
	}{
		{"TDX on GCP", "Google", "Google Compute Engine", tdxGuestDevice, "CSP=gcp\nINITDATA_BINDING=yes\n"},
		{"no TDX on GCP", "Google", "Google Compute Engine", "", "CSP=gcp\nINITDATA_BINDING=no\n"},
		{"Azure CVM", "Microsoft Corporation", "Virtual Machine", "", "CSP=azure\nINITDATA_BINDING=yes\n"},
		{"TDX, unknown CSP", "QEMU", "Standard PC", tdxGuestDevice, "INITDATA_BINDING=yes\n"},
		{"no TDX, unknown CSP", "QEMU", "Standard PC", "", "INITDATA_BINDING=no\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := platformFixture(t, tt.vendor, tt.product)
			if tt.device != "" {
				writeFixture(t, root, tt.device, "")
			}
			path := filepath.Join(t.TempDir(), "peerpod", "platform.env")
			if err := writePlatformEnv(root, path); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("platform.env = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestWritePlatformEnvDMIError(t *testing.T) {
	root := t.TempDir()
	// A directory where sys_vendor should be makes reading it fail.
	writeFixture(t, root, "sys/class/dmi/id/sys_vendor/x", "")
	path := filepath.Join(t.TempDir(), "platform.env")
	if err := writePlatformEnv(root, path); err == nil {
		t.Fatal("want an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("platform.env written despite the error: %v", err)
	}
}
