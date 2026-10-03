# PodVM tests

Those tests are meant to test basic PodVM business logic outside of a
Kubernetes and CAA deployment, aiming to reduce the debug cycle for
PodVM development.

## x86_64

The tests boot an existing x86_64 debug podvm image with QEMU and
control it over AF_VSOCK. They require QEMU, OVMF, KVM, and vhost-vsock.
Guest commands use SSH over AF_VSOCK. QEMU forwards the
agent-protocol-forwarder port to a dynamically selected host loopback port
for ttrpc tests.

Set `TEST_PODVM_IMAGE`, consistently with the e2e tests:

```sh
TEST_PODVM_IMAGE=podvm/build/podvm-ubuntu-amd64.qcow2 make test-podvm
```

Repeat a test to investigate intermittent failures:

```sh
TEST_PODVM_IMAGE=podvm/build/podvm-ubuntu-amd64.qcow2 \
TEST_PODVM_COUNT=20 TEST_PODVM_TIMEOUT=30m \
RUN_TESTS=TestPodVM/SystemdServices/AttestationAgent make test-podvm
```

The test injects an ephemeral SSH key through a systemd VM credential. Hence
the image does not need a preconfigured `authorized_keys` file or otherwise
modified podvm image.

It _does_ require a debug podvm image, because we probe the guest with
ssh-over-vsock. A test suite that will only exercise the agent protocol
can use a release image.

Tests can provide a `cloudinit.CloudConfig`, which is written to a config
config drive and attached to the guest.

Optional settings:

- `TEST_PODVM_BOOT_TIMEOUT` changes the two-minute guest boot timeout.
- `TEST_PODVM_GUEST_CID` changes the default AF_VSOCK CID of 3.
- `TEST_PODVM_CONTAINER_IMAGE` overrides the guest-pulled container image.
- `TEST_PODVM_QEMU_BINARY` selects a different QEMU executable.
- `TEST_PODVM_OVMF_CODE` and `TEST_PODVM_OVMF_VARS` override OVMF discovery.
