# Setup instructions

## Prerequisites:

- A configured vCenter instance
- datacenter, datastore and cluster are required fields

## Creating the vsphere template:
- Install packer on your system by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
- Create the vsphere template
	- [setting up authenticated registry support](../docs/registries-authentication.md)
```
cd vsphere/image && make
```

For a rhel podvm image, try:
```
cd vsphere/image && PODVM_DISTRO=rhel make
```

If successful, you can find the image on your vcenter deployment

## File structure
- **vcenter.auto.pkrvars.hcl** - The main template.
- **settings.auto.pkrvars.hcl** - Guest settings referenced by the template file
- **vcenter.auto.pkrvars.hcl** - Your vCenter config

If settings.auto.pkrvars.hcl and vcenter.auto.pkrvars.hcl are not present, they will be created
when you first run make. You can also choose to create these files if you prefer.

Please take a look at variables.pkr.hcl for all the variables supported.

Here are a few that you might find useful to configure the guest:
- *vm_cpu_count*
  This field configures the number of vcpus for the guest.
- *vm_guest_size*
  Determines the amount of guest memory.
- *vm_disk_size*
  The size of the virtual hard disk. Please note that installation might fail if the disk size is too small.
- *vm_network_name*
  The virtualized network adapter to use. vmxnet3 is the default.

## Potential issues
The start of the installation uses automated keyboard input. Timing issues may prevent entering
the shell. Please try experimenting with vm_boot_wait if you encounter this problem.
