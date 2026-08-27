#!/bin/bash
# This script must be run as root. It provisions an existing VM as a BYOM pod VM
# by configuring SSH/SFTP access and deploying the podvm binaries and systemd services.

set -o errexit
set -o pipefail
set -o nounset

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Error: this script must be run as root."
    exit 1
fi

PODVM_BYOM_BINARIES_IMAGE=${PODVM_BYOM_BINARIES_IMAGE:-"podvm-byom-binaries-ubuntu-amd64:latest"}
PODVM_BYOM_TAR_NAME=${PODVM_BYOM_TAR_NAME:-"podvm-byom.tar.gz"}
DISABLE_SSH_LOGIN=${DISABLE_SSH_LOGIN:-"false"}
USER_NAME=${USER_NAME:-"peerpod"}
SSH_PUBLIC_KEY_PATH=${SSH_PUBLIC_KEY_PATH:-""}

if [ -z "${SSH_PUBLIC_KEY_PATH}" ]; then
    echo "Error: SSH_PUBLIC_KEY_PATH is not set."
    exit 1
fi

# sshd config
SSH_CONFIG="/etc/ssh/sshd_config"

# Backup existing sshd_config
cp "${SSH_CONFIG}" "/etc/ssh/sshd_config.bak.$(date +%F_%T)"

# Append SSH config to disable login
if [[ "$DISABLE_SSH_LOGIN" == "true" ]]; then
cat >> "$SSH_CONFIG" <<EOF
# Disable all forms of login
PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM yes
PubkeyAuthentication yes

EOF
fi

if ! id ${USER_NAME} >/dev/null 2>&1; then
    echo "User ${USER_NAME} not found, creating new user"
    mkdir -p /home/${USER_NAME}
    useradd -r -s /sbin/nologin -d /home/${USER_NAME} ${USER_NAME}
fi

mkdir -p /home/${USER_NAME}/.ssh && chmod 700 /home/${USER_NAME}/.ssh
cat ${SSH_PUBLIC_KEY_PATH} >> /home/${USER_NAME}/.ssh/authorized_keys
chmod 600 /home/${USER_NAME}/.ssh/authorized_keys
chown -R ${USER_NAME}:${USER_NAME} /home/${USER_NAME}

# Ensure internal-sftp subsystem is set; replace existing line or append if absent
if grep -qE "^\s*Subsystem\s+sftp" "$SSH_CONFIG"; then
    sed -i -E 's/^\s*Subsystem\s+sftp.*/Subsystem sftp internal-sftp/' "$SSH_CONFIG"
else
    echo "Subsystem sftp internal-sftp" >> "$SSH_CONFIG"
fi

# Write Match User block directly to sshd_config if not already present
if ! grep -qE "^\s*Match\s+User\s+${USER_NAME}\b" "$SSH_CONFIG"; then
    cat >> "$SSH_CONFIG" <<EOF

# Restrict ${USER_NAME} user to SFTP only with chroot
Match User ${USER_NAME}
    ForceCommand internal-sftp
    ChrootDirectory /media
    PermitTunnel no
    AllowAgentForwarding no
    AllowTcpForwarding no
    X11Forwarding no
EOF
fi

# Validate config then reload or restart SSH service
echo "Validating SSH config..."
if ! sshd -t; then
    echo "Error: Invalid SSH configuration. Restart aborted."
    exit 1
fi

if systemctl is-active --quiet sshd; then
    systemctl restart sshd || { echo "Error: failed to restart sshd."; exit 1; }
elif systemctl is-active --quiet ssh; then
    systemctl restart ssh || { echo "Error: failed to restart ssh."; exit 1; }
else
    echo "Error: SSH service not found or inactive."
    exit 1
fi

# Create a docker container to extract the contents
docker rm -f podvm-container 2>/dev/null || true
docker create --name podvm-container ${PODVM_BYOM_BINARIES_IMAGE} true
docker cp podvm-container:/${PODVM_BYOM_TAR_NAME} ./${PODVM_BYOM_TAR_NAME}
docker rm podvm-container

# Create podvm contents target directory
rm -rf /tmp/files && mkdir -p /tmp/files

# Extract the tarball contents into /tmp/files
tar xvf ./${PODVM_BYOM_TAR_NAME} -C /tmp/files
rm -f ./${PODVM_BYOM_TAR_NAME}

# Run the helper scripts from /tmp/files
source /tmp/files/copy-files.sh

# Change mount point of kata-containers to type tmpfs.
KATA_MOUNT_FILE=/etc/systemd/system/'run-kata\x2dcontainers.mount'

if grep -q '^Type=none' "$KATA_MOUNT_FILE" && grep -q '^Options=bind' "$KATA_MOUNT_FILE"; then
  sudo sed -i 's/^Type=none/Type=tmpfs/; s/^Options=bind/Options=mode=0755,uid=root,gid=root/' "$KATA_MOUNT_FILE"
  echo "Updated $KATA_MOUNT_FILE to Type=tmpfs and Options=mode=755"
fi

# Reload services 
systemctl daemon-reload
systemctl enable process-user-data.path
systemctl disable process-user-data.service
systemctl enable media-cidata.mount
systemctl enable reboot-watcher.path
systemctl enable sftp-dir.service

services=(
    api-server-rest.path
    attestation-agent.path
    confidential-data-hub.path
    kata-agent.path
    media-cidata.mount
    process-user-data.path
    reboot-watcher.path
    'run-kata\x2dcontainers.mount'
    sftp-dir.service
)

# Check status of agent-protocol-forwarder
activating_service="agent-protocol-forwarder.service"
systemctl start ${activating_service} || true
state=$(systemctl show -p ActiveState --value "${activating_service}")
if [[ "$state" == "activating" ]]; then
    echo "${activating_service} is still activating (expected)"
else
    echo "${activating_service} is not in desired state (got: ${state})"
    systemctl status "${activating_service}" --no-pager
    exit 1
fi

# Start each service and check status
for svc in "${services[@]}"; do
    echo "Starting $svc..."
    systemctl start "$svc"

    if systemctl is-active --quiet "$svc"; then
        echo "$svc is running"
    else
        echo "$svc failed to start"
        systemctl status "$svc" --no-pager
        exit 1
    fi
done

# Create configuration files
systemd-tmpfiles --create

ip_address=$(hostname -I | awk '{print $1}')
echo "VM is ready for use as a pod VM for BYOM. IP: ${ip_address}" 
