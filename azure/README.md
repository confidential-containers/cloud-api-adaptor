# Setup instructions

- Install packer by using the [following](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli) instructions.

- Create a Resource Group

```bash
export AZURE_RESOURCE_GROUP="REPLACE_ME"
export AZURE_REGION="REPLACE_ME"

az group create --name "${AZURE_RESOURCE_GROUP}" --location "${AZURE_REGION}"
```

- Create Service Principal to build image

```bash
export AZURE_SUBSCRIPTION_ID=$(az account show --query id --output tsv)

az ad sp create-for-rbac \
  --name "Packer Build"  \
  --role "Contributor"   \
  --scopes /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP} \
  --query "{ AZURE_CLIENT_ID: appId, AZURE_CLIENT_SECRET: password, AZURE_TENANT_ID: tenant }"
```


- Set environment variables

The env var `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID` can be copied from the output of the last command:

```bash
export AZURE_CLIENT_ID="REPLACE_ME"
export AZURE_CLIENT_SECRET="REPLACE_ME"
export AZURE_TENANT_ID="REPLACE_ME"
```

- Create Role Assignment

```bash
az role assignment create \
  --assignee ${AZURE_CLIENT_ID} \
  --role "Contributor"    \
  --scope /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP}
```

- Create a custom Azure VM image based on Ubuntu having kata-agent and other dependencies.
	[setting up authenticated registry support](../docs/registries-authentication.md)
```bash
export VM_SIZE="REPLACE_ME"
cd image
CLOUD_PROVIDER=azure PODVM_DISTRO=ubuntu make image && cd -
```

The output image id will be used while running the cloud-api-adaptor, which get's uploaded to your Azure portal using Packer.

You can also build the image using docker
```bash
cd image
docker build -t azure \
--secret id=AZURE_CLIENT_ID \
--secret id=AZURE_CLIENT_SECRET \
--secret id=AZURE_SUBSCRIPTION_ID \
--secret id=AZURE_TENANT_ID \
--build-arg AZURE_REGION=${AZURE_REGION} \
--build-arg AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP} \
-f Dockerfile .
```

If you want to use a different base image, then you'll need to provide additional build-args:
`PUBLISHER`, `OFFER`, `SKU`

Sometimes using the marketplace image requires accepting a licensing agreement and also using a published plan.
Following [link](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage) provides more detail.

For example using the CentOS 8.5 image from eurolinux publisher requires a plan and license agreement.
You'll need to first get the URN:
```
az vm image list --location $AZURE_REGION --publisher eurolinuxspzoo1620639373013  --offer centos-8-5-free --sku centos-8-5-free --all --output table
```
Then you'll need to accept the agreement:
```
az vm image terms accept --urn eurolinuxspzoo1620639373013:centos-8-5-free:centos-8-5-free:8.5.5
```

Then you can use the following command line to build the image:
```
docker build -t azure \
--secret id=AZURE_CLIENT_ID \
--secret id=AZURE_CLIENT_SECRET \
--secret id=AZURE_SUBSCRIPTION_ID \
--secret id=AZURE_TENANT_ID \
--build-arg AZURE_REGION=${AZURE_REGION} \
--build-arg AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP}  \
--build-arg PUBLISHER=eurolinuxspzoo1620639373013 \
--build-arg SKU=centos-8-5-free \
--build-arg OFFER=centos-8-5-free \
--build-arg PLAN_NAME=centos-8-5-free \
--build-arg PLAN_PRODUCT=centos-8-5-free \
--build-arg PLAN_PUBLISHER=eurolinuxspzoo1620639373013 \
--build-arg PODVM_DISTRO=centos \
-f Dockerfile .
```

Here is another example of building RHEL based image:

```
docker build -t azure \
--secret id=AZURE_CLIENT_ID \
--secret id=AZURE_CLIENT_SECRET \
--secret id=AZURE_SUBSCRIPTION_ID \
--secret id=AZURE_TENANT_ID \
--build-arg AZURE_REGION=${AZURE_REGION} \
--build-arg AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP}  \
--build-arg PUBLISHER=RedHat \
--build-arg SKU=9-raw \
--build-arg OFFER=rhel-raw \
--build-arg PODVM_DISTRO=rhel \
-f Dockerfile .
```

# Running cloud-api-adaptor

- If using Calico CNI, [configure](https://projectcalico.docs.tigera.io/networking/vxlan-ipip#configure-vxlan-encapsulation-for-all-inter-workload-traffic) VXLAN encapsulation for all inter workload traffic.

- Create Service Principal for the CAA

```bash
az ad sp create-for-rbac \
  -n peer-pod-vm-creator \
  --role Contributor \
  --scopes "/subscriptions/$AZURE_SUBSCRIPTION_ID/resourceGroups/$AZURE_RESOURCE_GROUP" \
  --query "{ clientid: appId, secret: password, tenantid: tenant }"
```

- Update [kustomization.yaml](../install/overlays/azure/kustomization.yaml) with the required values.

- Deploy Cloud API Adaptor by following the [install](../install/README.md) guide.

