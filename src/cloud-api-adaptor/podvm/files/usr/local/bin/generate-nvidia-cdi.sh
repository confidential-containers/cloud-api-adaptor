#!/bin/bash

# Create device nodes. Modules are already loaded by systemd-modules-load.service
# reading /usr/lib/modules-load.d/nvidia.conf — do not pass --load-kernel-modules
# here as that would race with the modules-load unit. nvidia-persistenced is managed
# by its own systemd unit (nvidia-persistenced.service) which this service depends on
# via nvidia-cdi.service; do not start it inline here.
nvidia-ctk -d system create-device-nodes --control-devices

# Set confidential compute to ready state
nvidia-smi conf-compute -srs 1

# Generate NVIDIA CDI specification
nvidia-ctk cdi generate --output=/var/run/cdi/nvidia.yaml > /var/log/nvidia-cdi-gen.log 2>&1
