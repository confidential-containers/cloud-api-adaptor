# Setup instructions

- Install packer by using the [following](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli) instructions.

- Create an Azure resource group:

```bash
export RESOURCE_GROUP="REPLACE_ME"
export LOCATION="REPLACE_ME"

az group create \
  --name "${RESOURCE_GROUP}" \
  --location "${LOCATION}"
```

- Create service principal that will be used for building image and its credentials will be provided when deploying Cloud API Adaptor daemonset:

```bash
export SUBSCRIPTION_ID=$(az account show --query id --output tsv)

az ad sp create-for-rbac \
  --name "packerbuilder-${RESOURCE_GROUP}"  \
  --role "Contributor"   \
  --scopes "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}" \
  --query "{ CLIENT_ID: appId, CLIENT_SECRET: password, TENANT_ID: tenant }"
```

- Set the environment variables

Copy the env var `CLIENT_ID`, `CLIENT_SECRET`, `TENANT_ID` from the output of the above command:

```bash
export CLIENT_ID="REPLACE_ME"
export CLIENT_SECRET="REPLACE_ME"
export TENANT_ID="REPLACE_ME"
```

- Create role assignment:

```bash
az role assignment create \
  --assignee "${CLIENT_ID}" \
  --role "Contributor" \
  --scope "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}"
```

- Create a custom Azure VM image based on Ubuntu 22.04 packed with kata-agent, agent-protocol-forwarder and other dependencies. For setting up authenticated registry support read this [documentation](../docs/registries-authentication.md).

- Run the following commands to build the pod VM image:

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

The output image id will be used while running the cloud-api-adaptor.

# Running cloud-api-adaptor

- If using Calico CNI, [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic) VXLAN encapsulation for all inter workload traffic.

- Update [kustomization.yaml](../install/overlays/azure/kustomization.yaml) with the required values.

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide.
