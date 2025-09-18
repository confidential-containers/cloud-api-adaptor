# BYOM Provider Configuration

The BYOM (Bring Your Own Machine) provider allows you to use pre-created VMs as
peer pods. This provider communicates with VMs via SSH/SFTP to deliver the
initial configuration (`userdata`)

## Pod VM image Configuration

You must run the following commands from `src/cloud-api-adaptor/podvm-mkosi` as the base directory.

### Create SSH key pair

```bash
ssh-keygen -f ./id_rsa -N ""
```
This will create `id_rsa` and `id_rsa.pub`. You'll need this in the subsequent steps.

Copy `id_rsa.pub` to `./resources/authorized_keys`

```bash
cp id_rsa.pub ./resources/authorized_keys
```

Copy `id_rsa` and `id_rsa.pub` to `../install/overlays/byom`

```bash
cp id_rsa id_rsa.pub ../install/overlays/byom
```

### Build Pod VM image

Create sftp enabled pod VM image by running the following command.

```bash
make sftp
```

The image will be available under `./build` directory.

Create a VM using the generated image and capture it's IP. 

Here is an example to create a libvirt VM using `virt-install` and capture it's IP.

Copy the image to the libvirt image directory.

```bash
cp ./build/podvm-fedora-amd64.qcow2 /var/lib/libvirt/images
```

Create VM.

```bash
virt-install \
   --name podvm-test \
   --boot uefi \
   --memory 2048 \
   --vcpus 1 \
   --import \
   --os-variant fedora40 \
   --network default \
   --disk /var/lib/libvirt/images/podvm-fedora-amd64.qcow2
```

For some environment, you might have to explicitly specify the UEFI bootloader. 

Replace `--boot uefi` with the following:

```bash
--boot loader=<path-to-ovmf> 
```

Example on Ubuntu 24.04

```bash
--boot loader=/usr/share/ovmf/OVMF.fd
```

Capture the VM IP

```bash
virsh domifaddr podvm-test
```

## Deployment Configuration

You must run the following command from `src/cloud-api-adaptor/install/overlays/byom` as the base directory.

### Set pre-create VM IPs in `kustomization.yaml`

Update the IPs as per your environment.

```yaml
- VM_POOL_IPS: <192.168.1.100,192.168.1.101,...>
```

### SSH Host Key Verification

The `byom` provider supports two modes for SSH host key verification:

#### 1. Stateless TOFU (Trust On First Use) - Default

Update the files for `ssh-key-secret` in `kustomization.yaml` to point to the SSH key pair copied in the beginning

```yaml
- name: ssh-key-secret
  namespace: confidential-containers-system
  files: # key generation example: ssh-keygen -f ./id_rsa -N ""
  - id_rsa # set - path to private key
  - id_rsa.pub # set - path to public key
```

#### 2. Host Key Allowlist - Recommended

SSH host keys are **server-side keys** that identify the VM to clients (different from client authentication keys).

You can use the following methods to get the host keys of the pre-created VMs

1. Direct SSH Key Scan

```bash

# Extract individual key types in proper format (without IP prefix)
ssh-keyscan -t rsa 192.168.1.100 | sed 's/^[^ ]* //' > vm1_rsa.pub
ssh-keyscan -t ecdsa 192.168.1.100 | sed 's/^[^ ]* //' > vm1_ecdsa.pub
ssh-keyscan -t ed25519 192.168.1.100 | sed 's/^[^ ]* //' > vm1_ed25519.pub
```

1. Extract from VM Directly

```bash
# Assuming you have SSH access to the VM
ssh root@192.168.1.100 "cat /etc/ssh/ssh_host_rsa_key.pub" > vm1_rsa.pub
ssh root@192.168.1.100 "cat /etc/ssh/ssh_host_ecdsa_key.pub" > vm1_ecdsa.pub
ssh root@192.168.1.100 "cat /etc/ssh/ssh_host_ed25519_key.pub" > vm1_ed25519.pub
```

##### Expected Key Format

Host key files must be in **authorized_keys format** (standard SSH public key format):

**Correct format:**

```bash
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQ... [optional-comment]
ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY... [optional-comment]
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG... [optional-comment]
```

**Incorrect format (known_hosts):**

```bash
192.168.1.100 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQ...
```

Update the following entries in `kustomization.yaml`.

```yaml
configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - SSH_HOST_KEY_ALLOWLIST_DIR="/etc/ssh-allowlist"  # Enable allowlist mode

secretGenerator:
- name: ssh-host-key-allowlist
  namespace: confidential-containers-system
  files:
  - vm1_rsa.pub      # Host keys from VM 1
  - vm1_ecdsa.pub
  - vm1_ed25519.pub
  - vm2_rsa.pub      # Host keys from VM 2
  - vm2_ecdsa.pub
  - vm2_ed25519.pub

##SSH_HOST_KEY_ALLOWLIST
  - ssh_host_key_allowlist_volume_mount.yaml # set (for SSH host key allowlist)
##SSH_HOST_KEY_ALLOWLIST
```

## Deploy

Run the following command to deploy CAA with `byom` provider.
You must run this from `src/cloud-api-adaptor` directory.

```bash
make deploy
```

## Delete teh deployment

```bash
make delete
```
