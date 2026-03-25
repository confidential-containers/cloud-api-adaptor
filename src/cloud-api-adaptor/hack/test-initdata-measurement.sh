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
#   6. Reads the actual PCR8 value via the serial console
#   7. Compares actual vs expected PCR8
#
# Usage:
#   test-initdata-measurement.sh -i <image> [-o <ovmf>] [-t <timeout>]
#
# Requirements: qemu-system-x86_64, swtpm, socat, xxd, sha384sum, sha256sum,
#               genisoimage or xorriso, gzip, base64

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
IMAGE=""
OVMF=""
BOOT_TIMEOUT=120

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
for tool in qemu-system-x86_64 swtpm socat xxd sha384sum sha256sum gzip base64; do
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
for i in $(seq 10); do
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

SERIAL_SOCK="$WORKDIR/serial.sock"
CONSOLE_LOG="$WORKDIR/console.log"
touch "$CONSOLE_LOG"

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

# The serial chardev logs all VM output to CONSOLE_LOG and also accepts input
# via the UNIX socket, which we use to run commands after auto-login.
qemu-system-x86_64 \
	-machine "$MACHINE_FLAGS" \
	"${CPU_FLAGS[@]+"${CPU_FLAGS[@]}"}" \
	-m 1024 \
	-drive "file=$OVMF,format=raw,if=pflash" \
	-drive "file=$IMAGE,format=$IMG_FORMAT" \
	-drive "file=$WORKDIR/cidata.iso,media=cdrom" \
	-chardev "socket,id=chrtpm,path=$WORKDIR/vtpm/swtpm.sock" \
	-tpmdev "emulator,id=tpm0,chardev=chrtpm" \
	-device tpm-tis,tpmdev=tpm0 \
	-chardev "socket,id=serial0,path=$SERIAL_SOCK,server=on,wait=off,logfile=$CONSOLE_LOG" \
	-serial chardev:serial0 \
	-nographic \
	-no-reboot &
QEMU_PID=$!
echo "QEMU PID: $QEMU_PID"

# Wait for the serial socket to appear
for i in $(seq 30); do
	[[ -S "$SERIAL_SOCK" ]] && break
	sleep 1
done
[[ -S "$SERIAL_SOCK" ]] || die "QEMU serial socket was not created"

# ---------------------------------------------------------------------------
# Step 6: Interact with the serial console
# ---------------------------------------------------------------------------
echo
echo "==> Step 6: Waiting for boot and service completion (timeout: ${BOOT_TIMEOUT}s)..."

# The debug image uses auto-login on the serial console.  After the system
# reaches multi-user.target, we:
#   1. Press Enter to ensure the shell prompt is active.
#   2. Wait for process-user-data.service to finish (it extends PCR8 in its
#      ExecStartPost).
#   3. Read PCR8 with tpm2_pcrread and save the output.
#   4. Power off the VM.
#
# Half the timeout is used for the initial boot delay; the remainder covers
# service completion and command execution.
HALF_TIMEOUT=$((BOOT_TIMEOUT / 2))

(
	# Wait for the VM to boot and auto-login
	sleep "$HALF_TIMEOUT"

	# Wake up the terminal / ensure the shell prompt is active
	printf '\n'
	sleep 2

	# In case auto-login did not fire, attempt a root login
	printf 'root\n'
	sleep 2

	# Wait synchronously for process-user-data to reach an inactive-dead or
	# active-exited state before reading PCR8.
	printf 'until systemctl is-active process-user-data.service 2>/dev/null || systemctl status process-user-data.service 2>&1 | grep -q "Deactivated\|inactive\|failed"; do sleep 1; done\n'
	sleep 5

	# Read PCR8 and mark the output so we can locate it in the log
	printf 'echo "PCR8_BEGIN"; tpm2_pcrread sha256:8; echo "PCR8_END"\n'
	sleep 5

	# Shut down cleanly
	printf 'poweroff\n'
	sleep 10
) | socat - "UNIX-CONNECT:$SERIAL_SOCK" >/dev/null 2>&1 &

# Wait for QEMU to exit (after poweroff), respecting the total timeout
echo "Waiting for VM to shut down..."
if ! timeout "$((BOOT_TIMEOUT + 30))" wait "$QEMU_PID" 2>/dev/null; then
	echo "Warning: VM did not shut down within timeout; continuing" >&2
fi
QEMU_PID=""

# ---------------------------------------------------------------------------
# Step 7: Parse the actual PCR8 value from the console log
# ---------------------------------------------------------------------------
echo
echo "==> Step 7: Parsing PCR8 from console log..."

if [[ ! -s "$CONSOLE_LOG" ]]; then
	echo "Console log is empty; VM may have failed to boot." >&2
	exit 1
fi

# Extract the lines between the PCR8_BEGIN/PCR8_END markers (most recent run)
PCR8_SECTION=$(sed -n '/PCR8_BEGIN/,/PCR8_END/{/PCR8_BEGIN/d;/PCR8_END/d;p}' \
	"$CONSOLE_LOG" | tail -5)

# tpm2_pcrread sha256:8 output format:
#   sha256:
#     8: 0x<64-hex-chars>
ACTUAL_PCR8=$(echo "$PCR8_SECTION" |
	grep -E '^\s+8\s*:\s*0x' |
	tail -1 |
	awk '{print $NF}' |
	sed 's/^0x//' |
	tr '[:upper:]' '[:lower:]')

if [[ -z "$ACTUAL_PCR8" ]]; then
	echo "Could not find PCR8 value in console log." >&2
	echo "--- Relevant console output ---"
	grep -A5 "PCR8_BEGIN" "$CONSOLE_LOG" 2>/dev/null || true
	echo "--- Last 60 lines of console log ---"
	tail -60 "$CONSOLE_LOG"
	exit 1
fi

# ---------------------------------------------------------------------------
# Step 8: Compare
# ---------------------------------------------------------------------------
echo
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
