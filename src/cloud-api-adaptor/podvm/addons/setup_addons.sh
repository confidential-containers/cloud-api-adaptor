#!/bin/bash

set -euo pipefail

# This is the dir in the pod vm image during build
ADDONS_DIR="/tmp/addons"

# Check environment variables and execute corresponding scripts
if [[ "${ENABLE_NVIDIA_GPU}" == "yes" ]]; then
	echo "Setting up Nvidia GPU"
        ${ADDONS_DIR}/nvidia_gpu/setup.sh
fi
