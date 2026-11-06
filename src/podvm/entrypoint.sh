#!/bin/bash

# Adapted from https://github.com/kubernetes-sigs/kind/blob/main/images/base/files/usr/local/bin/entrypoint

set -o errexit
set -o nounset
set -o pipefail


set_machine_id() {
  # Deletes the machine-id embedded in the podvm image and generates a new one.
  echo "clearing and regenerating /etc/machine-id"
  rm -f /etc/machine-id
  systemd-machine-id-setup
}


set_product_uuid() {
  # The system UUID is usually read from DMI via sysfs, Fake it so that 
  # each podvm(container) have a different uuid
  mkdir -p /podvm
  [[ ! -f /podvm/product_uuid ]] && cat /proc/sys/kernel/random/uuid > /podvm/product_uuid
  if [[ -f /sys/class/dmi/id/product_uuid ]]; then
    echo  "faking /sys/class/dmi/id/product_uuid to be random"
    mount -o ro,bind /podvm/product_uuid /sys/class/dmi/id/product_uuid
  fi
  if [[ -f /sys/devices/virtual/dmi/id/product_uuid ]]; then
    echo "faking /sys/devices/virtual/dmi/id/product_uuid as well"
    mount -o ro,bind /podvm/product_uuid /sys/devices/virtual/dmi/id/product_uuid
  fi
}


set_machine_id
set_product_uuid

# we want systemd to be PID1, so exec to it
echo "starting init"
exec "$@"
