#!/bin/bash -e
# Very simple podvm image check intended to be executed on disposable machine
# DO NOT RUN THIS ON YOUR LAPTOP, files might be left behind.
# Requirements listed in ../../../.github/workflows/podvm_smoketest.yaml

set -euo pipefail

SOCAT_PID=""
DESTRUCTIVE=0
IMG=""

usage() {
	echo "Usage: $0 [-d] IMG"
	echo "  IMG     : Required positional argument for the image file."
	echo "  -d      : Destructive, it moves the image and avoids any cleanup (0)."
	exit 1
}

while getopts ":d" opt; do
	case ${opt} in
		d)
			DESTRUCTIVE=1
			;;
		\?)
			echo "Invalid option: -$OPTARG" >&2
			usage
			;;
		:)
			echo "Option -$OPTARG requires an argument." >&2
			usage
			;;
	esac
done

shift $((OPTIND-1))
IMG=$(realpath "$1")

if [ -z "$IMG" ]; then
	echo "Error: Please specify the IMAGE as a positional argument."
	exit 1
fi
FMT=${IMG##*.}

WORKDIR="$(mktemp -d)"
SCRIPTDIR=$(dirname "$(realpath "$0")")


cleanup() {
	# Cleanup (only when DESTRUCTIVE!=1)
	if [ "${DESTRUCTIVE}" -ne 1 ]; then
		set +e
		popd
		[ -n "${SOCAT_PID}" ] && kill "${SOCAT_PID}"
		sudo virsh destroy smoketest
		rm -Rf "${WORKDIR}"
	fi
}

trap 'cleanup' EXIT ERR

# Ensure we have kata-agent-ctl
KATACTL=$(which kata-agent-ctl 2>/dev/null || true)
if [ -z "${KATACTL}" ]; then
	if [ -e kata-agent-ctl ]; then
		KATACTL=$(realpath kata-agent-ctl)
		chmod +x "$KATACTL"
		echo "::debug:: Using kata-agent-ctl from this directory"
	fi
else
	echo "::debug:: Using kata-agent-ctl from PATH ${KATACTL}"
fi
pushd "$WORKDIR"
if [ -z "$KATACTL" ]; then
	if [ "$(uname -m)" != "x86_64" ]; then
		echo "::error:: kata-agent-ctl command not cached for $(uname -m), please compile it yourself and put into PATH or current dir."
		exit 1
	fi
	KATA_REF=$(yq -e '.oci.kata-containers.reference' ${SCRIPTDIR}/../../versions.yaml)
	KATA_REG=$(yq -e '.oci.kata-containers.registry' ${SCRIPTDIR}/../../versions.yaml)
	echo "::debug:: Pulling kata-ctl from ${KATA_REG}/agent-ctl:${KATA_REF}-x86_64"
	oras pull "${KATA_REG}/agent-ctl:${KATA_REF}-x86_64"
	tar -xJvf kata-static-agent-ctl.tar.xz ./opt/kata/bin/kata-agent-ctl --transform='s/opt\/kata\/bin\/kata-agent-ctl/kata-agent-ctl/'
	rm kata-static-agent-ctl.tar.xz
	KATACTL=$(realpath kata-agent-ctl)
	chmod +x "$KATACTL"
fi

# Create cloud-init iso
echo "::debug:: Preparing cloud-init iso"
mkdir cloud-init
touch cloud-init/meta-data
cat <<EOF > cloud-init/user-data
#cloud-config

write_files:
- path: /run/peerpod/daemon.json
  content: |
    {
        "pod-network": {
            "podip": "10.244.1.21/24",
            "pod-hw-addr": "32:b9:59:6b:f0:d5",
            "interface": "eth0",
            "worker-node-ip": "10.224.0.5/16",
            "tunnel-type": "vxlan",
            "routes": [
                {
                    "dst": "0.0.0.0/0",
                    "gw": "10.244.1.1",
                    "dev": "eth0",
                    "protocol": "boot"
                },
                {
                    "dst": "10.244.1.0/24",
                    "gw": "",
                    "dev": "eth0",
                    "protocol": "kernel",
                    "scope": "link"
                }
            ],
            "neighbors": null,
            "mtu": 1500,
            "index": 2,
            "vxlan-port": 8472,
            "vxlan-id": 555002,
            "dedicated": false
        },
        "pod-namespace": "default",
        "pod-name": "smoketest",
        "enable-scratch-disk": true,
        "enable-scratch-encryption": true
    }
EOF
genisoimage -output cloud-init.iso -volid cidata -joliet -rock cloud-init/user-data cloud-init/meta-data

# Move files to libvirt-accessible location
echo "::debug:: Moving files to libvirt-accessible location"
IMAGE=$(realpath "./podvm.${FMT}")
if [ "${DESTRUCTIVE}" -eq 1 ]; then
	mv "${IMG}" "${IMAGE}"
else
	cp "${IMG}" "${IMAGE}"
fi
chmod a+rwx "${WORKDIR}"
sudo chown -R libvirt-qemu "${WORKDIR}" || true
sudo chmod +x "${WORKDIR}"

# Resize the VM disk image to add free space
sudo qemu-img resize -f "${FMT}" "${IMAGE}" +1G

# Start the VM
echo "::debug:: Starting VM"
# TODO: Add AAVMF for arm
[ -e "/usr/share/OVMF/OVMF_CODE.fd" ] && OVMF="/usr/share/OVMF/OVMF_CODE.fd"
sudo virt-install \
	--name smoketest \
	--ram 1024 \
	--vcpus 2 \
	--disk "path=${IMAGE},format=${FMT}" \
	--disk "path=${WORKDIR}/cloud-init.iso,device=cdrom" \
	--import \
	--network network=default \
	--os-variant detect=on,require=off \
	--graphics none \
	--virt-type=kvm \
	--boot loader="${OVMF}" \
	--transient \
	--noautoconsole \
	--channel unix,mode=bind,path=${WORKDIR}/smoketest.agent,target_type=virtio,name=org.qemu.guest_agent.0

SECONDS=0
while [ $SECONDS -lt 120 ]; do
	sleep 5
	VM_IP="$(sudo virsh -q domifaddr smoketest | awk '{print $4}' | cut -d/ -f1)"
	[ -n "${VM_IP}" ] && break
done
if [ -z "${VM_IP}" ]; then
	echo "::error:: Failed to get ipaddr in 120s"
	exit 1
fi

# Perform smoke test
echo "::debug:: Performing the test"
HOST_PORT="${VM_IP}:15150"
SOCK="./agent.sock"
echo "bridge ${HOST_PORT} to ${SOCK}"
socat "UNIX-LISTEN:${SOCK},fork" "TCP:${HOST_PORT}" &
SOCAT_PID=$!

( for _ in {1..5}; do
	$KATACTL connect \
		--server-address "unix://${SOCK}" \
		--cmd Check && exit || true
	sleep 5
	false
done) || { echo "::error:: Failed to connect to peer-pod"; exit 1; }

if ! $KATACTL connect --server-address "unix://${SOCK}" --cmd CreateSandbox; then
	echo "::error:: Failed to CreateSandbox"
	exit 1;
fi

if ! $KATACTL connect --server-address "unix://${SOCK}" --cmd DestroySandbox; then
	echo "::error:: Failed to DestroySandbox"
	exit 1;
fi

# Check encrypted disk mount
# Connect to qemu-ga to run lsblk and process o/p
# qemu-ga expects the command in json format
# virsh qemu-agent-command smoketest '{"execute": "guest-exec", "arguments": { "path": "/usr/bin/lsblk", "capture-output": true }}'
# The o/p will be like '{"return":{"pid":1609}}'.
# Then need to execute guest-exec-status with the returned pid
# virsh qemu-agent-command smoketest '{"execute": "guest-exec-status", "arguments": { "pid": 1609}}'
# The o/p will be like  {"return":{"exitcode":0,"out-data":"TkFNRSAgICAgICAgICAgICAgICAgICAgICAgICBNQ....","exited":true}}
# base64 decoding will of the out-data will show the details
#NAME                         MAJ:MIN RM  SIZE RO TYPE  MOUNTPOINTS
#sda                            8:0    0  2.9G  0 disk
#├─sda1                         8:1    0  512M  0 part
#├─sda2                         8:2    0  345M  0 part
#│ └─root                     252:0    0  345M  1 crypt /
#├─sda3                         8:3    0   64M  0 part
#│ └─root                     252:0    0  345M  1 crypt /
#└─sda4                         8:4    0    1G  0 part
#  └─encrypted_disk_py7P1_dif 252:1    0    1G  0 crypt
#    └─encrypted_disk_py7P1   252:2    0    1G  0 crypt /run/kata-containers/image
#sr0

check_encrypted_disk_mount() {
  local domain="$1"  # e.g., "smoketest"
  local exec_output exec_pid exec_status base64_data decoded_output

  # Run lsblk via guest-exec
  exec_output=$(sudo virsh qemu-agent-command "$domain" '{"execute": "guest-exec", "arguments": { "path": "/usr/bin/lsblk", "capture-output": true }}')
  exec_pid=$(echo "$exec_output" | jq -r '.return.pid')

  if [[ -z "$exec_pid" || "$exec_pid" == "null" ]]; then
    echo "Failed to get PID from guest-exec."
    return 1
  fi

  # Wait for the command to finish
  exec_status=$(sudo virsh qemu-agent-command "$domain" \
       --timeout 5 \
      "{\"execute\": \"guest-exec-status\", \"arguments\": { \"pid\": $exec_pid }}")

  exited=$(echo "$exec_status" | jq -r '.return.exited')

  if [[ "$exited" != "true" ]]; then
    echo "Command did not exit in time."
    return 1
  fi

  # Decode the output
  base64_data=$(echo "$exec_status" | jq -r '.return["out-data"]')
  if [[ -z "$base64_data" || "$base64_data" == "null" ]]; then
    echo "No output from lsblk."
    return 1
  fi

  decoded_output=$(echo "$base64_data" | base64 -d)

  # Check if an encrypted device is mounted at /run/kata-containers/image
  if echo "$decoded_output" | grep -q "crypt.*/run/kata-containers/image"; then
    echo "Encrypted disk is mounted at /run/kata-containers/image"
    return 0
  else
    echo "Encrypted disk is NOT mounted at /run/kata-containers/image"
    return 1
  fi
}

if ! check_encrypted_disk_mount smoketest; then
   echo "::error:: Encrypted disk not found"
   exit 1
fi

