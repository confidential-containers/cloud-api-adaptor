# Azure CI Static Infrastructure Setup

This is documentation for the maintainers of the project who will ensure the safe and smooth functioning of the CI resources for the CAA Azure provider.

## Prerequisite

- **Terraform**: Install terraform by following [this documentation](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli).
- **Azure CLI**: Install Azure CLI `az` using the [following documentation](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli).

## Terraform

### Terraform State Storage

Terraform is a declarative infrastructure tool. We define the state of the infrastructure and terraform reconciles the state every time it it invoked. It also creates a state file which is either stored locally or on a remote storage. Since we want more than one person to collaborate and make modifications to this CI's static infrastructure we will store this state in the cloud itself.

To store this state we need to create an [Azure Storage Account](https://learn.microsoft.com/en-us/azure/storage/common/storage-account-overview), the storage account has to be scoped in an [Azure Resource Group](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal#what-is-a-resource-group).

Login using `az` CLI by running the following command:

```bash
az login
```

Now export these environment variables, note that these values should match the `backend` block from the [providers.tf](./providers.tf) file:

```bash
export resource_group_name="caa-azure-state"
export storage_account_name="caaterraformstate"
export container_name="terraform-state"
export location="eastus"
```

Run the following commands to create the terraform state holder:

```bash
az group create \
    --name $resource_group_name \
    --location $location

az storage account create \
    --name $storage_account_name \
    --resource-group $resource_group_name \
    --location $location \
    --sku Standard_LRS \
    --kind StorageV2

az storage container create \
    --name $container_name \
    --account-name $storage_account_name
```

### Static Infra Creation

Now that we have prerequisite storage account in place, now we will use terraform to create the static infrastructure needed for the CI runs. Increase the value of the `ver` in file [terraform.tfvars](./terraform.tfvars) by one and commit the changes.

Run the following commands to stand up the CI's static infrastructure:

```bash
terraform init
terraform apply
```

Make a note of the output of the above command.

## Github Setup

Now expose the output of the above command as secrets verbatim. Follow [this guide](https://docs.github.com/en/actions/security-guides/encrypted-secrets#creating-encrypted-secrets-for-a-repository) on adding secrets to Github repository.

