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

You need to create a routing table and add that table to your VNET subnet.

- Find your private IP of the k8s host

```bash
PRIVATE_IP=$(az vm list-ip-addresses --resource-group "${RESOURCE_GROUP}" --name "${VM_NAME}" --query '[0].virtualMachine.network.privateIpAddresses[0]' -o tsv)
```

- Find CIDR on the k8s host

```bash
kubectl get -o jsonpath='{.items[0].spec.cidr}' blockaffinities && echo
```

- Create routing table and route

```bash
export CIDR="REPLACE_ME" # From the output above

ROUTE_TABLE="calico-routes"

az network route-table create -g "${RESOURCE_GROUP}" -n "${ROUTE_TABLE}"

az network route-table route create -g "${RESOURCE_GROUP}" \
  --route-table-name "${ROUTE_TABLE}" \
  -n master-route \
  --next-hop-type VirtualAppliance \
  --address-prefix "${CIDR}" \
  --next-hop-ip-address "${PRIVATE_IP}"
```

- Update VNET's subnet with the routing table

```bash
az network vnet subnet update -g "${RESOURCE_GROUP}" \
  -n "${VM_NAME}"Subnet \
  --vnet-name "${VM_NAME}"VNET \
  --network-security-group "${VM_NAME}"NSG \
  --route-table "${route_table_name}"
```

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
