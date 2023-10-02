# Podvm images with mkosi

[mkosi](https://github.com/systemd/mkosi) builds a bootable OS image from scratch. This way, we have full control over every detail of the image, especially over the image format and boot process. On the long run, we will implement fully, bit-by-bit reproducible images with mkosi, and use measured boot and an immutable root FS to ensure the image integrity through remote attestation.

## Building the image

```sh
make
```

### Upload the image to the desired cloud provider

You can upload the image with the tool of your choice, but the recommended way is using [uplosi](https://github.com/edgelesssys/uplosi). Follow the uplosi readme to configure your upload for the desired cloud provider. Then run:

```sh
# Using -i to increment the image version after the upload.
uplosi -i build/system.raw
```

If you want to use the image with libvirt, run the following to convert to qcow2 format:

```sh
qemu-img convert -f raw -O qcow2 build/system.raw build/system.qcow2
```

## Custom image configuration

You can easily place additional files in `resources/binaries-tree` after it has been populated by the
binaries build step. Notice that systemd units need to be enabled in the presets and links in the tree
won't be copied into the image.

## Limitations

The following limitations apply to these images. Notice that the limitations are intentional to
reduce complexity of configuration and CI and shall not be seen as open to-dos.

- `DISABLE_CLOUD_CONFIG=false` is implied. The mkosi images are currently using
    cloud init/cloud config, and there is no possibility to disable it.
