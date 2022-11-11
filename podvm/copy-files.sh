# copy-files.sh is used to copy required files into 
# the correct location on the podvm image

sudo mkdir -p /etc/containers
sudo cp /tmp/files/etc/agent-config.toml /etc/agent-config.toml
sudo cp -a /tmp/files/etc/containers/* /etc/containers/
sudo cp -a /tmp/files/etc/systemd/* /etc/systemd/


sudo mkdir -p /usr/local/bin
sudo cp -a /tmp/files/usr/* /usr/

sudo cp -a /tmp/files/pause_bundle /
