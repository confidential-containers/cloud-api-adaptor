# Secure Execution Support for Peer Pods on IBM Cloud VPC

IBM Cloud VPC already supports to create Hyper Protect Virtual Server on LinuxONE. And Confidential Computing is enabled by using the [IBM Secure Execution](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-introduction) for Linux technology. For more information, please refer to [Confidential computing with LinuxONE](https://cloud.ibm.com/docs/vpc?topic=vpc-about-se)

To support Secure Execution for Peer Pods on IBM Cloud VPC, we need build a Secure Execution enabled custom VM image for pod VMs, to replace the non-SE custom image. This document describes how to build a SE enabled custom image, based on the existing [image build script](./image/build.sh) for IBM Cloud VPC.

> **Note**: In [the document](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-secure-workload) describe the details about execution secure workload, you can go through the document to get a simplified view about how your workload is protected.

## Set up a demo environment without Secure Execution on your development machine

Follow the [README.md](./README.md) to setup a demo environment on IBM Cloud VPC. We need create a **s390x** architecture based cluster, so please include the following two lines in the `terraform.tfvars` file:
```
instance_profile_name = "bz2-2x8"
image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"
```

## Prepare to build SE enabled image

1. **Build genprotimg on worker node**

`genprotimg` is used to build an encrypted boot record from a given kernel, initial RAM disk, parameters, and public host-key document.

The worker node is using Ubuntu 18.04, so we need build genprotimg from the source on GitHub:
```bash
ssh root@ip-of-your-worker-node
git clone https://github.com/ibm-s390-linux/s390-tools.git
cd s390-tools/genprotimg/
apt install -y libglib2.0-dev libssl-dev libcurl4-openssl-dev
make install
```

2. **Download host key document from Resource Link on your development machine**

The host key must match the host system for which the image is prepared. You can download a host key document from Resource Link.

As a registered user, access the search page:
```
https://www.ibm.com/servers/resourcelink/hom03010.nsf/pages/HKDSearch?OpenDocument
```

If you have never signed in to Resource Link, you need to register before you can access the host key document search page. Please refer to document [Obtaining a host key document from Resource Link](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-obtain-host-key-document#lxse_obtain_hkd) for details.

Need input the **machine type** and **machine serial number**, which can be obtained from `/proc/sysinfo` of the s390x worker node. Check the values of **Type** and **Sequence Code**. Machine serial number would be the last 5 or 7 characters. Please refer to [this documentation](https://www.ibm.com/docs/en/linux-on-systems?topic=tasks-find-machine-serial) for details.

When you obtain the host key document, please copy the downloaded host key document `HKD-<type>-<serial>.crt` to the worker node host keys directory, the directory can be any, if you use `/root/hostkeys/` as the host keys directory, you can use follow commands:
```bash
ssh root@ip-of-peer-pod-worker-node mkdir /root/hostkeys/
scp HKD-<type>-<serial>.crt root@ip-of-peer-pod-worker-node:/root/hostkeys/
```

> **Note**
> - You can download multiple different host keys and prepare only one SE enabled image for different host systems.
>
> eg. If you use `/root/hostkeys/` as the hosy keys directory and you want to deploy the SE enabled image on two host systems which the machine type are `8562` the machine serial number are `1234567` and `7654321` then the hosy keys directory tree looks like as follow:
> ```bash
> ssh root@ip-of-peer-pod-worker-node tree /root/hostkeys/
> ...
> /root/hostkeys/
> ├── HKD-8562-1234567.crt
> └── HKD-8562-7654321.crt
> 
> 0 directories, 2 files
> ```
> - Get the **machine type** and **machine serial number** from worker node only works when there is only one s390x arch host system in the target zone, if there are multiple s390x arch host systems in the target zone, please try to create z vsis on the target s390x arch host system one by one then get the **machine type** and **machine serial number** from the created z vsis on diferent host systems
> - Another way maybe you can contact the system admin to get the **machine type** and **machine serial number** for target host systems

## Build one SE enabled custom image

Please confirm the IBM Cloud API key, COS instance name and COS bucket name, VPC name, subnet name, region and zone when you set up the demo environment, set `SE_BOOT` as `1`, set `HOST_KEYS_DIR` with the host key documents directory (eg. `/root/hostkeys/`), set `IMAGE_NAME` as `se-<image_name>-s390x` (eg. `se-podvm-s390x`) then run follow commands:
```
export IBMCLOUD_API_KEY=<your_api_key>
export IBMCLOUD_API_ENDPOINT="https://cloud.ibm.com"
export IBMCLOUD_COS_REGION=<the region your IKS cluster and zVSI is running on>
export IBMCLOUD_COS_SERVICE_INSTANCE=<your_cos_service_instance_name>
export IBMCLOUD_COS_BUCKET=<your_cos_bucket_name>
export IBMCLOUD_COS_SERVICE_ENDPOINT="https://s3.${IBMCLOUD_COS_REGION}.cloud-object-storage.appdomain.cloud"
export IBMCLOUD_VPC_NAME=<your_VPC_name>
export IBMCLOUD_VPC_SUBNET_NAME=<your_subnet_name>
export IBMCLOUD_VPC_REGION=<the_region_name>
export IBMCLOUD_VPC_ZONE=<the_zone_name>
export SE_BOOT=1
export HOST_KEYS_DIR=/root/hostkeys
export IMAGE_NAME=se-podvm-s390x
export CLOUD_PROVIDER=ibmcloud

cd /root/cloud-api-adaptor/ibmcloud/image
make push
make verify
```
> **Note**
> - `make push` will call the [build script](./image/build.sh) to build SE enabled qcow2 image first and then call the [push script](./image/push.sh) to upload the built se-enabled qcow2 image to cos bucket and then create one se-enabled custom image.
- The [build script](./image/build.sh) is following the steps in [the document](https://www.ibm.com/docs/en/linux-on-systems?topic=execution-workload-owner-tasks) to prepare SE enabled qcow2 image
> - `make verify` will call the [verify script](./image/verify.sh) to create one SE enabled vsi and then delete it
> - The `IMAGE_NAME` is must end with `-s390x` when `SE_BOOT=1`

### Troubleshooting
The build se-image will use `/dev/nbd0` and `/dev/nbd1`, please make sure they are avaliable to use
```bash
qemu-nbd --disconnect /dev/nbd0
qemu-nbd --disconnect /dev/nbd1
lsblk
```
If the device can't be disconnected:
- When the `MOUNTPOINT` is not empty, please umount it
```
umount <MOUNTPOINT>
```
- When there is one `crypt` partation, plesse close it
```
lsblk
...
nbd1                                           43:32   0  100G  0 disk
├─nbd1p1                                       43:33   0  255M  0 part
└─nbd1p2                                       43:34   0 92.9G  0 part
  └─LUKS-ad6e1db7-6833-4638-8eb1-08972554149b 253:0    0 92.9G  0 crypt

cryptsetup close LUKS-ad6e1db7-6833-4638-8eb1-08972554149b
```

## Restart cloud-api-adaptor on worker node with new image ID

When the se enabled custom image is created, get the image ID.

Go the development machine to run terraform. We can use the terraform configuration [start-cloud-api-adaptor](./terraform/start-cloud-api-adaptor/) to start the `cloud-api-adaptor` process on the Kubernetes worker node.

Create a `terraform.tfvars` file in the [configuration directory](./terraform/start-cloud-api-adaptor). The `terraform.tfvars` file should look like this:
```
ibmcloud_api_key = "<your API Key>"
region_name = "<the_region_name>"
cluster_name = "<cluster name>"
ssh_key_name = "<your SSH key name>" OR ssh_key_id = "<your SSH key id>"
vpc_name = "<name of your VPC>" OR vpc_id = "<ID of your VPC>"
primary_subnet_name = "<name of your primary subnet>" OR primary_subnet_id = "<ID of your primary subnet>"
primary_security_group_name = "<name of your primary security group>" OR primary_security_group_id = "<ID of your primary security group>"
podvm_image_id = "<ID of your se enabled peer pod VM image>"
instance_profile_name = "bz2e-2x8"
```

Please set `podvm_image_id` as the new custom image ID above. And set `instance_profile_name` as a valid confidential computing profile name(eg `bz2e-2x8`).

Usually, we need destroy existing `cloud-api-adaptor` process first, and then start the new process with the new Pod VM image ID:
```bash
terraform init
terraform destroy
terraform apply
```

## Test

We can follow guide [Deploy the nginx pod and sniff test nginx](./README.md#deploy-the-nginx-pod-and-sniff-test-nginx) to deploy a nginx pod. Verify the Peer Pod VM is created with the SE enabled custom image.
