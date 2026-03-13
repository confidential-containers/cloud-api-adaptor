#!/bin/bash

#load drivers
nvidia-ctk -d system create-device-nodes --control-devices --load-kernel-modules

nvidia-persistenced
# set confidential compute to ready state
nvidia-smi conf-compute -srs 1
# Generate NVIDIA CDI configuration
nvidia-ctk cdi generate --output=/var/run/cdi/nvidia.yaml > /var/log/nvidia-cdi-gen.log 2>&1
