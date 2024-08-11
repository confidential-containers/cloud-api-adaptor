#!/bin/bash
# copy-files.sh is used to copy required files into
# the correct location on the podvm image

sudo mkdir -p /etc/containers
sudo cp /tmp/files/etc/aa-offline_fs_kbc-keys.json /etc/aa-offline_fs_kbc-keys.json
sudo cp -a /tmp/files/etc/containers/* /etc/containers/
sudo cp -a /tmp/files/etc/systemd/* /etc/systemd/
if [ -e /tmp/files/etc/aa-offline_fs_kbc-resources.json ]; then
	sudo cp /tmp/files/etc/aa-offline_fs_kbc-resources.json /etc/aa-offline_fs_kbc-resources.json
fi

if [ -e /tmp/files/etc/certificates/tls.crt ] && [ -e /tmp/files/etc/certificates/tls.key ]; then
        sudo mkdir -p /etc/certificates
	sudo cp -a /tmp/files/etc/certificates/tls.crt /etc/certificates/
	sudo cp -a /tmp/files/etc/certificates/tls.key /etc/certificates/
fi

# If self-signed certificates, then ca certificate needs to be provided
if [ -e /tmp/files/etc/certificates/ca.crt ]; then
	sudo cp -a /tmp/files/etc/certificates/ca.crt /etc/certificates/
fi

sudo mkdir -p /usr/local/bin
sudo cp -a /tmp/files/usr/* /usr/

sudo cp -a /tmp/files/pause_bundle /

# Copy the kata-agent OPA policy files
sudo mkdir -p /etc/kata-opa
sudo cp -a /tmp/files/etc/kata-opa/* /etc/kata-opa/
sudo cp -a /tmp/files/etc/tmpfiles.d/policy.conf /etc/tmpfiles.d/
