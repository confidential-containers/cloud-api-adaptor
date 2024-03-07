# Cloud API Adaptor (CAA) on AWS

This documentation will walk you through setting up CAA (a.k.a. Peer Pods) on Amazon Elastic Kubernetes Service (EKS). It explains how to deploy:

- One worker EKS
- CAA on that Kubernetes cluster
- An Nginx pod backed by CAA pod VM

> **Note**: Run the following commands from the root of this repository.

## Prerequisites

- Install `aws` CLI [tool](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- Install `eksctl` CLI [tool](https://eksctl.io/installation/)
- Set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_REGION` for AWS cli access
- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)
- Install packer's Amazon plugin `packer plugins install github.com/hashicorp/amazon`

## Build CAA pod-VM image

- Set environment variables
```
export CLOUD_PROVIDER=aws # mandatory
export AWS_REGION="us-east-1" # mandatory
export PODVM_DISTRO=ubuntu # mandatory
export INSTANCE_TYPE=c4.xlarge # optional, default is c4.xlarge
export IMAGE_NAME=peer-pod-ami # optional
export VPC_ID=REPLACE_ME # optional, otherwise, it creates and uses the default vpc in the specific region
export SUBNET_ID=REPLACE_ME # must be set if VPC_ID is set
```


[Optional] If you want to change the volume size of the generated AMI, then set the `VOLUME_SIZE` environment variable.
For example if you want to set the volume size to 40 GiB, then do the following:
```
export VOLUME_SIZE=40
```

[Optional] If you want to use a specific port or address for `agent-protocol-forwarder`, set `FORWARDER_PORT` environment variable.
```
export FORWARDER_PORT=<port-number>
```

- Create a custom AWS VM image based on Ubuntu 22.04 having kata-agent and other dependencies

> **NOTE**: For setting up authenticated registry support read this [documentation](../docs/registries-authentication.md).

```
cd aws/image
make image
```

You can also build the custom AMI by running the packer build inside a container:

```
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
-f Dockerfile .
```

If you want to use an existing `VPC_ID` with public `SUBNET_ID` then use the following command:
```
docker build -t aws \
--secret id=AWS_ACCESS_KEY_ID \
--secret id=AWS_SECRET_ACCESS_KEY \
--build-arg AWS_REGION=${AWS_REGION} \
--build-arg VPC_ID=${VPC_ID} \
--build-arg SUBNET_ID=${SUBNET_ID} \
-f Dockerfile .
```

- Note down your newly created AMI_ID and export it via `PODVM_AMI_ID` env variable

Once the image creation is complete, you can use the following CLI command as well to
get the AMI_ID. The command assumes that you are using the default AMI name: `peer-pod-ami`

```
export PODVM_AMI_ID=$(aws ec2 describe-images --query "Images[*].[ImageId]" --filters "Name=name,Values=peer-pod-ami" --region ${AWS_REGION} --output text)
echo ${PODVM_AMI_ID}
```

## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

If you would like to deploy the latest code from the default branch (`main`) of this repository then expose the following environment variable:

```bash
export registry="quay.io/confidential-containers"
```

## Deploy Kubernetes using EKS

- Create cluster config file
This config will create a single node cluster. Feel free to change it as per your requirement.

```
cat > ekscluster-config.yaml <<EOF
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: caa-eks
  region: ${AWS_REGION}
nodeGroups:
  - name: ng-1-workers
    labels: { role: workers }
    instanceType: m5.xlarge
    desiredCapacity: 1
    privateNetworking: true
    ssh:
      allow: true # will use ~/.ssh/id_rsa.pub as the default ssh key
EOF
```
- Create cluster
```
eksctl create cluster -f ekscluster-config.yaml
```

Wait for the cluster to be created.

- Install `fuse` package on the worker node
This is required for `nydus snapshotter` that is used for CoCo.
You can either SSH to the node, or run a debug shell and install the `fuse` package


## Deploy CAA

### Create the credentials file
```
cat <<EOF > install/overlays/aws/aws-cred.env
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
EOF
```

### Update the `kustomization.yaml` file

Run the following command to update the [`kustomization.yaml`](../install/overlays/aws/kustomization.yaml) file:

```bash
cat <<EOF > install/overlays/aws/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
- ../../yamls

images:
- name: cloud-api-adaptor
  newName: quay.io/confidential-containers/cloud-api-adaptor # change image if needed
  newTag: d4496d008b65c979a4d24767979a77ed1ba21e76

generatorOptions:
  disableNameSuffixHash: true

configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - CLOUD_PROVIDER="aws"   
  - PODVM_AMI_ID="${PODVM_AMI_ID}"  
  #- PODVM_INSTANCE_TYPE="m6a.large" # default instance type to use
  #- PODVM_INSTANCE_TYPES="" # comma separated list of supported instance types
secretGenerator:
- name: auth-json-secret
  namespace: confidential-containers-system
  files:
  #- auth.json # set - path to auth.json pull credentials file
- name: peer-pods-secret
  namespace: confidential-containers-system
  # This file should look like this (w/o quotes!):
  # AWS_ACCESS_KEY_ID=...
  # AWS_SECRET_ACCESS_KEY=...
  envs:
    - aws-cred.env
patchesStrategicMerge:
EOF
```

### Deploy CAA on the Kubernetes cluster

Run the following command to deploy CAA:

```bash
CLOUD_PROVIDER=aws make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

## Run sample application

### Ensure runtimeclass is present

Verify that the `runtimeclass` is created after deploying CAA:

```bash
kubectl get runtimeclass
```

Once you can find a `runtimeclass` named `kata-remote` then you can be sure that the deployment was successful. A successful deployment will look like this:

```console
$ kubectl get runtimeclass
NAME          HANDLER       AGE
kata-remote   kata-remote   7m18s
```

### Deploy workload

Create an `nginx` deployment:

```yaml
echo '
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        io.containerd.cri.runtime-handler: kata-remote
    spec:
      runtimeClassName: kata-remote
      containers:
      - image: nginx@sha256:9700d098d545f9d2ee0660dfb155fe64f4447720a0a763a93f2cf08997227279
        name: nginx
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```bash
kubectl get pods -n default
```

You can verify that the peer-pod VM was created by running the following command:

```bash
aws ec2 describe-instances --filters "Name=tag:Name,Values=podvm*" \
   --query 'Reservations[*].Instances[*].[InstanceId, Tags[?Key==`Name`].Value | [0]]' --output table
```

Here you should see the VM associated with the pod `nginx`. If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

## Cleanup

Delete all running pods using the runtimeClass `kata-remote`. You can use the following command for the same:

```
kubectl get pods -A -o custom-columns='NAME:.metadata.name,NAMESPACE:.metadata.namespace,RUNTIMECLASS:.spec.runtimeClassName' | grep kata-remote | awk '{print $1, $2}'
```

Verify that all peer-pod VMs are deleted. You can use the following command to list all the peer-pod VMs
(VMs having prefix `podvm`) and status:

```
aws ec2 describe-instances --filters "Name=tag:Name,Values=podvm*" \
--query 'Reservations[*].Instances[*].[InstanceId, Tags[?Key==`Name`].Value | [0], State.Name]' --output table
```

Delete the EKS cluster by running the following command:

```bash
eksctl delete -f -f ekscluster-config.yaml
```
