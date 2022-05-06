# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs.

This procedure has been confirmed using the following repositories.
* https://github.com/liudalibj/cloud-api-adaptor/tree/zvsi
* https://github.com/yoheiueda/kata-containers/tree/peerpod-2022.04.04

The setup procedure includes the following sub tasks.

* Create a Virtual Private Cloud (VPC) including security groups, subnet, and gateway
* Create a Kubernetes cluster on two virtual server instances (VSIs)
* Build a custom VM image for pod VMs
* Install cloud-api-adaptor on a worker node
* Run a demo

## Prerequisites

To automate preparation of VPC and VSIs, you need to install terraform and ansible on your client machine. Please follow the the official installation guides.

* [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)


If you are using Ubuntu linux, you can run follow commands simply:
```
sudo apt-get update && sudo apt-get install -y gnupg software-properties-common curl
curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
sudo apt-get install terraform -y
```
* [Install Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)


If you are using Ubuntu linux, you can run follow commands simply:
```
sudo apt-get install -y python3
sudo ln -s /usr/bin/python3 /usr/bin/python
sudo add-apt-repository --yes --update ppa:ansible/ansible
sudo apt-get install ansible
```

Optionally, you can install IBM Cloud CLI.

* [Installing the stand-alone IBM Cloud CLI](https://cloud.ibm.com/docs/cli?topic=cli-install-ibmcloud-cli)


If you are using Ubuntu linux, you can run follow commands simply:
```
curl -fsSL https://clis.cloud.ibm.com/install/linux | sh
ibmcloud plugin install vpc-infrastructure cloud-object-storage
```

Note that you can use the IBM Cloud Web UI for most of the operations of IBM Cloud.

* [https://cloud.ibm.com/vpc-ext/overview](https://cloud.ibm.com/vpc-ext/overview)

You need IBM Cloud API key. You can create your own API key at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys). Please make sure that you are selecting the correct region in the Web UI.

## Create a VPC

First, you need to create a Virtual Private Cloud (VPC). The Terraform configuration files are in [ibmcloud/terraform/common](./terraform/common/).


To use the Terraform configuration, you need to create a file `terraform.tfvars` at in the same directory of the other files of the Terraform configuration to specify your IBM Cloud API Key. `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
```
You can also customize the other parameters by specifying custom values in `terraform.tfvars`. The default values of such parameters are defined in [variables.tf](./terraform/common/variables.tf)


Then, you can create your VPC by executing the following commands.

```
cd ibmcloud/terraform/common
terraform init
terraform plan
terraform apply
```

The following cloud resources will be created. Please check [main.tf](terraform/common/main.tf) for the details.
* VPC
* Security groups
* Subnets
* Public gateway
* Floating IP for the public gateway

## Create a Kubernetes cluster

Another Terraform configuration is available at [ibmcloud/terraform/cluster](./terraform/cluster) to create a
Kubernetes cluster on the VPC you just created.


Note that you can create multiple clusters by using different cluster names.


As usual, you need to create `terraform.tfvars` to specify parameter values. `terraform.tfvars` looks like this.

```
ibmcloud_api_key = "<your API key>"
ssh_key_name = "<your SSH key name>"
cluster_name = "<cluster name>"
```

`ssh_key_name` is a name of your SSH key registered in IBM Cloud.
You can add your SSH key at [https://cloud.ibm.com/vpc-ext/compute/sshKeys](https://cloud.ibm.com/vpc-ext/compute/sshKeys). This ssh key will be installed on control-plane and worker nodes.

`cluster_name` is a name of a Kubernetes cluster. This name is used for the prefix of the names of control-plane and worker nodes. If you want to create another cluster in the same VPC, you need to use a different name for the new cluster.


**Note:** In order to create the cluster based on a different type of VSI image you can overwrite more parameters here e.g. to create a **s390x** based cluster add follow to `terraform.tfvars`
```
instance_profile_name = "bz2-2x8"
image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"
```

Then, execute the following commands to create a new Kubernetes cluster consisting of two virtual server instances. One for a control-plane node, and another one for a worker node. Please check [main.tf](terraform/cluster/main.tf) for the details.

```
cd ibmcloud/terraform/cluster
terraform init
terraform plan
terraform apply
```

You can check the status of provisioned Kubernetes node VM instances at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs).


This Terraform configuration also triggers execution of an Ansible playbook to set up
Kubernetes and other prerequisite software in the two nodes. Please check [ansible/playbook.yml](terraform/cluster/ansible/playbook.yml) for the details.


If ansible fails for some reason, you can rerun the ansible playbook as follows.
```
cd ansible
ansible-playbook -i ./inventory -u root ./playbook.yml
```

When ansible fails, Terraform does not execute the setup script for Kubernetes. In this case, you can manually run it as follows.


Note that you do not need to run this script manually, when everything goes well.

```
./scripts/setup.sh --bastion <floating IP of the worker node> --control-plane <IP address of the control-plane node> --workers  <IP address of the worker node>
```

When two VSIs are successfully provisioned, a floating IP address is assigned to the worker node. Please use the floating IP address to access the worker node from the Internet.
```
ssh root@floating-ip-of-worker-node
```

## Build a pod VM image

You need to build a pod VM image for peer pod VMs. A pod VM image contains the following components.

* Kata agent
* Agent protocol forwarder
* skopeo
* umoci

The build scripts are located in [ibmcloud/image](./image). The prerequisite software to build a pod VM image is already installed in the worker node by [the Ansible playbook](terraform/cluster/ansible/playbook.yml) for convenience.

You need to configure Cloud Object Storage (COS) to upload your custom VM image.

https://cloud.ibm.com/objectstorage/

First, create a COS service instance if you have not create one. Then, create a COS bucket with the COS instance. The COS service instance and bucket names are necessary to upload a custom VM image.

Next, you need to grant access to COS to import images as described at [https://cloud.ibm.com/docs/vpc?topic=vpc-object-storage-prereq&interface=cli](https://cloud.ibm.com/docs/vpc?topic=vpc-object-storage-prereq&interface=cli).

```
ibmcloud login -r jp-tok -apikey $api_key
COS_INSTANCE_GUID=$(ibmcloud resource service-instance --output json "$IBMCLOUD_COS_SERVICE_INSTANCE" | jq -r '.[].guid')
ibmcloud iam authorization-policy-create is cloud-object-storage Reader --source-resource-type image --target-service-instance-id $COS_INSTANCE_GUID
```

You can use a Terraform template located at [ibmcloud/terraform/podvm-build](./terraform/podvm-build) to use Terraform and Ansible to build a pod VM image on the k8s worker node, upload it to a COS bucket and verify it. The architecture of the pod VM image built on the k8s worker node will be the same as that of the node. For example, a k8s worker node using an Intel x86 VSI will build an Intel x86 pod VM image and a k8s worker node using an IBM s390x VSI will build an IBM s390x pod VM image.

Note that building a pod VM image on a worker node using the Terraform template is not recommended for production, and we need to build a pod VM image somewhere secure to protect workloads running in a peer pod VM.

Create the `terraform.tfvars` in [the template directory](./terraform/podvm-build). `terraform.tfvars` should look like this.

```
ibmcloud_api_key = "<your API key>"
ibmcloud_user_id = "<IBM Cloud User ID>"
cluster_name = "<cluster name>"
cos_service_instance_name = "<COS Service Instance Name>"
cos_bucket_name = "<COS Bucket Name>"
```

If you used the Terraform templates in [common](./terraform/common) and [cluster](./terraform/cluster) to create the VPC and VSIs, you should set `ibmcloud_api_key` and `cluster_name` to the same values as those you entered in `terraform.tfvars` for those templates.

`ibmcloud_user_id` should be set to the IBM Cloud user ID who owns the API key `ibmcloud_api_key`. You can look up the user ID using.

```
ibmcloud account users
```

If `ibmcloud account users` displays multiple user IDs, choose the user ID whose state is `ACTIVE`.

The `cos_service_instance_name` and `cos_bucket_name` tfvars are optional and default to values based on your `cluster_name` tfvar if not set, as per the example in the following table.

| Resource | Value |
|----------|-------|
| `cluster_name` tfvar | peer-pod-cluster |
| COS Service Instance name | peer-pod-cluster-cos-service-instance | 
| COS Bucket name | peer-pod-cluster-cos-bucket |

For uploding the pod VM image using Terraform, the COS Bucket must be a regional bucket in the same region (default `jp-tok`) as the VPC and VSIs.

**Note:** The `Operator` and `Console Admin` roles must be [assigned](https://cloud.ibm.com/docs/vpc?topic=vpc-vsi_is_connecting_console&interface=ui) to the user. The Terraform template will create the `Console Admin` role for the user `ibmcloud_user_id` is set to in the template `terraform.tfvars`.

Execute the following commands on your client machine to build, upload and verify the pod VM image.

```
cd ibmcloud/terraform/podvm-build
terraform init
terraform plan
terraform apply
```

**Note:** if your worker node is **s390x** based, the suffix of the created QCOW2 file for the custom image will be `-s390x` otherwise it will be `-amd64`.

**Note:** when verifying the pod VM image, creating a server from a new image may take long time the first time. It typically takes about 10 minutes. From the second time, creating a server from the image takes one minute.


You can check the name and ID of the new image at [https://cloud.ibm.com/vpc-ext/compute/images](https://cloud.ibm.com/vpc-ext/compute/images). Alternatively, you can use the `ibmcloud` command to list your images as follows.

```
ibmcloud is images --visibility=private
```


## Install custom Kata shim

The Ansible playbook automatically installs the custom Kata shim binary and its configuration file. If you want to rebuild the Kata shim, please follow the steps below.

```
cd /root/kata-containers/src/runtime
make $PWD/containerd-shim-kata-v2
install containerd-shim-kata-v2 /usr/local/bin/
```

A minimum Kata shim configuration file at `/etc/kata-containers/configuration.toml` looks like this.

```
[runtime]
internetworking_model = "none"
disable_new_netns = true
disable_guest_seccomp = true
enable_pprof = true
enable_debug = true
[hypervisor.remote]
remote_hypervisor = "/run/peerpod/hypervisor.sock"
[agent.kata]
```

## Install Cloud API adaptor

The Ansible playbook automatically installs the Cloud API adaptor binary. If you want to rebuild it, please follow the steps below.

```
cd /root/cloud-api-adaptor
CLOUD_PROVIDER=ibmcloud make
install cloud-api-adaptor /usr/local/bin/
```

## Launch Cloud API adaptor

You can start Cloud API adaptor as follows. Please update the variable values if you use custom ones. The VPC, region, zone, subnet, security name are must same as the values you just used for creating VPC.

```
api_key=<your API key>
image_name=<pod VM image name>
ssh_key_name=<your SSH key name>
vpc_name=tok-vpc
subnet_name=tok-primary-subnet
security_group_name=tok-primary-security-group
vpc_region=jp-tok
vpc_zone=jp-tok-2
instance_profile=bx2-2x8
```
**Note**: Modify instance_profile to change the type of VSI provisioned e.g. to create 2 vCPU, 8GB RAM balanced `s390x` VSIs for the peer-pod use `instance_profile=bz2-2x8`

```
ibmcloud login -a https://cloud.ibm.com -r $vpc_region -apikey $api_key

image_id=$(ibmcloud is image --output json $image_name | jq -r .id)
vpc_id=$(ibmcloud is vpc --output json $vpc_name | jq -r .id)
ssh_key_id=$(ibmcloud is key --output json $ssh_key_name | jq -r .id)
subnet_id=$(ibmcloud is subnet --output json $subnet_name | jq -r .id)
security_groupd_id=$(ibmcloud is security-group --output json $security_group_name | jq -r .id)

/usr/local/bin/cloud-api-adaptor ibmcloud \
    -api-key "$api_key" \
    -key-id "$ssh_key_id" \
    -image-id "$image_id" \
    -profile-name "$instance_profile" \
    -zone-name "$vpc_zone" \
    -primary-subnet-id "$subnet_id" \
    -primary-security-group-id "$security_groupd_id" \
    -vpc-id "$vpc_id" \
    -pods-dir /run/peerpod/pods \
    -socket /run/peerpod/hypervisor.sock
```

## Demo

Open a new terminal on your client machine, ssh to worker node.


You can create a demo pod as follows. This YAML file will create an nginx pod using a peer pod VM.

```
ssh root@floating-ip-of-worker-node
cd /root/cloud-api-adaptor/ibmcloud/demo
kubectl apply -f runtime-class.yaml -f nginx.yaml
```

The following command shows the status of the pod you just created. When it becomes running, a new peer pod VM instance is running.
```
kubectl get pods
```

You can check the status of pod VM instance at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs). Alternatively, you can use the `ibmcloud` command to list your images as follows.

```
ibmcloud is instances
```

The above YAML file also define a NodePort service. You can access the HTTP port of the pod at the worker node as follows.

```
curl http://localhost:30080
```

The cloud API adaptor establishes a network tunnel between the worker and pod VMs, and the network traffic to/from the pod VM is transparently transferred via the tunnel.


You can also check the pod VM instance architecture by command:
```
kubectl exec nginx -- uname -a
```
If you are using `s390x` based image as pod vm image, the output looks like:
```
Linux nginx 5.4.0-109-generic #123-Ubuntu SMP Fri Apr 8 11:56:05 UTC 2022 s390x GNU/Linux
```

**Note** When the peer pod VSI is created and it fails to start due to [capacity problems](https://cloud.ibm.com/docs/vpc?topic=vpc-instance-status-messages#cannot-start-capacity).


Please stop `cloud-api-adaptor` on worker node, try to run peer pod VSI on another zone:
- Create a new subnet on the target zone by hand.
- Start `cloud-api-adaptor` with new `vpc_zone` and `primary-subnet-id` on worker node.
- Create nginx demo again. 
