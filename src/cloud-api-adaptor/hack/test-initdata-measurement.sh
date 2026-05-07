#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Confidential Containers Contributors
#
# Integration test: verify that process-user-data correctly measures initdata
# into PCR8 of a vTPM.
#
# The test:
#   1. Creates a minimal initdata TOML body (algorithm=sha384)
#   2. Pre-calculates the expected PCR8 measurement:
#      sha256(32-zero-bytes || sha384(initdata)[:32])
#   3. Builds a cloud-config cidata ISO containing the encoded initdata
#   4. Boots the mkosi debug PodVM image in QEMU with a software vTPM (swtpm)
#   5. Waits for process-user-data.service to complete (which extends PCR8 via
#      ExecStartPost in the service override)
#   6. Reads the actual PCR8 value via qemu-guest-agent
#   7. Compares actual vs expected PCR8
#
# Usage:
#   test-initdata-measurement.sh -i <image> [-o <ovmf>] [-t <timeout>]
#
# Requirements: qemu-system-x86_64, swtpm, socat, xxd, jq, sha384sum,
#               sha256sum, genisoimage or xorriso, gzip, base64

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
IMAGE=""
OVMF=""
BOOT_TIMEOUT=30

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
usage() {
	cat <<EOF
Usage: $(basename "$0") -i <image> [-o <ovmf>] [-t <timeout>]

Options:
  -i <image>    Path to the PodVM debug qcow2 or raw disk image (required)
  -o <ovmf>     Path to OVMF firmware file (auto-detected if omitted)
  -t <timeout>  Total boot timeout in seconds (default: 120)
  -h            Show this help message
EOF
	exit 1
}

die() {
	echo "ERROR: $*" >&2
	exit 1
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
while getopts "i:o:t:h" opt; do
	case "$opt" in
	i) IMAGE="$OPTARG" ;;
	o) OVMF="$OPTARG" ;;
	t) BOOT_TIMEOUT="$OPTARG" ;;
	h) usage ;;
	*) usage ;;
	esac
done

[[ -n "$IMAGE" ]] || die "image path is required (-i)"
[[ -f "$IMAGE" ]] || die "image file not found: $IMAGE"

# ---------------------------------------------------------------------------
# Detect OVMF firmware
# ---------------------------------------------------------------------------
if [[ -z "$OVMF" ]]; then
	for candidate in \
		/usr/share/edk2/ovmf/OVMF_CODE.fd \
		/usr/share/OVMF/OVMF_CODE.fd \
		/usr/share/OVMF/OVMF_CODE_4M.fd \
		/usr/share/qemu/OVMF.fd \
		/usr/share/edk2-ovmf/OVMF_CODE.fd; do
		if [[ -f "$candidate" ]]; then
			OVMF="$candidate"
			break
		fi
	done
fi
[[ -n "$OVMF" && -f "$OVMF" ]] || \
	die "OVMF firmware not found; specify with -o <path>"

# ---------------------------------------------------------------------------
# Check required tools
# ---------------------------------------------------------------------------
for tool in qemu-system-x86_64 swtpm socat jq xxd sha384sum sha256sum gzip base64; do
	command -v "$tool" &>/dev/null || die "required tool '$tool' not found"
done

# Find an ISO creation tool
ISO_TOOL=""
for tool in genisoimage xorriso mkisofs; do
	if command -v "$tool" &>/dev/null; then
		ISO_TOOL="$tool"
		break
	fi
done
[[ -n "$ISO_TOOL" ]] || die "no ISO creation tool found (need genisoimage, xorriso, or mkisofs)"

# ---------------------------------------------------------------------------
# Temporary workspace
# ---------------------------------------------------------------------------
WORKDIR=$(mktemp -d /tmp/test-initdata-XXXXXX)
QEMU_PID=""
SWTPM_PID=""

cleanup() {
	if [[ -n "$QEMU_PID" ]]; then
		kill "$QEMU_PID" 2>/dev/null || true
		wait "$QEMU_PID" 2>/dev/null || true
	fi
	if [[ -n "$SWTPM_PID" ]]; then
		kill "$SWTPM_PID" 2>/dev/null || true
	fi
	rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "Workspace: $WORKDIR"

# ---------------------------------------------------------------------------
# Step 1: Create initdata TOML
# ---------------------------------------------------------------------------
echo
echo "==> Step 1: Creating initdata TOML..."

# A minimal but valid initdata body.  The empty aa.toml entry satisfies the
# file-extraction loop in extractInitdataAndHash without requiring any real
# attestation configuration.
cat >"$WORKDIR/initdata.toml" <<'EOF'
algorithm = "sha384"
version = "0.1.0"

# randomized body: $WORKDIR
[data]
"aa.toml" = ""
EOF

echo "initdata TOML:"
cat "$WORKDIR/initdata.toml"

# ---------------------------------------------------------------------------
# Step 2: Pre-calculate expected PCR8
# ---------------------------------------------------------------------------
echo
echo "==> Step 2: Pre-calculating expected PCR8..."

# The service override extends PCR8 via:
#   tpm2_pcrextend 8:sha256=$(head -c64 /run/peerpod/initdata.digest)
#
# initdata.digest is the SHA384 hex string of the raw TOML bytes.
# head -c64 takes the first 64 ASCII hex characters = the first 32 bytes of
# the SHA384 digest.
#
# TPM PCR extension formula (PCR8 starts at all-zeros):
#   new_PCR8 = SHA256(current_PCR8 || data)
#            = SHA256(00...00_32bytes || sha384(initdata)[:32])

SHA384_HEX=$(sha384sum "$WORKDIR/initdata.toml" | awk '{print $1}')
echo "SHA384(initdata): $SHA384_HEX"

# First 64 hex chars = 32 bytes  (what tpm2_pcrextend receives)
TRUNCATED_HEX="${SHA384_HEX:0:64}"
echo "Truncated to 32B: $TRUNCATED_HEX"

# SHA256(32-zero-bytes || 32-byte-truncated-sha384)
ZEROES_HEX="0000000000000000000000000000000000000000000000000000000000000000"
EXPECTED_PCR8=$(
	printf '%s%s' "$ZEROES_HEX" "$TRUNCATED_HEX" |
		xxd -r -p |
		sha256sum |
		awk '{print $1}'
)
echo "Expected PCR8:    $EXPECTED_PCR8"

# ---------------------------------------------------------------------------
# Step 3: Encode initdata and create the cloud-config user-data
# ---------------------------------------------------------------------------
echo
echo "==> Step 3: Building cloud-config and cidata ISO..."

# process-user-data reads /media/cidata/user-data as a cloud-config YAML.
# The write_files entry writes the encoded initdata to /run/peerpod/initdata,
# which extractInitdataAndHash then decodes and hashes.
INITDATA_B64=$(gzip -c "$WORKDIR/initdata.toml" | base64 -w0)

{
	echo '#cloud-config'
	echo 'write_files:'
	echo '  - path: /run/peerpod/initdata'
	# Use a double-quoted YAML scalar so the base64 string is written verbatim
	# (no trailing newline) – required because Go's base64.StdEncoding is strict.
	printf '    content: "%s"\n' "$INITDATA_B64"
} >"$WORKDIR/user-data"

# Minimal meta-data required by the cidata schema
echo "instance-id: test-initdata-$(date +%s)" >"$WORKDIR/meta-data"

# Create the cidata ISO (must carry the volume label "cidata" so the
# process-user-data service override can mount it at /media/cidata)
if [[ "$ISO_TOOL" == "xorriso" ]]; then
	xorriso -as mkisofs -V cidata -o "$WORKDIR/cidata.iso" \
		"$WORKDIR/user-data" "$WORKDIR/meta-data"
else
	"$ISO_TOOL" -V cidata -J -r -o "$WORKDIR/cidata.iso" \
		"$WORKDIR/user-data" "$WORKDIR/meta-data"
fi
echo "Created: $WORKDIR/cidata.iso"

# ---------------------------------------------------------------------------
# Step 4: Start software TPM
# ---------------------------------------------------------------------------
echo
echo "==> Step 4: Starting swtpm..."

mkdir -p "$WORKDIR/vtpm"
swtpm socket \
	--tpmstate dir="$WORKDIR/vtpm" \
	--ctrl type=unixio,path="$WORKDIR/vtpm/swtpm.sock" \
	--tpm2 \
	--log level=0 &
SWTPM_PID=$!

# Give swtpm a moment to create its socket
for _ in {1..10}; do
	[[ -S "$WORKDIR/vtpm/swtpm.sock" ]] && break
	sleep 1
done
[[ -S "$WORKDIR/vtpm/swtpm.sock" ]] || die "swtpm socket was not created"
echo "swtpm PID: $SWTPM_PID"

# ---------------------------------------------------------------------------
# Step 5: Boot the VM in QEMU
# ---------------------------------------------------------------------------
echo
echo "==> Step 5: Starting QEMU VM..."

CONSOLE_LOG="$WORKDIR/console.log"
touch "$CONSOLE_LOG"

# Copy OVMF firmware to WORKDIR to avoid permission issues (e.g. on CI runners
# where /usr/share/OVMF may not be readable by the QEMU process).
OVMF_LOCAL="$WORKDIR/OVMF_CODE.fd"
cp "$OVMF" "$OVMF_LOCAL"

# Detect image format from file extension
IMG_FORMAT="${IMAGE##*.}"

# Use KVM acceleration when available
CPU_FLAGS=()
if [[ -w /dev/kvm ]]; then
	MACHINE_FLAGS="type=q35,accel=kvm,smm=off"
	CPU_FLAGS=(-cpu host)
	echo "KVM acceleration enabled"
else
	echo "Warning: /dev/kvm not available – QEMU will use TCG (slow)" >&2
	MACHINE_FLAGS="type=q35,smm=off"
fi

qemu-system-x86_64 \
	-machine "$MACHINE_FLAGS" \
	"${CPU_FLAGS[@]+"${CPU_FLAGS[@]}"}" \
	-m "1536M" \
	-drive "file=$OVMF_LOCAL,format=raw,if=pflash" \
	-drive "file=$IMAGE,format=$IMG_FORMAT" \
	-drive "file=$WORKDIR/cidata.iso,media=cdrom" \
	-chardev "socket,id=chrtpm,path=$WORKDIR/vtpm/swtpm.sock" \
	-tpmdev "emulator,id=tpm0,chardev=chrtpm" \
	-device tpm-tis,tpmdev=tpm0 \
	-device virtio-serial \
	-monitor none \
	-serial "file:$CONSOLE_LOG" \
	-nographic \
	-no-reboot &
QEMU_PID=$!
echo "QEMU PID: $QEMU_PID"

# ---------------------------------------------------------------------------
# Step 6: Wait for VM to reach a login prompt (indicates boot completion)
# ---------------------------------------------------------------------------
echo
echo "==> Step 6: Waiting for VM to reach login prompt (timeout: ${BOOT_TIMEOUT}s)..."
for _ in {1..60}; do
    grep -q "login:" "$CONSOLE_LOG" && break
    sleep 2
done
grep -q "login:" "$CONSOLE_LOG" || die "VM did not reach login prompt within ${BOOT_TIMEOUT}s"
echo "VM is ready."

# ---------------------------------------------------------------------------
# Step 7: Scrub PCR8 from console log
# ---------------------------------------------------------------------------
echo
echo "==> Step 7: Extracting PCR8 value from console log..."

ACTUAL_PCR8=$(grep -A 40 'Detected vTPM PCR values:' "$CONSOLE_LOG" \
    | grep '^\s*8 :' \
    | awk '{print $3}' \
    | sed 's/^0x//' \
    | tr '[:upper:]' '[:lower:]' \
    | tr -d '[:space:]')
[[ -n "$ACTUAL_PCR8" ]] || die "PCR8 value not found in console log"

# ---------------------------------------------------------------------------
# Step 8: Compare
# ---------------------------------------------------------------------------
echo
echo "==> Step 8: Comparing expected vs actual PCR8..."
echo "Expected PCR8: $EXPECTED_PCR8"
echo "Actual PCR8:   $ACTUAL_PCR8"

if [[ "$ACTUAL_PCR8" == "$EXPECTED_PCR8" ]]; then
	echo
	echo "PASS: initdata PCR8 measurement matches the pre-calculated value."
	exit 0
else
	echo
	echo "FAIL: PCR8 measurement does NOT match." >&2
	exit 1
fi
