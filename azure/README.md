# Setup instructions

- Install packer by following the instructions in the following [link](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

- Create a Resource Group

```bash
export RESOURCE_GROUP_NAME="REPLACE_ME"
export LOCATION="REPLACE_ME"

az group create --name "${RESOURCE_GROUP_NAME}" --location "${LOCATION}"
```

- Create Service Principal

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

export VM_SIZE="REPLACE_ME"
```

- Create a custom Azure VM image based on Ubuntu 20.04 having kata-agent and other dependencies.

```bash
CLOUD_PROVIDER=azure make build
```

The output image id will be used while running the cloud-api-adaptor.

# Running cloud-api-adaptor

- Create Service Principal for the CAA

```bash
az ad sp create-for-rbac \
  -n peer-pod-vm-creator \
  --role Contributor \
  --scopes "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP_NAME" \
  --query "{ clientid: appId, secret: password, tenantid: tenant }"
```

Use the output from the previous command as values for the next one, viz. for flags `-clientid`, `-secret` and `-tenantid`.

- Replace the following values by going to the https://portal.azure.com/ :

```bash
./cloud-api-adaptor azure \
  -subscriptionid $SUBSCRIPTION_ID \
  -clientid <> \
  -secret "<>" \
  -tenantid <> \
  -resourcegroup $RESOURCE_GROUP_NAME \
  -region $LOCATION \
  -subnetid "/subscriptions/..." \
  -securitygroupid "/subscriptions/... network security group" \
  -instance-size <> \
  -imageid "/subscriptions/... image id generated before"
```
