#!/bin/bash -e
# Very simple podvm image check intended to be executed on disposable machine
# DO NOT RUN THIS ON YOUR LAPTOP, files might be left behind.
# Requirements listed in ../../../.github/workflows/podvm_smoketest.yaml

set -euo pipefail

SOCAT_PID=""
DESTRUCTIVE=0
IMG=""

# Check encrypted device mount
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

usage() {
	echo "Usage: $0 [-d] IMG"
	echo "  IMG     : Required positional argument for the image file."
	echo "  -d      : Destructive, it moves the image and avoids any cleanup (0)."
	exit 1
}

cleanup() {
	local exit_code=$?
	# Cleanup (only when DESTRUCTIVE!=1)
	if [ "${exit_code}" -ne 0 ]; then
		echo "Serial log of ${VM_NAME:-smoketest}"
		sudo cat "/var/log/libvirt/qemu/${VM_NAME:-smoketest}.serial.log" || echo "Failed to read /var/log/libvirt/qemu/${VM_NAME:-smoketest}.serial.log"
	fi
	if [ "${DESTRUCTIVE}" -ne 1 ]; then
		set +e
		popd >/dev/null 2>&1 || true
		[ -n "${SOCAT_PID}" ] && kill "${SOCAT_PID}" >/dev/null 2>&1
		sudo virsh destroy "${VM_NAME:-smoketest}" >/dev/null 2>&1 || true
		sudo rm -Rf "${WORKDIR}" || true
	fi
}

trap 'cleanup' EXIT ERR

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

echo "::debug:: Running smoke test"
FMT=${IMG##*.}

WORKDIR="$(mktemp -d)"
SCRIPTDIR=$(dirname "$(realpath "$0")")

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
	KATA_REF=$(yq -e '.oci.kata-containers.reference' "${SCRIPTDIR}/../../versions.yaml")
	KATA_REG=$(yq -e '.oci.kata-containers.registry' "${SCRIPTDIR}/../../versions.yaml")
	echo "::debug:: Pulling kata-ctl from ${KATA_REG}/agent-ctl:${KATA_REF}-x86_64"
	oras pull "${KATA_REG}/agent-ctl:${KATA_REF}-x86_64"
	tar --ztsd -xvf kata-static-agent-ctl.tar.zst ./opt/kata/bin/kata-agent-ctl --transform='s/opt\/kata\/bin\/kata-agent-ctl/kata-agent-ctl/'
	rm kata-static-agent-ctl.tar.zst
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
- path: /run/peerpod/apf.json
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
        "pod-name": "smoketest"
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
sudo chown -R libvirt-qemu "${WORKDIR}" || sudo chown -R qemu "${WORKDIR}" || true
sudo chmod +x "${WORKDIR}"

# Start the VM
echo "::debug:: Starting VM"
VM_NAME="smoketest"
sudo virt-install \
	--name "${VM_NAME}" \
	--ram 2048 \
	--vcpus 2 \
	--disk "path=${IMAGE},format=${FMT}" \
	--disk "path=${WORKDIR}/cloud-init.iso,device=cdrom" \
	--import \
	--network network=default \
	--os-variant detect=on,require=off \
	--graphics none \
	--virt-type=kvm \
	--boot uefi \
	--transient \
	--noautoconsole \
	--channel "unix,mode=bind,path=${WORKDIR}/${VM_NAME}.agent,target_type=virtio,name=org.qemu.guest_agent.0" \
	--serial "file,path=/var/log/libvirt/qemu/${VM_NAME}.serial.log"

SECONDS=0
while [ $SECONDS -lt 120 ]; do
	sleep 5
	VM_IP="$(sudo virsh -q domifaddr "${VM_NAME}" | awk '{print $4}' | cut -d/ -f1)"
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
