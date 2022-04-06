# Setup instructions

- KVM host with libvirt configured.
- Libvirt network and storage pool created
- A base storage volume created for POD VM

## Creation of base storage volume

- Create qcow2 image by executing the following command
```
cd ../ibmcloud/image
make build
```

- Copy the qcow2 image to the libvirt machine

- Create volume
```
export IMAGE=<full-path-to-qcow2>

virsh vol-create-as --pool default --name podvm-base.qcow2 --capacity 107374182400 --allocation 2361393152 --prealloc-metadata --format qcow2
virsh vol-upload --vol podvm-base.qcow2 $IMAGE --pool default --sparse
```

If you want to set default password for debugging then you can use guestfish to edit the qcow2 and make any suitable changes.

# Running cloud-api-adaptor

```
./cloud-api-adaptor libvirt \
     -host-interface ens3 \
     -network-name default  \
     -pool-name default \
     -uri "qemu+ssh://root@192.168.122.1/system" \
     -data-dir "/var/lib/libvirt" 

```

