# Setup instructions

- KVM host with libvirt configured.
- Libvirt network and storage pool created
- A base storage volume created for POD VM

## Creation of base storage volume

- Ubuntu 20.04 VM with minimum 50GB disk and the following packages installed
  - `cloud-image-utils`
  - `qemu-system-x86`

- Install packer on the VM by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)

- Create qcow2 image by executing the following command
	- [setting up authenticated registry support](../docs/registries-authentication.md)
```
cd ../podvm
CLOUD_PROVIDER=libvirt make image
```
- The default image uses an Ubuntu cloud image base. If instead, you would like to create a rhel based podvm image, you can
set PODVM_DISTRO when invoking build as such:
```
export RHEL_IMAGE_URL="" <- either a local path or an url
export RHEL_IMAGE_CHECKSUM="" <- the checksum of the aforementioned image
CLOUD_PROVIDER=libvirt PODVM_DISTRO=rhel make build
```

The image will be available under the `output` directory

- Copy the qcow2 image to the libvirt machine

- Create volume
```
export IMAGE=<full-path-to-qcow2>

virsh vol-create-as --pool default --name podvm-base.qcow2 --capacity 20G --allocation 2G --prealloc-metadata --format qcow2
virsh vol-upload --vol podvm-base.qcow2 $IMAGE --pool default --sparse
```

If you want to set default password for podvm debugging then you can use guestfish to edit the qcow2 and make any suitable changes.

# Running cloud-api-adaptor

Refer to the [cloud-api-adaptor deployment instructions](../install/README.md#deploy-cloud-api-adaptor)
* If your libvirt host is not configured to be passwordless make sure you set ssh-key-secret in the [kustomization file](../install/overlays/libvirt/kustomization.yaml)
