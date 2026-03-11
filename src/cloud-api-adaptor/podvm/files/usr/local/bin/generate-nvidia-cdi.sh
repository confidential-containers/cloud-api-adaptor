#!/bin/bash

# Create device nodes. Kernel modules (nvidia, nvidia-uvm, nvidia-modeset) are
# loaded by systemd-modules-load.service from /usr/lib/modules-load.d/nvidia.conf.
# Do not pass --load-kernel-modules here to avoid racing with that unit.
# nvidia-persistenced is managed by nvidia-persistenced.service; this unit
# depends on it via nvidia-cdi.service, so do not start it inline here.
nvidia-ctk -d system create-device-nodes --control-devices

# Set confidential compute to ready state
nvidia-smi conf-compute -srs 1

# Generate NVIDIA CDI specification
nvidia-ctk cdi generate --output=/var/run/cdi/nvidia.yaml > /var/log/nvidia-cdi-gen.log 2>&1
