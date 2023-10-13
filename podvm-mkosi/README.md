# Podvm images with mkosi

[mkosi](https://github.com/systemd/mkosi) builds a bootable OS image from scratch. This way, we have full control over every detail of the image, especially over the image format and boot process. On the long run, we will implement fully, bit-by-bit reproducible images with mkosi, and use measured boot and an immutable root FS to ensure the image integrity through remote attestation.

## Building the image

```sh
make
```

### Upload the image to the desired cloud provider

You can upload the image with the tool of your choice, but the recommended way is using [uplosi](https://github.com/edgelesssys/uplosi). Follow the uplosi readme to configure your upload for the desired cloud provider. Then run:

```sh
# Using -i and a imageVersionFile to increment the image version after the upload.
uplosi -i build/system.raw
```

If you want to use the image with libvirt, run the following to convert to qcow2 format:

```sh
qemu-img convert -f raw -O qcow2 build/system.raw build/system.qcow2
```

## Debug image

There is a debug variant of the image that provides a specific configuration to debug things within
the podvm. It has additional packages installed that are commonly needed for debugging.
Further, the image has access through the serial console enabled, you can access it through the portal
of the cloud provider.

```sh
# for building the binaries
make fedora-binaries-builder
make binaries
# for building a debug image
make image-debug
```

Notice that building a debug image will overwrite any previous existing debug or production image.

For using SSH, create a file `resources/authorized_keys` with your SSH public key. Ensure the permissions
are set to `0400` for the `authorized_keys` file. SSH access is only possible for the `root` user.

## Custom image configuration

You can easily place additional files in `resources/binaries-tree` after it has been populated by the
binaries build step. Notice that systemd units need to be enabled in the presets and links in the tree
won't be copied into the image.

If you want to add additional packages to the image, you can define `mkosi.presets/system/mkosi.conf.d/fedora-extra.conf`:

```ini
[Match]
Distribution=fedora

[Content]
Packages=
    cowsay
```

## Limitations

The following limitations apply to these images. Notice that the limitations are intentional to
reduce complexity of configuration and CI and shall not be seen as open to-dos.

- `DISABLE_CLOUD_CONFIG=false` is implied. The mkosi images are currently using
    cloud init/cloud config, and there is no possibility to disable it.
