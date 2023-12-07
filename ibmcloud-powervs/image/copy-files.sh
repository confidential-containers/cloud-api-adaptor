#!/bin/bash
# copy-files.sh is used to copy required files into
# the correct location on the podvm image

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
PODVM_DIR=${REPO_ROOT}/podvm

sudo mkdir -p /etc/containers
sudo cp ${PODVM_DIR}/files/etc/agent-config.toml /etc/agent-config.toml
sudo cp /tmp/files/etc/aa-offline_fs_kbc-keys.json /etc/aa-offline_fs_kbc-keys.json

if [ -n "${FORWARDER_PORT}" ]; then
    cat <<END >> /etc/default/agent-protocol-forwarder 
OPTIONS=-listen 0.0.0.0:${FORWARDER_PORT}
END
fi

sudo cp -a ${PODVM_DIR}/files/etc/containers/* /etc/containers/
sudo cp -a ${PODVM_DIR}/files/etc/systemd/* /etc/systemd/
if [ -e ${PODVM_DIR}/files/etc/aa-offline_fs_kbc-resources.json ]; then
	sudo cp ${PODVM_DIR}/files/etc/aa-offline_fs_kbc-resources.json /etc/aa-offline_fs_kbc-resources.json
fi


sudo mkdir -p /usr/local/bin
sudo cp -a ${PODVM_DIR}/files/usr/* /usr/

sudo cp -a ${PODVM_DIR}/files/pause_bundle /

if [ -e ${PODVM_DIR}/files/auth.json ]; then
       sudo mkdir -p /root/.config/containers/
       sudo cp -a ${PODVM_DIR}/files/auth.json /root/.config/containers/auth.json
fi
