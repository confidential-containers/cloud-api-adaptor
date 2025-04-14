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
sudo chown -R libvirt-qemu "${WORKDIR}" || true
sudo chmod +x "${WORKDIR}"

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
	--noautoconsole

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
