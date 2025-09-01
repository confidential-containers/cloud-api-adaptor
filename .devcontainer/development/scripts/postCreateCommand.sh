#!/usr/bin/env bash
DEBIAN_FRONTEND=noninteractive sudo apt-get update -qq && sudo apt-get install -yq qemu-utils

mkdir -p  ~/.local/scripts/
cp "${CONTAINER_WORKSPACE_FOLDER}/.devcontainer/development/scripts/general_greeting.sh" ~/.local/scripts/ || exit
echo "source ~/.local/scripts/general_greeting.sh" >> ~/.bashrc


YQ_VERSION=$(awk -F'= *' '/^YQ_VERSION/ {print $2}' ${CONTAINER_WORKSPACE_FOLDER}/src/cloud-api-adaptor/Makefile.defaults)


# Detect raw values
RAW_ARCH=$(uname -m)
RAW_OS=$(uname -s)

# Normalize OS
case "$RAW_OS" in
    Linux*)     DISTRO_OS="linux" ;;
    Darwin*)    DISTRO_OS="darwin" ;;
    FreeBSD*)   DISTRO_OS="freebsd" ;;
    CYGWIN*|MINGW*|MSYS*) DISTRO_OS="windows" ;;
    *)          DISTRO_OS="unknown" ;;
esac

# Normalize architecture
case "$RAW_ARCH" in
    x86_64)   DISTRO_ARCH="amd64" ;;
    aarch64)  DISTRO_ARCH="arm64" ;;
    armv7l)   DISTRO_ARCH="arm/v7" ;;
    armv6l)   DISTRO_ARCH="arm/v6" ;;
    i386|i686) DISTRO_ARCH="386" ;;
    s390x)    DISTRO_ARCH="s390x" ;;
    *)        DISTRO_ARCH="$RAW_ARCH" ;;  # fallback to raw
esac

# Output results (optional)
echo "DISTRO_OS=$DISTRO_OS"
echo "DISTRO_ARCH=$DISTRO_ARCH"

# Build URL
YQ_DOWNLOAD_URL="https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_${DISTRO_OS}_${DISTRO_ARCH}"

echo "Downloading yq version ${YQ_VERSION} for ${DISTRO_OS}/${DISTRO_ARCH} from:"
echo "${YQ_DOWNLOAD_URL}"

sudo wget -qO /usr/bin/yq "${YQ_DOWNLOAD_URL}"
sudo chmod +x /usr/bin/yq

go install github.com/edgelesssys/uplosi@latest
