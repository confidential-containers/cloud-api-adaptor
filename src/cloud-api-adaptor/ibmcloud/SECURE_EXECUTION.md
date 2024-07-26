# Secure Execution Support for Peer Pods on IBM Cloud
This guide describes how to set up a demo environment on IBM Cloud for Secure Execution enabled peer pod VMs using the operator deployment approach.

IBM Cloud VPC already supports to create Hyper Protect Virtual Server on LinuxONE. And Confidential Computing is enabled by using the [IBM Secure Execution](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-introduction) for Linux technology. For more information, please refer to [Confidential computing with LinuxONE](https://cloud.ibm.com/docs/vpc?topic=vpc-about-se)

To support Secure Execution, we need build a Secure Execution enabled peer pod VM image, to replace the non-SE peer pod VM image.

> **Note**: In [the document](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-secure-workload) describe the details about execution secure workload, you can go through the document to get a simplified view about how your workload is protected.

## Set up a demo environment without Secure Execution on your development machine

Follow the [README.md](./README.md) to set up a demo environment on IBM Cloud for peer pod VMs using the operator deployment approach.

## Build and upload a Secure Execution enabled peer pod VM image

#### Build the Secure Execution enabled peer pod VM image
The general approach to built peer pod VM images is documented [here](../podvm/README.md), but the specific way to create an SE peer pod VM image, are to run the following instructions under `cloud-api-adaptor/podvm` folder:

1. Build `podvm_builder` image:
```bash
$ docker build -t podvm_builder \
         -f Dockerfile.podvm_builder .
```

2. Build `podvm_binaries_s390x` image:
```bash
$ docker build -t podvm_binaries_s390x \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg ARCH=s390x \
         -f Dockerfile.podvm_binaries .
```

3. Download host key document from Resource Link

The host key must match the host system for which the podvm image is prepared. You can download a host key document from Resource Link.

As a registered user, access the search page:
[Host key document search](https://www.ibm.com/servers/resourcelink/hom03010.nsf/pages/HKDSearch?OpenDocument)

If you have never signed in to Resource Link, you need to register before you can access the host key document search page. Please refer to document [Obtaining a host key document from Resource Link](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-obtain-host-key-document#lxse_obtain_hkd) for details.

Need input the **machine type** and **machine serial number**, which can be obtained from `/proc/sysinfo` of the s390x worker node. Check the values of **Type** and **Sequence Code**. Machine serial number would be the last 5 or 7 characters. Please refer to [this documentation](https://www.ibm.com/docs/en/linux-on-systems?topic=tasks-find-machine-serial) for details.

> From the root directory of the `cloud-api-adaptor repository`, run the following steps:
> ```bash
> pushd ibmcloud/cluster
> export REGION=$(terraform output --raw region)
> popd
> export IBMCLOUD_API_KEY="<your api key>"
> ibmcloud login -r ${REGION} -apikey ${IBMCLOUD_API_KEY}
> ibmcloud is floating-ips | grep node-1
> ssh root@<floating ip>
> cat /proc/sysinfo
> Manufacturer:         IBM             
> Type:                 8562
> LIC Identifier:       401e26ff62dc9b82
> Model:                701              A01             
> Sequence Code:        0000000000012345
> ...
> ```
> Here 8562 is the machine type, 12345 is the serial number.

When you obtain the host key document, please copy the downloaded host key document `HKD-<type>-<serial>.crt` to `podvm/files` directory.

> **Note**
> - You can download multiple different host keys and prepare only one SE enabled image for different host systems.
>
> eg. If you want to deploy the SE enabled image on two host systems which the machine type are `8562` the machine serial number are `1234567` and `7654321` then the `files` directory tree looks like as follow:
> ```bash
> $ tree -L 1 files
> files
> ├── HKD-8562-1234567.crt
> ├── HKD-8562-7654321.crt
> ├── etc
> └── usr
> ```
> - Get the **machine type** and **machine serial number** from worker node only works when there is only one s390x arch host system in the target zone, if there are multiple s390x arch host systems in the target zone, please try to create z vsis on the target s390x arch host system one by one then get the **machine type** and **machine serial number** from the created z vsis on diferent host systems
> - Another way maybe you can contact the system admin to get the **machine type** and **machine serial number** for target host systems

4. Build `se_podvm_s390x` image:
```bash
$ docker build -t se_podvm_s390x \
         --build-arg ARCH=s390x \
         --build-arg SE_BOOT=true \
         --build-arg BUILDER_IMG=podvm_builder \
         --build-arg BINARIES_IMG=podvm_binaries_s390x \
         --build-arg UBUNTU_IMAGE_URL="" \
         --build-arg UBUNTU_IMAGE_CHECKSUM="" \
         -f Dockerfile.podvm .
```
> **Note**
> - You must passing the `SE_BOOT=true`, `ARCH=s390x`,`UBUNTU_IMAGE_URL=""` and `UBUNTU_IMAGE_CHECKSUM=""` build arguments to docker.
> - Make sure passing the `BINARIES_IMG=podvm_binaries_s390x` build argument to docker, `podvm_binaries_s390x` is the image from the previous step.

#### Upload the Secure Execution enabled peer pod VM image to IBM Cloud
You can follow the process [documented](./IMPORT_PODVM_TO_VPC.md) to extract and upload
the Secure Execution enabled peer pod image you've just built to IBM Cloud as a custom image, noting to replace the
`quay.io/confidential-containers/podvm-ibmcloud-ubuntu-s390x` reference with the local container image that you built
above e.g. `se_podvm_s390x:latest`, and run the script with `--os hyper-protect-1-0-s390x`.

The sample command, assume your working dir is `cloud-api-adaptor/ibmcloud/image`:
```bash
$ ./import.sh se_podvm_s390x:latest eu-gb --instance se-cos-instance --bucket se-podvm-image-cos-bucket --region jp-tok --os hyper-protect-1-0-s390x
```

This script will end with the line: `Image <image-name> with id <image-id> is available`. The `image-id` field will be
needed in the kustomize step later.

## Update the `cloud-api-adaptor` pod
1. Delete previous deployed `cloud-api-adaptor`
```bash
$ kubectl delete -k install/overlays/ibmcloud
```
2. Update the ibmcloud kustomize file to protect your workload with Secure Execution
```bash
export PODVM_IMAGE_ID="<the id of the Secure Execution enabled image uploaded to IBM Cloud earlier>"
export INSTANCE_PROFILE_NAME="bz2e-2x8"
export target_file_path="install/overlays/ibmcloud/kustomization.yaml"
sed -i "s%IBMCLOUD_PODVM_IMAGE_ID=.*%IBMCLOUD_PODVM_IMAGE_ID="${PODVM_IMAGE_ID}"%" ${target_file_path}
sed -i "s%IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME=.*%IBMCLOUD_PODVM_INSTANCE_PROFILE_NAME="${INSTANCE_PROFILE_NAME}"%" ${target_file_path}
```
3. Deploy the `cloud-api-adaptor` pod
```bash
$ kubectl apply -k install/overlays/ibmcloud
```
4 Wait until all the pods are running with:
```bash
$ kubectl get pods -n confidential-containers-system --watch
```

## Test

You can follow guide [Validating the set-up](./README.md#validating-the-set-up) to deploy a nginx pod.
- Verify the Peer Pod VM is created with the Secure Execution enabled custom image.
```bash
$ ibmcloud is ins
Listing instances in all resource groups and region eu-gb under account DaLi Liu's Account as user liudali@cn.ibm.com...
ID                                          Name                   Status    Reserved IP    Floating IP       Profile    Image                                VPC                     Zone      Resource group
0797_57631ed3-5c64-4a26-ba11-3a78d69269df   podvm-nginx-1e6a5060   running   10.242.64.24   -                 bz2e-2x8   se-podvm-2413ed0-dirty-s390x         pp-s390x-image-se-vpc   eu-gb-2   default
```
> **Note**
> - `Profile` should be `bz2e-2x8`.
> - `Image` should be match `<image-name>` from the previous step.
- Verify the Secure Execution feature is enabled inside the container.
```bash
$ kubectl exec nginx -- grep facilities /proc/cpuinfo | grep 158
```
> **Note** If the command displays any output, the CPU is compatible with Secure Execution.
- Verify the kernel includes support for Secure Execution inside the container
```bash
$ kubectl exec nginx -- cat /sys/firmware/uv/prot_virt_guest
```
> **Note** If the command output is `1`, the kernel supports Secure Execution.
