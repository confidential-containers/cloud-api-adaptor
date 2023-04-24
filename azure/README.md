# Cloud API Adaptor on Azure

This documentation will walk you through setting up Cloud API Adaptor (CAA) on Azure Kubernetes Service (AKS). We will build the pod vm image, CAA's application image, deploy one worker AKS, deploy CAA on that Kubernetes cluster and finally deploy a sample application that will run as a pod backed by CAA pod VM.

## Pre-requisites

### Resource Group

We will use this resource group for all of our deployments. Create an Azure resource group by runnint the following command:

```bash
export RESOURCE_GROUP="REPLACE_ME"
export LOCATION="REPLACE_ME"

az group create \
  --name "${RESOURCE_GROUP}" \
  --location "${LOCATION}"
```

### Service Principal

Create a service principal that will be used for building image and its credentials will be provided when deploying Cloud API Adaptor daemonset:

```bash
export SUBSCRIPTION_ID=$(az account show --query id --output tsv)

az ad sp create-for-rbac \
  --name "caa-${RESOURCE_GROUP}"  \
  --role "Contributor"   \
  --scopes "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}" \
  --query "{ CLIENT_ID: appId, CLIENT_SECRET: password, TENANT_ID: tenant }"
```

Set the environment variables by copying the env var `CLIENT_ID`, `CLIENT_SECRET`, `TENANT_ID` from the output of the above command:

```bash
export CLIENT_ID="REPLACE_ME"
export CLIENT_SECRET="REPLACE_ME"
export TENANT_ID="REPLACE_ME"
```

## Build Pod VM Image

- Install packer by following [these instructions](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

> **NOTE**: For setting up authenticated registry support read this [documentation](../docs/registries-authentication.md).

- Create a custom Azure VM image based on Ubuntu 22.04 packed with kata-agent, agent-protocol-forwarder and other dependencies. Run the following commands to build the pod VM image:

```bash
cd image
export PKR_VAR_resource_group="${RESOURCE_GROUP}"
export PKR_VAR_location="${LOCATION}"
export PKR_VAR_subscription_id="${SUBSCRIPTION_ID}"
export PKR_VAR_client_id="${CLIENT_ID}"
export PKR_VAR_client_secret="${CLIENT_SECRET}"
export PKR_VAR_tenant_id="${TENANT_ID}"

# Optional
# export PKR_VAR_az_image_name="REPLACE_ME"
# export PKR_VAR_vm_size="REPLACE_ME"
# export PKR_VAR_ssh_username="REPLACE_ME"

export CLOUD_PROVIDER=azure
make image
```

Use the output of the above command to populate the following environment variable it will be used while deploying the cloud-api-adaptor:

```bash
# e.g. format: /subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/images/...
export AZURE_IMAGE_ID="REPLACE_ME"
```

## Build CAA Container Image

Run the following set of commands from the root of this repository:

```bash
export registry="REPLACE_ME" # e.g. quay.io/username
export CLOUD_PROVIDER=azure
make image
```

## Deploy Kubernetes using AKS

Make changes to the following environment variable as you see fit:

```bash
export CLUSTER_NAME="REPLACE_ME"
export AKS_WORKER_USER_NAME="azuser"
export SSH_KEY=~/.ssh/id_rsa.pub
export AKS_RG="${RESOURCE_GROUP}-aks"
```

Deploy AKS with single worker node to the same resource group we created earlier:

```bash
az aks create \
    --resource-group "${RESOURCE_GROUP}" \
    --node-resource-group "${AKS_RG}" \
    --name "${CLUSTER_NAME}" \
    --location "${LOCATION}" \
    --node-count 1 \
    --node-vm-size Standard_F4s_v2 \
    --ssh-key-value "${SSH_KEY}" \
    --admin-username "${AKS_WORKER_USER_NAME}" \
    --os-sku Ubuntu
```

Download kubeconfig locally to access the cluster using `kubectl`:

```bash
az aks get-credentials \
    --resource-group "${RESOURCE_GROUP}" \
    --name "${CLUSTER_NAME}"
```

Label the nodes so that CAA can be deployed on it:

```bash
kubectl label nodes --all node-role.kubernetes.io/worker=
```

## Deploy Cloud API Adaptor

> **NOTE**: If you are using Calico CNI on a different Kubernetes cluster, then, [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic) VXLAN encapsulation for all inter workload traffic.

### AKS Resource Group permissions

AKS deploys the actual resources like the worker nodes in another resource group named in environment variable `AKS_RG`. For the CAA to be able to create pod VM in the same subnet as the worker nodes of the AKS cluster, run the following command:

```bash
az ad sp create-for-rbac \
    -n "caa-${RESOURCE_GROUP}" \
    --role Contributor \
    --scopes "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AKS_RG}" \
    --query "password"
```

From the output of the above command populate the environment variable below:

```bash
export AZURE_CLIENT_SECRET="REPLACE_ME"
```

### AKS Subnet ID

Fetch the VNET name of that AKS created automatically:

```bash
export VNET_NAME=$(az network vnet list \
  --resource-group "${AKS_RG}" \
  --query "[0].name" \
  --output tsv)
```

Export the subnet ID to be used for CAA daemonset deployment:

```bash
export SUBNET_ID=$(az network vnet subnet list \
  --resource-group "${AKS_RG}" \
  --vnet-name "${VNET_NAME}" \
  --query "[0].id" \
  --output tsv)
```

### Populate the `kustomization.yaml` File

Replace the values as needed for the following environment variables:

```bash
# For regular VMs use something like: Standard_D2as_v5, for CVMs use something like Standard_DC2as_v5.
export AZURE_INSTANCE_SIZE="REPLACE_ME"
```

Run the following command to update the [`kustomization.yaml`](../install/overlays/azure/kustomization.yaml) file:

```bash
cat <<EOF > install/overlays/azure/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
- ../../yamls
images:
- name: cloud-api-adaptor
  newName: "${registry}/cloud-api-adaptor"
  newTag: latest
generatorOptions:
  disableNameSuffixHash: true
configMapGenerator:
- name: peer-pods-cm
  namespace: confidential-containers-system
  literals:
  - CLOUD_PROVIDER="azure"
  - AZURE_SUBSCRIPTION_ID="${SUBSCRIPTION_ID}"
  - AZURE_REGION="${LOCATION}"
  - AZURE_INSTANCE_SIZE="${AZURE_INSTANCE_SIZE}"
  - AZURE_RESOURCE_GROUP="${RESOURCE_GROUP}"
  - AZURE_SUBNET_ID="${SUBNET_ID}"
  - AZURE_IMAGE_ID="${AZURE_IMAGE_ID}"
secretGenerator:
- name: peer-pods-secret
  namespace: confidential-containers-system
  literals:
  - AZURE_CLIENT_ID="${CLIENT_ID}"
  - AZURE_CLIENT_SECRET="${AZURE_CLIENT_SECRET}"
  - AZURE_TENANT_ID="${TENANT_ID}"
- name: ssh-key-secret
  namespace: confidential-containers-system
  files:
  - id_rsa.pub
EOF
```

The ssh public key should be accessible to the kustomization file:

```bash
cp $SSH_KEY install/overlays/azure/id_rsa.pub
```

### Deploy CAA on the Kuberentes cluster

Run the following command to deploy CAA:

```bash
make deploy
```

Generic CAA deployment instructions are also described [here](../install/README.md).

## Run Sample Application

### Ensure Runtimeclass

Verify that the runtime class is created after deploying CAA:

```bash
kubectl get runtimeclass
```

Once you can find a runtimeclass named `kata` then you can be sure that the deployment was successful. Successful deployment will look like this:

```console
$ kubectl get runtimeclass
NAME        HANDLER     AGE
kata        kata        7s
kata-clh    kata-clh    7s
kata-qemu   kata-qemu   7s
```

### Deploy Workload

Deploy a nginx deployment:

```yaml
echo '
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      runtimeClassName: kata
      containers:
      - name: nginx
        image: bitnami/nginx:1.14
        ports:
        - containerPort: 80
        imagePullPolicy: Always
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```bash
kubectl get pods -n default
```

You can verify that the pod vm was created by running the following command:

```bash
az vm list \
  --resource-group "${RESOURCE_GROUP}" \
  --output table
```

Here you should see the vm associated with the pod `nginx`.

## Cleanup

If you wish to clean up the whole set up, you can delete the resource group by running the following command:

```bash
az group delete \
  --name "${RESOURCE_GROUP}" \
  --yes --no-wait
```
