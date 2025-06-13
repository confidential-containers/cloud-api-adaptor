# Podvm images with mkosi

[mkosi](https://github.com/systemd/mkosi) builds a bootable OS image from scratch. This way, we have full control over every detail of the image, especially over the image format and boot process. On the long run, we will implement fully, bit-by-bit reproducible images with mkosi, and use measured boot and an immutable root FS to ensure the image integrity through remote attestation.

## Prerequisites

There are various ways to install mkosi documented on the [project page](https://github.com/systemd/mkosi). Different distributions also package mkosi in their repositories, alongside the dependencies.

Refer to the [CI workflow](../../../.github/workflows/podvm_mkosi.yaml) to see which additional tools are required to build an image.

### Building the image

```sh
make # this will rebuild the builder, the binaries and the OS image
```

> [!WARNING]
> The `make` (and `make binaries`) targets will remove all customization made
> in the `./resources/binaries-tree` dirs

```sh
make image # this will only rebuild the OS image
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
make debug # this will rebuild the builder, the binaries and the OS image
```

```sh
make image-debug # this will only rebuild the OS image
```

Notice that building a debug image will overwrite any previous existing debug or production image.

For using SSH, create a file `resources/authorized_keys` with your SSH public key. Ensure the permissions
are set to `0400` for the `authorized_keys` file. SSH access is only possible for the `root` user.

## Testing the image

To verify the podvm image is bootable and responsive before deploying to
cloud one can use a simple [smoketest](../podvm/hack/smoke_test.sh). It
uses libvirt to run the image and kata-agent-ctl to establish connection
to the podvm.

> [!WARNING]
> This script is intended for CI testing, expects tools in-place and contains
> only a rough cleanup. Use it on a disposable machine only.
>
> The smoke-test won't deploy tls certificates so the image must be built with
> `-disable-tls`:
>
>     mkdir -p ./resources/binaries-tree/etc/default
>     echo "TLS_OPTIONS=-disable-tls" > ./resources/binaries-tree/etc/default/agent-protocol-forwarder
>
> Do not forget to re-enable it before sending the image to production.

## Custom image configuration

You can easily place additional files in `resources/binaries-tree` after it has been populated by the
`make binaries` step. Notice that systemd units need to be enabled in the presets and links in the tree
won't be copied into the image. You can use `./mkosi.postinst` script to create symlinks.

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

- Deployed images cannot be customized with cloud-init. Runtime configuration data is retrieved
  from IMDS via the project's `process-user-data` tool.

## Build s390x image
We can use the mkosi **ToolsTree** feature defined in `mkosi.conf` to download latest tools automatically:
```
[Host]
ToolsTree=default
```
And install **mkosi** from the repository:
```sh
git clone -b v21 https://github.com/systemd/mkosi
ln -s $PWD/mkosi/bin/mkosi /usr/local/bin/mkosi
mkosi --version
```
Another issue is s390x does not support UEFI. Instead, we can first use **mkosi** to build non-bootable system files, then use **zipl** to generate the bootloader and finally create the bootable image.

It requires a **s390x host** to build s390x image with make commands:
```
TEE_PLATFORM=se-attester make binaries
make image
# SE_BOOT=true make image
# make image-debug
# SE_BOOT=true make image-debug
```

The final output is `build/podvm-s390x.qcow2` or `build/podvm-s390x-se.qcow2`, which can be used as the Pod VM image in libvirt environment.
