# Setup procedure for IBM Cloud

This guide describes how to set up a demo environment on IBM Cloud for peer pod VMs.

This procedure has been confirmed using the following repositories.
* https://github.com/confidential-containers/cloud-api-adaptor/tree/staging
* https://github.com/yoheiueda/kata-containers/tree/peerpod-2022.04.04

The setup procedure includes the following sub tasks.

* Create a Virtual Private Cloud (VPC) including security groups, subnet, and gateway
* Create a Kubernetes cluster on two virtual server instances (VSIs)
* Build a custom VM image for pod VMs
* Install cloud-api-adaptor on a worker node
* Run a demo

## Prerequisites

To automate preparation of VPC and VSIs, you need to install terraform and ansible on your `development machine`. Please follow the the official installation guides.

* [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)

> **Tip:** If you are using Ubuntu linux, you can run follow commands simply:
> ```bash
> $ sudo apt-get update && sudo apt-get install -y gnupg software-properties-common curl
> $ curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
> $ sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
> $ sudo apt-get install terraform -y
> ```

* [Install Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)

> **Tip:** If you are using Ubuntu linux, you can run follow commands simply:
> ```bash
> $ sudo apt-get install -y python3
> $ sudo ln -s /usr/bin/python3 /usr/bin/python
> $ sudo add-apt-repository --yes --update ppa:ansible/ansible
> $ sudo apt-get install ansible -y
> ```

Optionally, you can install IBM Cloud CLI.

* [Installing the stand-alone IBM Cloud CLI](https://cloud.ibm.com/docs/cli?topic=cli-install-ibmcloud-cli)

> **Tips**
> - If you are using Ubuntu linux, you can run follow commands simply:
>     ```bash
>     $ curl -fsSL https://clis.cloud.ibm.com/install/linux | sh
>     $ ibmcloud plugin install vpc-infrastructure cloud-object-storage
>     ```
> - You can use the [IBM Cloud Web UI](https://cloud.ibm.com/vpc-ext/overview) for most of the operations of IBM Cloud. And please make sure that you are selecting the correct region in the Web UI.
> 

* You need IBM Cloud API key. You can create your own API key at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).

## Create a VPC

First, you need to create a Virtual Private Cloud (VPC). The Terraform configuration files are in [ibmcloud/terraform/common](./terraform/common/).

To use the Terraform configuration, you need to create a file `terraform.tfvars` at in the same directory of the other files of the Terraform configuration to specify your IBM Cloud API Key. The `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
```
You can also customize the other parameters by specifying custom values in `terraform.tfvars`. The default values of such parameters are defined in [variables.tf](./terraform/common/variables.tf)

Then, you can create your VPC by executing the following commands on your `development machine`.

```bash
$ cd ibmcloud/terraform/common
$ terraform init
$ terraform plan
$ terraform apply
```

The following cloud resources will be created. Please check [main.tf](terraform/common/main.tf) for the details.
* VPC
* Security groups
* Subnets
* Public gateway
* Floating IP for the public gateway

## Create a Kubernetes cluster

Another Terraform configuration is available at [ibmcloud/terraform/cluster](./terraform/cluster) to create a Kubernetes cluster on the VPC you just created.

> **Tip:** You can create multiple clusters by using different cluster names.

As usual, you need to create `terraform.tfvars` to specify parameter values. The `terraform.tfvars` looks like this.

```
ibmcloud_api_key = "<your API key>"
ssh_key_name = "<your SSH key name>"
cluster_name = "<cluster name>"
```
> **Hint:** In order to create the cluster based on a different type of VSI image you can overwrite more parameters here e.g. to create a **s390x** based cluster add follow two lines to the `terraform.tfvars` file
>
>
>     instance_profile_name = "bz2-2x8"
>     image_name = "ibm-ubuntu-18-04-1-minimal-s390x-3"
>

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API Key that you just created at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `ssh_key_name` is a name of your SSH key registered in IBM Cloud. It is used to access a Generation 2 virtual server instance. You can add your SSH key at [https://cloud.ibm.com/vpc-ext/compute/sshKeys](https://cloud.ibm.com/vpc-ext/compute/sshKeys). This ssh key will be installed on control-plane and worker nodes. For more information, about SSH key, see [managing SSH Keys](https://cloud.ibm.com/docs/vpc?topic=vpc-ssh-keys).
>
> - `cluster_name` is a name of a Kubernetes cluster. This name is used for the prefix of the names of control-plane and worker nodes. If you want to create another cluster in the same VPC, you need to use a different name for the new cluster.
> - `instance_profile_name` is a name of IBM Cloud virtual server instance profile. This name is used to create IBM Cloud virtual server instance. For more information, about virtual server instance profile, see [instance profiles](https://cloud.ibm.com/docs/vpc?topic=vpc-profiles).
> - `image_name` is a name of IBM Cloud Infrastructure image. This name is used to create IBM Cloud virtual server instance. For more information, about VPC custom images, see [IBM Cloud Importing and managing custom images](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-images).


Then, execute the following commands to create a new Kubernetes cluster consisting of two virtual server instances. One for a control-plane node, and another one for a worker node. Please check [main.tf](terraform/cluster/main.tf) for the details.

```bash
$ cd ibmcloud/terraform/cluster
$ terraform init
$ terraform plan
$ terraform apply
```

> **Tip:** You can check the status of provisioned Kubernetes node VM instances at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs).


This Terraform configuration also triggers execution of an Ansible playbook to set up Kubernetes and other prerequisite software in the two nodes. Please check [ansible/playbook.yml](terraform/cluster/ansible/playbook.yml) for the details.


If ansible fails for some reason, you can rerun the ansible playbook as follows.
```bash
$ cd ansible
$ ansible-playbook -i ./inventory -u root ./playbook.yml
```

When ansible fails, Terraform does not execute the setup script for Kubernetes. In this case, you can manually run it as follows.

```bash
$ ./scripts/setup.sh --bastion <floating IP of the worker node> --control-plane <IP address of the control-plane node> --workers  <IP address of the worker node>
```

> **Note:** You do not need to run this script manually, when everything goes well.
As there is only a single note. All of the rest look correct though!

When two VSIs are successfully provisioned, a floating IP address is assigned to the worker node. You can use the floating IP address to access the worker node from the Internet, or to ssh into the worker node from your `development machine`:
```bash
$ ssh root@floating-ip-of-worker-node
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

You can use the Terraform template located at [ibmcloud/terraform/cos](./terraform/cos)to use Terraform to create a COS service instance, COS bucket, and IAM AuthorizationPolicy automatically. These resources are configured to store images. Create a `terraform.tfvars` file in the templates directory that includes these fields:

```
ibmcloud_api_key = "<your API key>"
cos_bucket_name = "<COS bucket name>"
cos_service_instance_name = "<COS instance name>"
```
> **Note:** The environment variable `cos_service_instance_name` in `variables.tf` has the default value `cos-image-instance` which will be used by Terraform if you do not provide a unique value in `terraform.tfvars`.

Then run the Template via the following commands: 
```bash
$ cd ibmcloud/terraform/cos
$ terraform init
$ terraform plan
$ terraform apply
```

You can use a Terraform template located at [ibmcloud/terraform/podvm-build](./terraform/podvm-build) to use Terraform and Ansible to build a pod VM image on the k8s worker node, upload it to a COS bucket and verify it. The architecture of the pod VM image built on the k8s worker node will be the same as that of the node. For example, a k8s worker node using an Intel **x86** VSI will build an Intel **x86** pod VM image and a k8s worker node using an IBM **s390x** VSI will build an IBM **s390x** pod VM image.

> **Warning:** Building a pod VM image on a worker node using the Terraform template is not recommended for production, and we need to build a pod VM image somewhere secure to protect workloads running in a peer pod VM.

Create the `terraform.tfvars` file in [the template directory](./terraform/podvm-build). The `terraform.tfvars` looks like this.
```
ibmcloud_api_key = "<your API key>"
ibmcloud_user_id = "<IBM Cloud User ID>"
cluster_name = "<cluster name>"
cos_service_instance_name = "<COS Service Instance Name>"
cos_bucket_name = "<COS Bucket Name>"
```

If you used the Terraform templates in [common](./terraform/common) and [cluster](./terraform/cluster) to create the VPC and VSIs, you should set `ibmcloud_api_key` and `cluster_name` to the same values as those you entered in `terraform.tfvars` for those two templates.

> **Notes:**
> - `ibmcloud_user_id` is the IBM Cloud user ID who owns the API key `ibmcloud_api_key`. You can look up the user ID using.
>     ```bash
>     $ ibmcloud account users
>     ```
>     If command `ibmcloud account users` displays multiple user IDs, choose the user ID whose state is `ACTIVE`.
> - `cos_service_instance_name` is the COS Service Instance Name, optional, default to value based on your `cluster_name` tfvar if not set, `${cluster_name}-cos-service-instance`.
> - `cos_bucket_name` is the COS Bucket Name, optional, default to value based on your `cluster_name` tfvar if not set, `${cluster_name}-cos-bucket`.


> **Notes:**
> - For uploading the pod VM image using Terraform, the COS Bucket must be a regional bucket in the same region (default `jp-tok`) as the VPC and VSIs.
> - The `Operator` and `Console Admin` roles must be [assigned](https://cloud.ibm.com/docs/vpc?topic=vpc-vsi_is_connecting_console&interface=ui) to the user. The Terraform template will create the `Console Admin` role for the user `ibmcloud_user_id` is set to in the template `terraform.tfvars`.

Execute the following commands on your `development machine` to build, upload and verify the pod VM image.

```bash
$ cd ibmcloud/terraform/podvm-build
$ terraform init
$ terraform plan
$ terraform apply
```

> **Notes:** 
> - If your worker node is **s390x** based, the suffix of the created QCOW2 file for the custom image will be `-s390x` otherwise it will be `-amd64`.
> - It typically takes about 15~20 minutes for task `Build peer pod VM image`
> - It typically takes about 7~10 minutes for task `Push peer pod VM image to Cloud Object Store and verify the image`
> - After all tasks finished, when you creating a server from the image it will only takes 1~5 minutes.
> - You can check the name and ID of the new image at [https://cloud.ibm.com/vpc-ext/compute/images](https://cloud.ibm.com/vpc-ext/compute/images). Alternatively, you can use the `ibmcloud` command to list your images as follows.
>    ```bash
>    $ ibmcloud is images --visibility=private
>    ```


## Install custom Kata shim

The Ansible playbook automatically installs the custom Kata shim binary and its configuration file to worker node. If you want to rebuild the Kata shim, please follow the steps below.

```bash
$ cd /root/kata-containers/src/runtime
$ make $PWD/containerd-shim-kata-v2
$ install containerd-shim-kata-v2 /usr/local/bin/
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

The Ansible playbook automatically installs the Cloud API adaptor binary to worker node. If you want to rebuild it, please follow the steps below.

```bash
$ cd /root/cloud-api-adaptor
$ CLOUD_PROVIDER=ibmcloud make
$ install cloud-api-adaptor /usr/local/bin/
```

## Launch Cloud API adaptor

A terraform template that will start the `cloud-api-adaptor` process on the Kubernetes worker node is available in [ibmcloud/terraform/start-cloud-api-adaptor](./terraform/start-cloud-api-adaptor).

Create a `terraform.tfvars` file in the [template directory](./terraform/start-cloud-api-adaptor) for this Terraform template on your `development machine`. The `terraform.tfvars` file should look like this

```
ibmcloud_api_key = "<your API Key>"
cluster_name = "<cluster name>"
ssh_key_name = "<your SSH key name>"
podvm_image_name = "<name of your pod VM image>"
```

> **Hints:**
> - The `instance_profile_name` optional variable sets the CPU architecture, number of vCPUs and memory of each peer pod Virtual Server instance. E.g., the `bz2-2x8` instance profile uses the s390x CPU architecture, has 2 vCPUs and 8 GiB of memory
> - If you created the cluster based on an s390x architecture VSI image you must set the `instance_profile_name` parameter to the name of an s390x-architecture instance profile. E.g., if your cluster uses the **s390x** CPU architecture add the following line to the `terraform.tfvars` file
>
>     instance_profile_name = "bz2-2x8"
>
> `bz2-2x8` can be replaced with the name of a different s390x-architecture instance profile

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API Key that you created at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `ssh_key_name` is a name of your SSH key registered in IBM Cloud. This must be the same SSH key that is installed on and used to access your control-plane and Kubernetes worker nodes.
> - `cluster_name` is a name of a Kubernetes cluster. You must use the same value for this parameter as you used for the corresponding parameter when running the Terraform template in [ibmcloud/terraform/cluster](terraform/cluster) to create the cluster.
> - `podvm_image_name` is the Custom Image for VPC that was built and uploaded by running the Terraform template in [ibmcloud/terraform/podvm-build](terraform/podvm-build). View [IBM Cloud Custom images for VPC](https://cloud.ibm.com/vpc-ext/compute/images) for your chosen region to view the name of the pod VM custom image that was built and uploaded, or run the command `ibmcloud is images --visibility=private`.
> - `instance_profile_name` is a name of IBM Cloud virtual server instance profile. This instance profile name is used to create IBM Cloud Virtual Server instances for peer pods. The default value is `bx2-2x8`, which is a Virtual Server instance that uses the Intel CPU architecture, has 2 vCPUs and 8 GiB of memory.

Execute the following commands on your `development machine` to start the cloud api adaptor on your worker instance:

```bash
$ cd ibmcloud/terraform/start-cloud-api-adaptor
$ terraform init
$ terraform plan
$ terraform apply
```

After `terraform apply` completes the `cloud-api-adaptor` process will run on the Kubernetes worker instance until it is deleted. See the subsection [delete the demo configuration and pod](./README.md#delete-the-demo-configuration-and-pod).

## Demo

### Deploy the nginx pod and sniff test nginx

A Terraform template that will deploy an nginx pod to the Kubernetes cluster is available in [ibmcloud/terraform/run-nginx-demo](./terraform/run-nginx-demo).

Create a `terraform.tfvars` file in the [template directory](./terraform/run-nginx-demo) for this Terraform template on your `development machine`. The `terraform.tfvars` file should look like this

```
ibmcloud_api_key = "<your API Key>"
cluster_name = "<cluster name>"
```

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API Key that you created at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `cluster_name` is a name of a Kubernetes cluster. You must use the same value for this parameter as you used for the corresponding parameter when running the Terraform template in [ibmcloud/terraform/cluster](terraform/cluster) to create the cluster.

Execute the following commands on your `development machine` to deploy the nginx demo workload:

```bash
$ cd ibmcloud/terraform/run-nginx-demo
$ terraform init
$ terraform plan
$ terraform apply
```

Deploying the demo workload will create a new nginx Pod and a NodePort service on your Kubernetes cluster, and a new Virtual Server instance for the peer pod will be created in your IBM Cloud VPC. The `run-nginx-demo` Terraform template will also sniff test the deployed nginx server by accessing the HTTP port of the NodePort service and test that the CPU architecture of the Kubernetes worker matches that of the peer pod instance.

> **Tip:** You can run the nginx sniff test manually if you log into the Kubernetes worker node using the floating IP that was assigned to it
> ```bash
> $ ssh root@floating-ip-of-worker-node
> ```
> Then run the command
> ```bash
> $ curl http://localhost:30080
> ```
> You can also check the CPU architecture the pod VM instance is using by running the command
> ```bash
> $ kubectl exec nginx -- uname -a
> ```
> while logged into to Kubernetes worker node. If you are using a `s390x` based image as the pod VM image, the output looks like this.
> ```
> Linux nginx 5.4.0-109-generic #123-Ubuntu SMP [Date] s390x GNU/Linux
> ```

> **Note:** The cloud API adaptor establishes a network tunnel between the worker and pod VM, and the network traffic to/from the pod VM is transparently transferred via the tunnel.

If you execute the `terraform destroy` command for the `run-nginx-demo` Terraform template the nginx Pod, ConfigMap and NodePort Service, as well as the RuntimeClass for Kata will be deleted on the cluster.

### Check the Virtual Server instance for the nginx pod exists

A Terraform template that checks the nginx peer pod instance has been successfully created on your IBM Cloud VPC is available in [ibmcloud/terraform/check-podvm-instance](./terraform/check-podvm-instance).

Create a `terraform.tfvars` file in the [template directory](./terraform/check-podvm-instance) for this Terraform template on your `development machine`. The `terraform.tfvars` file should look like this

```
ibmcloud_api_key = "<your API Key>"
podvm_image_name = "<name of your pod VM image>"
```

> **Notes:**
> - `ibmcloud_api_key` is your IBM Cloud API Key that you created at [https://cloud.ibm.com/iam/apikeys](https://cloud.ibm.com/iam/apikeys).
> - `podvm_image_name` is the Custom Image for VPC that was built and uploaded by running the Terraform template in [ibmcloud/terraform/podvm-build](terraform/podvm). View [IBM Cloud Custom images for VPC](https://cloud.ibm.com/vpc-ext/compute/images) for your chosen region to view the name of the pod VM custom image that was built and uploaded.

Execute the following commands on your `development machine` to check the nginx pod VM instance exists after deploying the nginx demo workload:

```bash
$ cd ibmcloud/terraform/check-podvm-instance
$ terraform init
$ terraform plan
$ terraform apply
```

If you want to re-run the check, run:
```bash
$ terraform destroy
$ terraform plan
$ terraform apply
```

> **Tip:** You can also check the status of pod VM instance at [https://cloud.ibm.com/vpc-ext/compute/vs](https://cloud.ibm.com/vpc-ext/compute/vs). Alternatively, you can use the `ibmcloud` command to list your images as follows.
>    ```bash
>    $ ibmcloud is instances
>    ```

> **Tip:** When the peer pod VSI is created and it fails to start due to [capacity problems](https://cloud.ibm.com/docs/vpc?topic=vpc-instance-status-messages#cannot-start-capacity).
> Please stop `cloud-api-adaptor` on worker node, try to run peer pod VSI on another zone:
> - Create a new subnet on the target zone by hand.
> - Start `cloud-api-adaptor` with new `vpc_zone` and `primary-subnet-id` on worker node.
> - Create nginx demo again. 

## Clean up

If you want to clean up the IBM Cloud resources created in the above instructions, you can use the following steps:

### Delete the demo configuration and pod
From your development machine navigate to the `run-nginx-demo` repository directory and delete nginx pod on your Kubernetes cluster with:
```bash
$ cd ibmcloud/terraform/run-nginx-demo
$ terraform destroy
```

If the `cloud-api-adaptor` process was still running `terraform destroy` for this Terraform template should automatically delete the peer pod created VM instance too. If the `cloud-api-adaptor` process has stopped, then you can manually check for extra pod VSIs by running:
```bash
$ ibmcloud is instances
```
to see if there are instances other than the control-plane and worker VSIs.

If so these can be deleted with: 
```bash
$ ibmcloud is instance-delete <peer_pod_instance_id>
```

### Stop the cloud API adaptor process
From your development machine navigate to the `start-cloud-api-adaptor` repository directory and stop the cloud API adaptor process on the Kubernetes worker node with:
```bash
$ cd ibmcloud/terraform/start-cloud-api-adaptor
$ terraform destroy
```

### Delete the peer pod VM image

To check and then delete the custom peer pod VM image created run:
```bash
$ ibmcloud is images --visibility=private
$ ibmcloud is image-delete <image_id>
```

### Delete the cluster

From your development machine navigate to the `cloud-api-adaptor` repository directory and delete the VPC Kubernetes
cluster with:
```bash
$ cd ibmcloud/terraform/cluster
$ terraform destroy
```

### Delete the VPC

From your development machine navigate to the `cloud-api-adaptor` repository directory and delete the VPC, security
groups, subnet and gateway with:
```bash
$ cd ibmcloud/terraform/common
$ terraform destroy
```