# Troubleshooting Guide

This will be updated over time, as issues arise and experience grows.

## Cluster creation when switching use `ssh_pub_key`

If you provide your SSH public key via the `ssh_pub_key` Terraform variable and, having already run `terraform apply`, you run `terraform apply` again then you may see the following error regarding your SSH key:
> Error: Error deleting SSH Key : SSH-key still in use

This is caused by the IBM Cloud terraform provider forcing the replacement of the SSH key resource. As such Terraform may try to delete the existing SSH key while the Virtual Server Instances are using it.

To resolve this issue:

- If running the end-to-end Terraform configuration in [ibmcloud/terraform](./terraform), run `terraform state rm module.cluster.ibm_is_ssh_key.created_ssh_key[0]` to remove the SSH key resource from Terraform's internal state.
- If running the stand-alone cluster creation Terraform configuration in [ibmcloud/terraform/cluster](./terraform/cluster), run `terraform state rm ibm_is_ssh_key.created_ssh_key[0]` to remove the SSH key resource from Terraform's internal state.
- Delete the `ssh_pub_key` variable from the `terraform.tfvars` file
- Then run `terraform apply` again

This will remove the SSH Key from the list of resources whose lifecycle is managed by Terraform.

When you delete the cluster you will need to manually delete the SSH Key in your IBM Cloud VPC Infrastructure.

## Issue with `podvm-build` playbook related to IBM Cloud

This issue was observed by one engineer, and occurred during the `terraform plan` stage of the [podvm-build](https://github.com/confidential-containers/cloud-api-adaptor/tree/staging/ibmcloud/terraform/podvm-build) playbook, where the command failed with: -

```text
│ Error: Iteration over null value
│
│   on main.tf line 16, in locals:
│   15:   is_policies_and_roles = flatten([
│   16:     for policy in data.ibm_iam_user_policy.user_policies.policies: [
│   17:       for resource in policy.resources: policy.roles
│   18:       if resource.service == "is" && resource.resource_group_id == "" && resource.resource_instance_id == ""
│   19:     ]
│   20:   ])
│     ├────────────────
│     │ data.ibm_iam_user_policy.user_policies.policies is null
│
│ A null value cannot be used as the collection in a 'for' expression.
```

This was reproduced as follows: -

### Clone the repo

`git clone -b staging git@github.com:confidential-containers/cloud-api-adaptor.git`

### Switch to the podvm-build subdirectory

`cd cloud-api-adaptor/ibmcloud/terraform/podvm-build`

### Setup terraform.tfvars

`echo 'ibmcloud_api_key="<REDACTED>"' >> terraform.tfvars`

`echo 'ibmcloud_user_id="david_hay@uk.ibm.com"' >> terraform.tfvars`

`echo 'cluster_name="davehay-cluster"' >> terraform.tfvars`

### Initialise Terraform

`terraform init`

```text
Initializing the backend...

Initializing provider plugins...

- Finding latest version of hashicorp/local...
- Finding latest version of hashicorp/null...
- Finding ibm-cloud/ibm versions matching "~> 1.34.0"...
- Installing hashicorp/local v2.2.3...
- Installed hashicorp/local v2.2.3 (signed by HashiCorp)
- Installing hashicorp/null v3.1.1...
- Installed hashicorp/null v3.1.1 (signed by HashiCorp)
- Installing ibm-cloud/ibm v1.34.0...
- Installed ibm-cloud/ibm v1.34.0 (self-signed, key ID AAD3B791C49CC253)

Partner and community providers are signed by their developers.
If you'd like to know more about provider signing, you can read about it here:
<https://www.terraform.io/docs/cli/plugins/signing.html>

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

### Create Terraform execution plan

`terraform plan`

```text
data.ibm_is_subnet.primary: Reading...
data.ibm_iam_user_policy.user_policies: Reading...
data.ibm_is_floating_ip.worker: Reading...
data.ibm_is_instance.worker: Reading...
data.ibm_iam_user_policy.user_policies: Read complete after 1s [id=david_hay@uk.ibm.com]
data.ibm_is_floating_ip.worker: Read complete after 3s [id=r022-3db669a0-606e-45e1-b493-084c78bb2714]
data.ibm_is_subnet.primary: Read complete after 4s [id=02f7-f79cfbb4-6872-4956-8b6a-68f09063833d]
data.ibm_is_instance.worker: Read complete after 4s [id=02f7_05f17f9c-a1a3-4b92-8f53-3a8c1cec08c6]
╷
│ Error: Iteration over null value
│
│   on main.tf line 16, in locals:
│   15:   is_policies_and_roles = flatten([
│   16:     for policy in data.ibm_iam_user_policy.user_policies.policies: [
│   17:       for resource in policy.resources: policy.roles
│   18:       if resource.service == "is" && resource.resource_group_id == "" && resource.resource_instance_id == ""
│   19:     ]
│   20:   ])
│     ├────────────────
│     │ data.ibm_iam_user_policy.user_policies.policies is null
│
│ A null value cannot be used as the collection in a 'for' expression.

For more information, please see the following Slack threads: -
```

This issue was observed with various releases of the [terraform-provider-ibm](https://github.com/IBM-Cloud/terraform-provider-ibm), including `1.34.0`, `1.41.1` and `1.42.0-beta0`.

The issue was also observed with a very simple Terraform playbook: -

```code
locals {
  is_policies_and_roles = flatten([
    for policy in data.ibm_iam_user_policy.user_policies.policies: [
    ]
  ])
}

data "ibm_iam_user_policy" "user_policies" {
  ibm_id = var.ibmcloud_user_id
}
```

and highlighted the fact that Terraform was unable to retrieve the required User Policies via `data.ibm_iam_user_policy.user_policies.policies` in order to be able to loop across the returned list.

Further investigation suggested that the issue only occurred when the IBM Cloud account had a specific User Policy, as per the following example: -

```text
Policy ID:   7152fe85-36da-46b5-ab9b-1eac8d212cd5
Roles:       Reader
Resources:
             Service Name   containers-kubernetes
             Region         us-south
             namespace      default
```

If this policy was removed from the account, the plan ran to completion, and the engineer was able to then run `terraform apply`.

If the policy was then added back into the account, the same `A null value cannot be used as the collection in a 'for' expression` exception occurred.

After discussions with the community who maintain the [terraform-provider-ibm](https://github.com/IBM-Cloud/terraform-provider-ibm) project, an issue - [Data source of IAM policies failing to list the policies if the policy has service specific attributes #3801](https://github.com/IBM-Cloud/terraform-provider-ibm/issues/3801) - was raised, and is being actively worked at time of writing ( May 2022 ).

In the meantime, if this problem occurs for other users, a check for policies containing service-specific attributes should be undertaken, as per the above example, where the User Policy is tagged with a Service Name of `container-kubernetes`.

Whilst it's not 100% clear which service-specific attributes might cause this issue, one can check using the [IBM Cloud CLI tool](https://cloud.ibm.com/docs/cli?topic=cli-getting-started), using a command similar to the following example: -

`ibmcloud iam user-policies david_hay@uk.ibm.com --output JSON | jq '.[] | select(.resources[].attributes[].value=="containers-kubernetes") | {ID:.id,Resources:.resources,Roles:.roles}'`

which should return output similar to the following: -

```json
{
  "ID": "8993f73a-06b1-4e59-81e2-2bd49ddbee3d",
  "Resources": [
    {
      "attributes": [
        {
          "name": "region",
          "operator": "stringEquals",
          "value": "us-south"
        },
        {
          "name": "serviceName",
          "operator": "stringEquals",
          "value": "containers-kubernetes"
        },
        {
          "name": "accountId",
          "operator": "stringEquals",
          "value": "f5e2ac71094077500e0d4b1ef85fdaec"
        }
      ]
    }
  ],
  "Roles": [
    {
      "role_id": "crn:v1:bluemix:public:iam::::role:Viewer",
      "display_name": "Viewer",
      "description": "As a viewer, you can view service instances, but you can't modify them."
    }
  ]
}
```

Commands such as this allow the engineer to selectively export, and then delete the User Policy or Policies that may be blocking the `terraform plan` stage, until the `terraform-provider-ibm` team are able to resolve [Data source of IAM policies failing to list the policies if the policy has service specific attributes #3801](https://github.com/IBM-Cloud/terraform-provider-ibm/issues/3801)

In terms of troubleshooting tips, setting the `TF_LOG` and `TF_LOG_PATH` variables proved useful, in terms of confirming that the `terraform plan` step did correctly return IBM Cloud User Policies etc. from the target account.

For more information, please see [Environment Variables](https://www.terraform.io/cli/config/environment-variables).
