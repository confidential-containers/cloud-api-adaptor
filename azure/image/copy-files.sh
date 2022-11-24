sudo mkdir -p /etc/containers
sudo cp /tmp/files/etc/agent-config.toml /etc/agent-config.toml
sudo cp -a /tmp/files/etc/containers/* /etc/containers/
sudo cp -a /tmp/files/etc/systemd/* /etc/systemd/


sudo mkdir -p /usr/local/bin
sudo cp -a /tmp/files/usr/* /usr/

sudo cp -a /tmp/files/pause_bundle /

if [ -e /tmp/files/auth.json ]; then
       sudo mkdir -p /root/.config/containers/
       sudo cp -a /tmp/files/auth.json /root/.config/containers/auth.json
fi
