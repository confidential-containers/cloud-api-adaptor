#!/bin/bash

# Two options: install - to check and install some prerequisites
#              uninstall - to uninstall the prerequisites

# GLOBAL VARIABLES
# Install directory for binary packages
install_dir="/usr/local/bin"
# File to record installed OS packages
os_packages_file=".installed_os_packages"
# File to record installed binary packages
bin_packages_file=".installed_bin_packages"
# File to record docker installation
docker_file=".installed_docker"
# File to record go installation
go_file=".installed_go"

# Get the directory where the script is located
SCRIPT_DIR=$(dirname "$(realpath "$0")")

# Determine the path to versions.yaml relative to the script's directory
VERSIONS_YAML_PATH=$(realpath "${SCRIPT_DIR}/../versions.yaml")

# function to trap errors and exit
function error_exit() {
    echo "$1" 1>&2
    exit 1
}

# function to check if passwordless sudo is enabled
check_sudo() {
    if sudo -n true 2>/dev/null; then
        echo "Passwordless sudo is enabled."
    else
        error_exit "Passwordless sudo is not enabled. Please enable passwordless sudo."
    fi
}

# function to compare versions
# returns true if $1 < $2
version_lt() {
    [ "$1" != "$2" ] && [ "$(printf '%s\n' "$@" | sort -V | head -n 1)" == "$1" ]
}

# function to install OS packages
# the packages are available in the variable REQUIRED_OS_PACKAGES
# the function will install the packages using the package manager
# Following are the packages that are installed:
# make
install_os_packages() {
    # Define the required OS packages
    REQUIRED_OS_PACKAGES=(
        "make"
    )

    # Install the required OS packages
    for package in "${REQUIRED_OS_PACKAGES[@]}"; do
        if [[ -x "$(command -v "${package}")" ]]; then
            echo "Package ${package} is already installed. Skipping."
            continue
        else
            echo "Installing ${package}..."
            # Update a hidden file to record what was installed. This is useful for uninstallation
            echo "${package}" >>"${os_packages_file}"
            if [[ -x "$(command -v apt-get)" ]]; then
                # shellcheck disable=SC2015
                sudo apt-get update &&
                    sudo apt-get install -y "${package}" ||
                    error_exit "Failed to install ${package}"
            elif [[ -x "$(command -v dnf)" ]]; then
                sudo dnf install -y "${package}" ||
                    error_exit "Failed to install ${package}"
            else
                error_exit "Unsupported OS"
            fi
        fi
    done

    echo "All OS packages installed successfully."

}

# function to download and install binary packages.
# the packages, their respective download locations and compression
# are available in the variable REQUIRED_BINARY_PACKAGES
# the function will download the packages, extract them and install them in /usr/local/bin
# Following are the packages that are installed:
# yq=https://github.com/mikefarah/yq/releases/download/v4.44.2/yq_linux_amd64
# kubectl=https://storage.googleapis.com/kubernetes-release/release/v1.29.4/bin/linux/amd64/kubectl
# kind=https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64

install_binary_packages() {
    # Define the required binary packages
    REQUIRED_BINARY_PACKAGES=(
        "yq=https://github.com/mikefarah/yq/releases/download/v4.44.2/yq_linux_amd64"
        "kubectl=https://storage.googleapis.com/kubernetes-release/release/v1.29.4/bin/linux/amd64/kubectl"
        "kind=https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64"
        "helm=https://get.helm.sh/helm-v4.0.4-linux-amd64.tar.gz"
    )

    # Specify the installation directory
    local install_dir="/usr/local/bin"

    # Install the required binary packages
    for package_info in "${REQUIRED_BINARY_PACKAGES[@]}"; do
        IFS='=' read -r package_name package_url <<<"${package_info}"
        download_path="/tmp/${package_name}"

        if [[ -x "${install_dir}/${package_name}" ]]; then
            echo "Package ${package_name} is already installed. Skipping."
            continue
        else
            echo "Downloading ${package_name}..."
            # Update a hidden file to record what was installed. This is useful for uninstallation
            echo "${package_name}" >>"${bin_packages_file}"
            curl -sSL "${package_url}" -o "${download_path}" ||
                error_exit "Failed to download ${package_name}"

            echo "Extracting ${package_name}..."
            if [[ "${package_url}" == *.tar.gz ]]; then
                sudo tar -xf "${download_path}" -C "${install_dir}" ||
                    error_exit "Failed to extract ${package_name}"
                if [ "${package_name}" = "helm" ]; then
                    sudo mv "${install_dir}/linux-amd64/helm" "${install_dir}"
                    sudo rm -rf "${install_dir}/linux-amd64"
                fi
            # If not a tar.gz file, then it is a binary file
            else
                echo "${package_name} is binary file."
                sudo mv "${download_path}" "${install_dir}/${package_name}" ||
                    error_exit "Failed to move ${package_name} to ${install_dir}"
            fi

            echo "Marking  ${install_dir}/${package_name} executable"
            sudo chmod +x "${install_dir}/${package_name}" ||
                error_exit "Failed to mark ${package_name} executable"

            echo "Cleaning up..."
            rm -f "${download_path}"
        fi
    done

    echo "All binary packages installed successfully."

}

# function to install golang
install_golang() {
    echo "Path to versions.yaml: $VERSIONS_YAML_PATH"

    # When installing, use the required min go version from the versions.yaml file
    REQUIRED_GO_VERSION="$(yq '.tools.golang' "$VERSIONS_YAML_PATH")"
    # Check if Go is already installed
    if [[ -x "$(command -v go)" ]]; then
        echo "Go is already installed"
        # Check if the installed version is less than the required version
        installed_go_version=$(v=$(go version | awk '{print $3}') && echo "${v#go}")

        if version_lt "$installed_go_version" "$REQUIRED_GO_VERSION"; then
            echo "Warning: Found ${installed_go_version}, is lower than our required $REQUIRED_GO_VERSION"
            echo "Please remove the existing go version and run this script again."
            exit 1
        else
            echo "Found ${installed_go_version}, good to go"
        fi
    else
        # Install Go
        echo "Installing Go"
        curl -fsSL https://go.dev/dl/go${REQUIRED_GO_VERSION}.linux-amd64.tar.gz -o go.tar.gz || error_exit "Failed to download Go"
        sudo tar -C /usr/local -xzf go.tar.gz || error_exit "Failed to extract Go"
        touch "$go_file"
        rm -f go.tar.gz
    fi

}

# function to uninstall golang
uninstall_golang() {
    # Check if Go is installed
    if [[ ! -x "$(command -v go)" ]]; then
        echo "Go is not installed"
        return
    fi

    # Uninstall Go
    echo "Uninstalling Go"

    # Uninstall only if go_file exists
    if [[ ! -f "$go_file" ]]; then
        echo "Go was not installed using this script. Skipping uninstallation."
        return
    fi

    sudo rm -rf /usr/local/go

    # Remove the file that records the installed packages
    rm -f "$go_file"
}

# function to install docker
install_docker() {
    # Check if Docker is already installed
    if [[ -x "$(command -v docker)" ]]; then
        echo "Docker is already installed"
    else
        # Install Docker
        echo "Installing Docker"
        curl -fsSL https://get.docker.com -o get-docker.sh || error_exit "Failed to download Docker installation script"
        sudo sh get-docker.sh || error_exit "Failed to install Docker"
        touch "$docker_file"
        rm -f get-docker.sh
        sudo groupadd docker
        sudo usermod -aG docker "$USER"
    fi
}

#function to uninstall the binary packages
uninstall_binary_packages() {
    # Remove the installed binary packages
    # The packages are available under bin_packages_file file
    # The function will remove the packages from install_dir

    # Check if the file exists
    if [[ ! -f "${bin_packages_file}" ]]; then
        echo "No binary packages to uninstall."
        return
    fi

    # Read the file and uninstall the packages
    while IFS= read -r package_name; do
        if [[ -x "${install_dir}/${package_name}" ]]; then
            echo "Uninstalling ${package_name}..."
            sudo rm -f "${install_dir}/${package_name}" ||
                error_exit "Failed to uninstall ${package_name}"

        else
            echo "Package ${package_name} is not installed. Skipping."
        fi
    done <"${bin_packages_file}"

    # Remove the file that records the installed packages
    rm -f "${bin_packages_file}"

    echo "All binary packages uninstalled successfully."
}

# function to uninstall the OS packages
uninstall_os_packages() {
    # Remove the installed Os packages
    # The packages are available under os_packages_file file
    # The function will remove the packages using the package manager

    # Check if the file exists
    if [[ ! -f "${os_packages_file}" ]]; then
        echo "No OS packages to uninstall."
        return
    fi

    # Read the file and uninstall the packages
    while IFS= read -r package_name; do
        if [[ -x "$(command -v "${package_name}")" ]]; then
            echo "Uninstalling ${package_name}..."
            if [[ -x "$(command -v apt-get)" ]]; then
                sudo apt-get purge -y "${package_name}" ||
                    error_exit "Failed to uninstall ${package_name}"
            elif [[ -x "$(command -v dnf)" ]]; then
                sudo dnf remove -y "${package_name}" ||
                    error_exit "Failed to uninstall ${package_name}"
            else
                error_exit "Unsupported OS"
            fi
        else
            echo "Package ${package_name} is not installed. Skipping."
        fi
    done <"${os_packages_file}"

    # Remove the file that records the installed packages
    rm -f "${os_packages_file}"

    echo "All OS packages uninstalled successfully."
}

# function to uninstall docker
uninstall_docker() {
    # Check if Docker is installed
    if [[ ! -x "$(command -v docker)" ]]; then
        echo "Docker is not installed"
        return
    fi

    # Uninstall Docker
    echo "Uninstalling Docker"

    # Uninstall only if docker_file exists
    if [[ ! -f "$docker_file" ]]; then
        echo "Docker was not installed using this script. Skipping uninstallation."
        return
    fi
    # Check if OS is Ubuntu
    if [[ -x "$(command -v apt-get)" ]]; then
        sudo apt-get purge -y docker-ce docker-ce-cli containerd.io \
            docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras || error_exit "Failed to uninstall Docker"
        sudo rm -rf /var/lib/docker
        sudo rm -rf /var/lib/containerd
    elif [[ -x "$(command -v dnf)" ]]; then
        sudo dnf remove -y docker-ce docker-ce-cli containerd.io \
            docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras || error_exit "Failed to uninstall Docker"
        sudo rm -rf /var/lib/docker
        sudo rm -rf /var/lib/containerd
    fi

    # Remove the file that records the installed packages
    rm -f "$docker_file"
}

#function to set PATH to include /usr/local/bin and /usr/local/go/bin
set_path() {

    # If bin_packages_file exists, then add /usr/local/bin to PATH
    if [[ -f "${bin_packages_file}" ]]; then
        # Add /usr/local/bin to PATH
        if [[ ":$PATH:" == *":/usr/local/bin:"* ]]; then
            echo "/usr/local/bin is already in PATH. Skipping."
        else
            echo "Adding /usr/local/bin to PATH..."
            # shellcheck disable=SC2016
            echo 'export PATH=$PATH:/usr/local/bin' >>"$HOME"/.bashrc
        fi
    fi

    # If go_file exists, then add /usr/local/go/bin to PATH
    if [[ -f "${go_file}" ]]; then
        # Add /usr/local/go/bin to PATH
        if [[ ":$PATH:" == *":/usr/local/go/bin:"* ]]; then
            echo "/usr/local/go/bin is already in PATH. Skipping."
        else
            echo "Adding /usr/local/go/bin to PATH..."
            # shellcheck disable=SC2016
            echo 'export PATH=$PATH:/usr/local/go/bin' >>"$HOME"/.bashrc
        fi
    fi

    # Reload the bashrc file
    # shellcheck source=/dev/null
    source "$HOME"/.bashrc

}

if [[ "$(uname)" != "Linux" || "$(uname -m)" != "x86_64" ]]; then
    error_exit "This script is only for Linux x86_64 OS."
fi

# Main function
if [[ "$1" == "install" ]]; then
    check_sudo
    install_os_packages
    install_binary_packages
    install_golang
    install_docker
    set_path
elif [[ "$1" == "uninstall" ]]; then
    check_sudo
    uninstall_binary_packages
    uninstall_os_packages
    uninstall_golang
    uninstall_docker
else
    error_exit "Invalid argument. Please provide either 'install' or 'uninstall'."
fi
