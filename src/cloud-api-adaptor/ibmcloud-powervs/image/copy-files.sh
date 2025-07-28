#!/bin/bash
# copy-files.sh is used to copy required files into
# the correct location on the podvm image

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
PODVM_DIR=${REPO_ROOT}/podvm

sudo mkdir -p /etc/containers
sudo cp "${PODVM_DIR}"/files/etc/agent-config.toml /etc/agent-config.toml
sudo cp "${PODVM_DIR}"/files/etc/aa-offline_fs_kbc-keys.json /etc/aa-offline_fs_kbc-keys.json

if [ -n "${FORWARDER_PORT}" ]; then
    cat <<END >> /etc/default/agent-protocol-forwarder 
OPTIONS=-listen 0.0.0.0:${FORWARDER_PORT}
END
fi

sudo cp -a "${PODVM_DIR}"/files/etc/containers/* /etc/containers/
sudo cp -a "${PODVM_DIR}"/files/etc/systemd/* /etc/systemd/
if [ -e "${PODVM_DIR}"/files/etc/aa-offline_fs_kbc-resources.json ]; then
    sudo cp "${PODVM_DIR}"/files/etc/aa-offline_fs_kbc-resources.json /etc/aa-offline_fs_kbc-resources.json
fi

if [ -e "${PODVM_DIR}"/files/etc/certificates/tls.crt ] && [ -e "${PODVM_DIR}"/files/etc/certificates/tls.key ]; then
    sudo mkdir -p /etc/certificates
    sudo cp "${PODVM_DIR}"/files/etc/certificates/tls.{key,crt} /etc/certificates/
fi

# If self-signed certificates, then ca certificate needs to be provided
if [ -e "${PODVM_DIR}"/files/etc/certificates/ca.crt ]; then
    sudo cp "${PODVM_DIR}"/files/etc/certificates/ca.crt /etc/certificates/
fi

if [ "${ENABLE_SFTP}" = "true" ]; then
    sudo cp -a "${PODVM_DIR}"/files/etc/systemd-addons/* /etc/systemd/system/
    systemctl disable process-user-data
    systemctl enable process-user-data.path
fi

sudo mkdir -p /usr/local/bin
sudo cp -a "${PODVM_DIR}"/files/usr/* /usr/

sudo cp -a "${PODVM_DIR}"/files/pause_bundle /

# Copy the kata-agent OPA policy files
sudo mkdir -p /etc/kata-opa
sudo cp -a "${PODVM_DIR}"/files/etc/kata-opa/* /etc/kata-opa/
sudo cp -a "${PODVM_DIR}"/files/etc/tmpfiles.d/policy.conf /etc/tmpfiles.d/

# Copy an empty auth.json for image pulling
sudo mkdir -p /etc/kata-oci
sudo cp -a "${PODVM_DIR}"/files/etc/kata-oci/* /etc/kata-oci/
sudo cp -a "${PODVM_DIR}"/files/etc/tmpfiles.d/auth.conf /etc/tmpfiles.d/
