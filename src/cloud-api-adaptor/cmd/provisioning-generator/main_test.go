// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := writeUnit(root, name, contents); err != nil {
		t.Fatal(err)
	}
}

func platformFixture(t *testing.T, vendor, product string) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "sys/class/dmi/id/sys_vendor", vendor+"\n")
	writeFixture(t, root, "sys/class/dmi/id/product_name", product+"\n")
	return root
}

func readOutput(t *testing.T, output, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(output, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func normalizeUnit(contents string) string {
	var lines []string
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestDetectCSP(t *testing.T) {
	tests := []struct {
		name, vendor, product, csp string
	}{
		{"AWS", "Amazon EC2", "c6i.large", "aws"},
		{"GCP", "Google", "Google Compute Engine", "gcp"},
		{"GCP product without vendor", "", "Google Compute Engine", "gcp"},
		{"Alibaba", "Alibaba Cloud", "Alibaba Cloud ECS", "alibaba"},
		{"Azure", "Microsoft Corporation", "Virtual Machine", "azure"},
		{"Hyper-V", "Microsoft Corporation", "Hyper-V UEFI Release", "azure"},
		{"QEMU", "QEMU", "Standard PC (Q35 + ICH9, 2009)", ""},
		{"libvirt", "Red Hat", "KVM", ""},
		{"unknown", "Unknown", "Unknown", ""},
		{"generic KVM", "Unknown", "KVM", ""},
		{"empty DMI", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := platformFixture(t, tt.vendor, tt.product)
			csp, err := detectCSP(root)
			if err != nil || csp != tt.csp {
				t.Fatalf("CSP = %q, error = %v; want %q", csp, err, tt.csp)
			}
		})
	}
}

func TestConfigDriveUnits(t *testing.T) {
	root := platformFixture(t, "Unknown", "Unknown")
	output := t.TempDir()
	if err := generate(output, root, "amd64"); err != nil {
		t.Fatal(err)
	}
	want := "[Unit]\nRequires=media-cidata.mount\nAfter=media-cidata.mount\n" +
		"\n[Service]\nEnvironment=PODVM_PROVISIONING_SOURCE=config-drive\n"
	if got := readOutput(t, output, sourceDropInFile); normalizeUnit(got) != normalizeUnit(want) {
		t.Fatalf("unexpected provisioning drop-in:\n%s", got)
	}
	mount := readOutput(t, output, "media-cidata.mount")
	for _, line := range []string{
		"DefaultDependencies=no\n",
		"What=/dev/disk/by-label/cidata\n",
		"Where=/media/cidata\n",
		"Type=iso9660\n",
		"Options=ro\n",
		"TimeoutSec=30s\n",
	} {
		if !strings.Contains(mount, line) {
			t.Errorf("mount missing %q", line)
		}
	}
	if got := readOutput(t, output, `dev-disk-by\x2dlabel-cidata.device.d/20-provisioning-source.conf`); normalizeUnit(got) != "[Unit]\nJobTimeoutSec=30s\n" {
		t.Fatalf("unexpected device timeout drop-in:\n%s", got)
	}
}

func TestMissingDMIFallsBack(t *testing.T) {
	root := t.TempDir()
	csp, err := detectCSP(root)
	if err != nil || csp != "" {
		t.Fatalf("CSP = %q, error = %v; want empty CSP", csp, err)
	}
	output := t.TempDir()
	if err := generate(output, root, "amd64"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(normalizeUnit(readOutput(t, output, sourceDropInFile)), "Environment=PODVM_PROVISIONING_SOURCE=config-drive\n") {
		t.Fatal("missing DMI did not select config-drive provisioning")
	}
	readOutput(t, output, "media-cidata.mount")
}

func TestIMDSUnits(t *testing.T) {
	for _, tt := range []struct{ source, vendor, product string }{
		{"imds-azure", "Microsoft Corporation", "Virtual Machine"},
		{"imds-aws", "Amazon EC2", "c6i.large"},
		{"imds-gcp", "Google", "Google Compute Engine"},
		{"imds-alibaba", "Alibaba Cloud", "Alibaba Cloud ECS"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			root := platformFixture(t, tt.vendor, tt.product)
			output := t.TempDir()
			if err := generate(output, root, "amd64"); err != nil {
				t.Fatal(err)
			}
			want := "[Unit]\nWants=network-online.target\nAfter=network-online.target\n" +
				"\n[Service]\nEnvironment=PODVM_PROVISIONING_SOURCE=" + tt.source + "\n"
			if got := readOutput(t, output, sourceDropInFile); normalizeUnit(got) != normalizeUnit(want) {
				t.Fatalf("unexpected provisioning drop-in:\n%s", got)
			}
			for _, name := range []string{"media-cidata.mount", `dev-disk-by\x2dlabel-cidata.device.d`} {
				if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected config-drive output %s: %v", name, err)
				}
			}
		})
	}
}

func TestReadFailureBlocksProvisioning(t *testing.T) {
	for _, name := range []string{"sys/class/dmi/id/sys_vendor", "sys/class/dmi/id/product_name"} {
		t.Run(name, func(t *testing.T) {
			root := platformFixture(t, "QEMU", "Standard PC")
			path := filepath.Join(root, name)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			output := t.TempDir()
			if err := generate(output, root, "amd64"); err == nil {
				t.Fatal("expected read error")
			}
			if !strings.Contains(normalizeUnit(readOutput(t, output, sourceDropInFile)), "ExecStartPre=/usr/bin/false\n") {
				t.Fatal("provisioning not blocked")
			}
		})
	}
}

func TestExcludedImages(t *testing.T) {
	for _, test := range []struct{ name, marker, arch string }{
		{"initrd", "etc/initrd-release", "amd64"},
		{"SFTP", "etc/systemd/system/multi-user.target.wants/process-user-data.path", "amd64"},
		{"s390x", "", "s390x"},
		{"arm64", "", "arm64"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.marker != "" {
				path := filepath.Join(root, test.marker)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/not-needed-for-generator", path); err != nil {
					t.Fatal(err)
				}
			}
			output := t.TempDir()
			if err := generate(output, root, test.arch); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 0 {
				t.Fatalf("expected no output, entries = %v, error = %v", entries, err)
			}
		})
	}
}

func TestOutputFailure(t *testing.T) {
	root := platformFixture(t, "Amazon EC2", "c6i.large")
	output := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(output, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(output, root, "amd64"); err == nil {
		t.Fatal("expected output error")
	}
}

func TestGeneratedUnitsVerify(t *testing.T) {
	if os.Getenv("TEST_PODVM_SYSTEMD_VERIFY") != "1" {
		t.Skip("set TEST_PODVM_SYSTEMD_VERIFY=1 to validate generated units with systemd")
	}
	validator, err := exec.LookPath("systemd-analyze")
	if err != nil {
		t.Fatal("TEST_PODVM_SYSTEMD_VERIFY requires systemd-analyze")
	}
	for _, source := range []string{"config-drive", "imds-aws", "failure"} {
		t.Run(source, func(t *testing.T) {
			vendor := ""
			if source == "aws" {
				vendor = "Amazon EC2"
			}
			root := platformFixture(t, vendor, "")
			output := t.TempDir()
			var err error
			if source == "failure" {
				err = blockProvisioning(output, os.ErrPermission)
			} else {
				err = generate(output, root, "amd64")
			}
			if (err != nil) != (source == "failure") {
				t.Fatalf("unexpected generation error: %v", err)
			}
			writeFixture(t, output, "process-user-data.service", `[Unit]
Description=Provisioning generator verification
DefaultDependencies=no

[Service]
Type=oneshot
ExecStart=/usr/bin/true
`)
			writeFixture(t, output, "network-online.target", `[Unit]
Description=Network ordering verification
DefaultDependencies=no
`)
			args := []string{"verify", "--man=no", filepath.Join(output, "process-user-data.service")}
			if source == "config-drive" {
				args = append(args, filepath.Join(output, "media-cidata.mount"))
			}
			cmd := exec.Command(validator, args...)
			cmd.Env = append(os.Environ(), "SYSTEMD_UNIT_PATH="+output, "SYSTEMD_PROC_CMDLINE=", "TMPDIR="+output)
			if result, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("verify generated units: %v\n%s", err, result)
			}
		})
	}
}
