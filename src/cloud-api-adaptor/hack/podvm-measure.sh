#!/bin/bash

set -euo pipefail

# Usage function
usage() {
	cat <<EOF
Usage: $(basename "$0") [-h] <command> [args]

Available commands:
    launch Launch a QEMU guest; -i <image_file> (required); -o <ovmf_code>
    swtpm  Set up a software TPM
    wait   Wait for guest boot completion
    scrape Scrape guest output or logs
    stop   Stop the QEMU guest

Options:
    -h          Display this help message

Example:
    $(basename "$0") launch
EOF
	exit 1
}

# Parse global options
while getopts "i:o:h" opt; do
	case ${opt} in
	i)
		image="$OPTARG"
		;;
	o)
		ovmf="$OPTARG"
		;;
	h)
		usage
		;;
	\?)
		echo "Invalid option" >&2
		usage
		;;
	esac
done
shift $((OPTIND - 1))

# Subcommand: launch
launch_guest() {
	if [[ -z "${image:-}" ]]; then
		usage
	fi
	echo "Launching QEMU guest..."
	qemu-system-x86_64 \
		-machine type=q35,accel=kvm,smm=off \
		-m 1024 \
		-cpu host \
		-drive file="${ovmf:-./OVMF_CODE.fd},format=raw,if=pflash" \
		-drive file="${image},format=${image##*.}" \
		-chardev socket,id=chrtpm,path=./vtpm/swtpm.sock \
		-tpmdev emulator,id=tpm0,chardev=chrtpm \
		-device tpm-tis,tpmdev=tpm0 \
		-nographic \
		-serial file:serial.log \
		-monitor telnet::45454,server,nowait
}

# Subcommand: stop
stop_guest() {
	echo "Stopping QEMU guest..."
	echo q | nc localhost 45454
}

# Subcommand: swtpm
setup_swtpm() {
	echo "Setting up software TPM..."
	mkdir -p vtpm
	swtpm socket \
		--tpmstate dir=./vtpm \
		--ctrl type=unixio,path=./vtpm/swtpm.sock \
		--tpm2
}

# Subcommand: wait
wait_for_guest() {
	echo "Waiting for guest to boot..."
	timeout=60
	elapsed=0
	touch serial.log
	while ! grep -q "login:" serial.log; do
		sleep 2
		elapsed="$((elapsed + 2))"
		if [ "$elapsed" -ge "$timeout" ]; then
			echo "Guest failed to boot within ${timeout}s." >&2
			exit 1
		fi
	done
}

# Subcommand: scrape
scrape_logs() {
	# Extract the PCR block from the log.

	# This assumes the block starts with "Detected vTPM PCR values:"
	# and ends when a non-indented line appears.
	pcr_block=$(sed -n '/Detected vTPM PCR values:/,/^[^[:space:]]/p' serial.log)

	# detect line "sha***:"
	alg=$(echo "$pcr_block" | grep -E "^[[:space:]]*sha.*:" | awk -F':' '{print $1}' | tr -d '[:space:]')
	echo -n "{\"measurements\":{\"${alg}\":{"
	# Now filter for PCRs 3, 9, and 11.
	echo "$pcr_block" | grep -E "^[[:space:]]*(3|9|11)[[:space:]]*:" | while read -r line; do
		# Use awk to split the line at the colon.
		index=$(echo "$line" | awk -F':' '{print $1}' | tr -d '[:space:]')
		value=$(echo "$line" | awk -F':' '{print tolower($2)}' | tr -d '[:space:]')
		echo -n "${need_sep:+,}\"pcr$(printf "%02d" "$index")\":\"$value\""
		need_sep=1
	done
	echo "}}}"
}

# Main command dispatch
main() {
	[[ $# -lt 1 ]] && usage

	cmd="$1"
	shift

	case "$cmd" in
	launch)
		launch_guest "$@"
		;;
	stop)
		stop_guest "$@"
		;;
	swtpm)
		setup_swtpm "$@"
		;;
	wait)
		wait_for_guest "$@"
		;;
	scrape)
		scrape_logs "$@"
		;;
	*)
		echo "Unknown command: $cmd" >&2
		usage
		;;
	esac
}

main "$@"
