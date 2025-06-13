# Setup instructions
## Prerequisites
- golang
- pvsadm tool
- IBM Cloud account

> Note: You can only run the below image build instructions on `ppc64le`(CentOS/RHEL). This would additionally require `qemu-img` to be installed.

#### Intall pvsadm

We will be using the tool `pvsadm` to customise and build the VM image. Download or install it using the below instructions.

1. You can download the binary from Github releases [here](https://github.com/ppc64le-cloud/pvsadm/releases)

2. To install from source
   ```
   cd $GOPATH/src/github.com/
   git clone https://github.com/ppc64le-cloud/pvsadm.git
   cd pvsadm && make build
   ```

3. For MacOS, refer [here](https://github.com/ppc64le-cloud/pvsadm#homebrew)

## Image Build

1. Customise the image preparation
```
pvsadm image qcow2ova --prep-template-default > image-prep.template
```

Add the following snippet to `image-prep.template`
```
yum install -y gcc gcc-c++ git make wget
export PATH=${PATH}:/usr/local/go/bin:${HOME}/.cargo/bin
wget https://github.com/mikefarah/yq/releases/download/v4.42.1/yq_linux_ppc64le
chmod +x yq_linux_ppc64le && mv yq_linux_ppc64le /usr/local/bin/yq
git clone https://github.com/confidential-containers/cloud-api-adaptor.git
cd cloud-api-adaptor/src/cloud-api-adaptor/ibmcloud-powervs/image
make build
```

> NOTE:
> 1. If you intend to use DHCP network type for creating peer pod VMs with
> PowerVS provider, you need to additionally add this to `image-prep.template` and setup a DHCP server.
> ```
> mkdir -p /etc/cloud/cloud.cfg.d
> cat <<EOF >> /etc/cloud/cloud.cfg.d/99-custom-networking.cfg
> network: {config: disabled}
> EOF
> ```
> 2. Setup a DHCP server using pvsadm
> ```
> pvsadm dhcpserver create --workspace-id <workspace-id> -k  <api-key>
> ```
> 3. To use a specific port or address for agent-protocol-forwarder, pass `FORWARDER_PORT=<port-number>` to the `make` command.

2. Download the qcow2 image and converts into ova type
```
pvsadm image qcow2ova --image-name <name> --image-dist centos --image-url https://cloud.centos.org/centos/9-stream/ppc64le/images/CentOS-Stream-GenericCloud-9-latest.ppc64le.qcow2 --prep-template image-prep.template --image-size 20
```


> Qcow2 CentOS images for ppc64le can be found [here](https://cloud.centos.org/centos/9-stream/ppc64le/images/)

## Import image to PowerVS

> You can export IBMCLOUD_API_KEY or pass it as a flag (`-k` or `--api-key`) to the below commads.

1. First, we need to upload the ova image to a COS bucket
```
pvsadm image upload -b <bucket-name> -f <file-name> -r <region> --accesskey <access-key-value> --secretkey <secret-key-value>
```

2. Import the ova image to a PowerVS service instance
```
pvsadm image import -n <service-instance-name> -b <bucket-name> -o <file-name> -r <region> --accesskey <access-key-value> --secretkey <secret-key-value> --pvs-image-name <final-image-name>
```
> The access key and secret key can be fetched from the IBM Cloud UI. For more details, refer [here](https://cloud.ibm.com/docs/cloud-object-storage?topic=cloud-object-storage-service-credentials)

## Running cloud-api-adaptor

1. Setup necessary cloud resources such as PowerVS Service instance, network, API Key etc..

2. Populate an env file with the IBM Cloud API key

   ```bash
   cat <<EOF > install/overlays/ibmcloud-powervs/cloud-credentials.env
   IBMCLOUD_API_KEY=${IBMCLOUD_API_KEY}
   EOF
   ```

3. Update [kustomization.yaml](../install/overlays/ibmcloud-powervs/kustomization.yaml) with the required details

4. Deploy Cloud API Adaptor by following the [install](../install/README.md) guide
