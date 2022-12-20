# Setup instructions

- Install packer by using the [following](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli) instructions.

- Create a Resource Group

```bash
export RESOURCE_GROUP="REPLACE_ME"
export LOCATION="REPLACE_ME"

az group create --name "${RESOURCE_GROUP}" --location "${LOCATION}"
```

- Create Service Principal to build image

```bash
SUBSCRIPTION_ID=$(az account show --query id --output tsv)

az ad sp create-for-rbac \
  --role Contributor \
  --scopes "/subscriptions/$SUBSCRIPTION_ID" \
  --query "{ client_id: appId, client_secret: password, tenant_id: tenant }"
```

- Set environment variables

The env var `CLIENT_ID`, `CLIENT_SECRET`, `TENANT_ID` can be copied from the output of the last command:

```bash
export CLIENT_ID="REPLACE_ME"
export CLIENT_SECRET="REPLACE_ME"
export TENANT_ID="REPLACE_ME"
```

- Create a custom Azure VM image based on Ubuntu 20.04 having kata-agent and other dependencies.
	[setting up authenticated registry support](../docs/registries-authentication.md)
```bash
export VM_SIZE="REPLACE_ME"
cd image
CLOUD_PROVIDER=azure make build && cd -
```

The output image id will be used while running the cloud-api-adaptor, which get's uploaded to your Azure portal using Packer.

- Export your Azure VM information and run k8s on it

```bash
VM_NAME="REPLACE_ME"
PEER_POD_NAME="OUTPUT_FROM_ABOVE"
```

# Running cloud-api-adaptor

- If using Calico CNI, [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic) VXLAN encapsulation for all inter workload traffic.

- Create Service Principal for the CAA

```bash
az ad sp create-for-rbac \
  -n peer-pod-vm-creator \
  --role Contributor \
  --scopes "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP" \
  --query "{ clientid: appId, secret: password, tenantid: tenant }"
```

- Update [kustomization.yaml](../install/overlays/azure/kustomization.yaml) with the required values.

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide.
