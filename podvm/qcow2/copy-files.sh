# copy-files.sh is used to copy required files into 
# the correct location on the podvm image

sudo mkdir -p /etc/containers
sudo cp /tmp/files/etc/agent-config.toml /etc/agent-config.toml
sudo cp -a /tmp/files/etc/containers/* /etc/containers/
sudo cp -a /tmp/files/etc/systemd/* /etc/systemd/
sudo mv /etc/systemd/system/run-kata-containers.mount /etc/systemd/system/run-kata\\x2dcontainers.mount
sudo mv /etc/systemd/system/multi-user.target.wants/run-kata-containers.mount /etc/systemd/system/multi-user.target.wants/run-kata\\x2dcontainers.mount
if [ -e /tmp/files/etc/aa-offline_fs_kbc-resources.json ]; then
	sudo cp /tmp/files/etc/aa-offline_fs_kbc-resources.json /etc/aa-offline_fs_kbc-resources.json
fi


sudo mkdir -p /usr/local/bin
sudo cp -a /tmp/files/usr/* /usr/

sudo cp -a /tmp/files/pause_bundle /
