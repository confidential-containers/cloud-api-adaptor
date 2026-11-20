// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package userdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTEE(t *testing.T) {
	tests := []struct {
		name    string
		devices []string
		tee     string
	}{
		{"TDX", []string{"dev/tdx_guest"}, "tdx"},
		{"SNP", []string{"dev/sev-guest"}, "sev-snp"},
		{"both devices", []string{"dev/tdx_guest", "dev/sev-guest"}, "tdx"},
		{"no TEE", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, d := range tt.devices {
				writeFixture(t, root, d, "")
			}
			tee, err := detectTEE(root)
			if err != nil || tee != tt.tee {
				t.Fatalf("TEE = %q, error = %v; want %q", tee, err, tt.tee)
			}
		})
	}
}

func TestWritePlatformEnv(t *testing.T) {
	tests := []struct {
		name, vendor, product, device, want string
	}{
		{"SNP on AWS", "Amazon EC2", "m6a.large", "dev/sev-guest", "TEE=sev-snp\nCSP=aws\n"},
		{"TDX on GCP", "Google", "Google Compute Engine", "dev/tdx_guest", "TEE=tdx\nCSP=gcp\n"},
		{"SNP on Azure", "Microsoft Corporation", "Virtual Machine", "dev/sev-guest", "TEE=sev-snp\nCSP=azure\n"},
		{"no TEE, unknown CSP", "QEMU", "Standard PC", "", "TEE=\nCSP=\n"},
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

func TestDetectTEEStatError(t *testing.T) {
	root := t.TempDir()
	// A file where the dev directory should be makes stat fail with ENOTDIR.
	writeFixture(t, root, "dev", "")
	if tee, err := detectTEE(root); err == nil {
		t.Fatalf("TEE = %q, want an error", tee)
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
