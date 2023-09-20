# Cloud API Adaptor (CAA) on Azure

This documentation will walk you through setting up CAA (a.k.a. Peer Pods) on Azure Kubernetes Service (AKS). It explains how to deploy:

- One worker AKS
- CAA on that Kubernetes cluster
- An Nginx pod backed by CAA pod VM

> **Note**: Run the following commands from the root of this repository.

## Pre-requisites

### Azure login

There are a bunch of steps that require you to be logged into your Azure account via `az login`. Retrieve your "Subscription ID" and set your preferred region:

```bash
export AZURE_SUBSCRIPTION_ID=$(az account show --query id --output tsv)
export AZURE_REGION="eastus"
```

### Resource group

> **Note**: Skip this step if you already have a resource group you want to use. Please, export the resource group name in the `AZURE_RESOURCE_GROUP` environment variable.

Create an Azure resource group by running the following command:

```bash
export AZURE_RESOURCE_GROUP="caa-rg-$(date '+%Y%m%b%d%H%M%S')"

az group create \
  --name "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"
```

## Build CAA pod-VM image

> **Note**: If you have made changes to the CAA code that affects the Pod-VM image and you want to deploy those changes then follow [these instructions](build-image.md) to build the pod-vm image.

An automated job builds the pod-vm image each night at 00:00 UTC. You can use that image by exporting the following environment variable:

```bash
export AZURE_IMAGE_ID="/CommunityGalleries/cocopodvm-d0e4f35f-5530-4b9c-8596-112487cdea85/Images/podvm_image0/Versions/$(date -v -1d "+%Y.%m.%d" 2>/dev/null || date -d "yesterday" "+%Y.%m.%d")"
```

Above image version is in the format `YYYY.MM.DD`, so to use the latest image use the date of yesterday.

## Build CAA container image

> **Note**: If you have made changes to the CAA code and you want to deploy those changes then follow [these instructions](https://github.com/confidential-containers/cloud-api-adaptor/blob/main/install/README.md#building-custom-cloud-api-adaptor-image) to build the container image from the root of this repository.

If you would like to deploy the latest code from the default branch (`main`) of this repository then expose the following environment variable:

```bash
export registry="quay.io/confidential-containers"
```

## Deploy Kubernetes using AKS

Make changes to the following environment variable as you see fit:

```bash
export CLUSTER_NAME="caa-$(date '+%Y%m%b%d%H%M%S')"
export AKS_WORKER_USER_NAME="azuser"
export SSH_KEY=~/.ssh/id_rsa.pub
export AKS_RG="${AZURE_RESOURCE_GROUP}-aks"
```

> **Note**: Optionally, deploy the worker nodes into an existing Azure Virtual Network (VNet) and Subnet by adding the following flag: `--vnet-subnet-id $SUBNET_ID`.

Deploy AKS with single worker node to the same resource group you created earlier:

```bash
az aks create \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --node-resource-group "${AKS_RG}" \
  --name "${CLUSTER_NAME}" \
  --enable-oidc-issuer \
  --enable-workload-identity \
  --location "${AZURE_REGION}" \
  --node-count 1 \
  --node-vm-size Standard_F4s_v2 \
  --nodepool-labels node.kubernetes.io/worker= \
  --ssh-key-value "${SSH_KEY}" \
  --admin-username "${AKS_WORKER_USER_NAME}" \
  --os-sku Ubuntu
```

Download kubeconfig locally to access the cluster using `kubectl`:

```bash
az aks get-credentials \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --name "${CLUSTER_NAME}"
```

## Deploy CAA

> **Note**: If you are using Calico Container Network Interface (CNI) on a different Kubernetes cluster, then, [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic) Virtual Extensible LAN (VXLAN) encapsulation for all inter workload traffic.

### User assigned identity and federated credentials

CAA needs privileges to talk to Azure API. This privilege is granted to CAA by associating a workload identity to the CAA service account. This workload identity (a.k.a. user assigned identity) is given permissions to create VMs, fetch images and join networks in the next step.

> **Note**: If you use an existing AKS cluster it might need to be configured to support workload identity and OpenID Connect (OIDC), please refer to the instructions in [this guide](https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster#update-an-existing-aks-cluster).

Start by creating an identity for CAA:

```bash
export AZURE_WORKLOAD_IDENTITY_NAME="caa-identity"

az identity create \
  --name "${AZURE_WORKLOAD_IDENTITY_NAME}" \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --location "${AZURE_REGION}"

export USER_ASSIGNED_CLIENT_ID="$(az identity show \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --name "${AZURE_WORKLOAD_IDENTITY_NAME}" \
  --query 'clientId' \
  -otsv)"
```

Annotate the CAA Service Account with the workload identity's `CLIENT_ID` and make the CAA DaemonSet use workload identity for authentication:

```bash
cat <<EOF > install/overlays/azure/workload-identity.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: cloud-api-adaptor-daemonset
  namespace: confidential-containers-system
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-api-adaptor
  namespace: confidential-containers-system
  annotations:
    azure.workload.identity/client-id: "$USER_ASSIGNED_CLIENT_ID"
EOF
```

### AKS resource group permissions

For CAA to be able to manage VMs assign the identity VM and Network contributor roles, privileges to spawn VMs in `$AZURE_RESOURCE_GROUP` and attach to a VNet in `$AKS_RG`.

```bash
az role assignment create \
  --role "Virtual Machine Contributor" \
  --assignee "$USER_ASSIGNED_CLIENT_ID" \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourcegroups/${AZURE_RESOURCE_GROUP}"

az role assignment create \
  --role "Reader" \
  --assignee "$USER_ASSIGNED_CLIENT_ID" \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourcegroups/${AZURE_RESOURCE_GROUP}"

az role assignment create \
  --role "Network Contributor" \
  --assignee "$USER_ASSIGNED_CLIENT_ID" \
  --scope "/subscriptions/${AZURE_SUBSCRIPTION_ID}/resourcegroups/${AKS_RG}"
```

Create the federated credential for the CAA ServiceAccount using the OIDC endpoint from the AKS cluster:

```bash
export AKS_OIDC_ISSUER="$(az aks show \
  --name "$CLUSTER_NAME" \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --query "oidcIssuerProfile.issuerUrl" \
  -otsv)"

az identity federated-credential create \
  --name caa-fedcred \
  --identity-name caa-identity \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --issuer "${AKS_OIDC_ISSUER}" \
  --subject system:serviceaccount:confidential-containers-system:cloud-api-adaptor \
  --audience api://AzureADTokenExchange
```

### AKS subnet ID

Fetch the VNet name of that AKS created automatically:

```bash
export AZURE_VNET_NAME=$(az network vnet list \
  --resource-group "${AKS_RG}" \
  --query "[0].name" \
  --output tsv)
```

Export the Subnet ID to be used for CAA Daemonset deployment:

```bash
export AZURE_SUBNET_ID=$(az network vnet subnet list \
  --resource-group "${AKS_RG}" \
  --vnet-name "${AZURE_VNET_NAME}" \
  --query "[0].id" \
  --output tsv)
```

### Populate the `kustomization.yaml` file

Replace the values as needed for the following environment variables:

> **Note**: For regular VMs use `Standard_D2as_v5` for the `AZURE_INSTANCE_SIZE`.

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
  - AZURE_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID}"
  - AZURE_REGION="${AZURE_REGION}"
  - AZURE_INSTANCE_SIZE="Standard_DC2as_v5"
  - AZURE_RESOURCE_GROUP="${AZURE_RESOURCE_GROUP}"
  - AZURE_SUBNET_ID="${AZURE_SUBNET_ID}"
  - AZURE_IMAGE_ID="${AZURE_IMAGE_ID}"
secretGenerator:
- name: peer-pods-secret
  namespace: confidential-containers-system
  literals: []
- name: ssh-key-secret
  namespace: confidential-containers-system
  files:
  - id_rsa.pub
patchesStrategicMerge:
- workload-identity.yaml
EOF
```

The SSH public key should be accessible to the `kustomization.yaml` file:

```bash
cp $SSH_KEY install/overlays/azure/id_rsa.pub
```

### Deploy CAA on the Kubernetes cluster

Run the following command to deploy CAA:

```bash
CLOUD_PROVIDER=azure make deploy
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
      runtimeClassName: kata-remote
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
        imagePullPolicy: Always
' | kubectl apply -f -
```

Ensure that the pod is up and running:

```bash
kubectl get pods -n default
```

You can verify that the peer-pod-VM was created by running the following command:

```bash
az vm list \
  --resource-group "${AZURE_RESOURCE_GROUP}" \
  --output table
```

Here you should see the VM associated with the pod `nginx`. If you run into problems then check the troubleshooting guide [here](../docs/troubleshooting/README.md).

## Cleanup

If you wish to clean up the whole set up, you can delete the resource group by running the following command:

```bash
az group delete \
  --name "${AZURE_RESOURCE_GROUP}" \
  --yes --no-wait
```
