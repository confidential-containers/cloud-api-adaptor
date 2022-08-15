locals {
  resource_group_id = var.resource_group_name != null ? data.ibm_resource_group.group[0].id : data.ibm_resource_group.default_group.id
  cos_bucket_region = var.cos_bucket_region != "" ? var.cos_bucket_region : var.region_name
}

data "ibm_resource_group" "group" {
  count = var.resource_group_name != null ? 1 : 0
  name = var.resource_group_name
}

data "ibm_resource_group" "default_group" {
  is_default = "true"
}

########## Create a COS instance
resource "ibm_resource_instance" "cos_instance" {
  name              = var.cos_service_instance_name
  service           = "cloud-object-storage"
  plan              = "standard"
  location          = "global"
  resource_group_id = local.resource_group_id
}

####### Create IAM Authorization Policy
resource "ibm_iam_authorization_policy" "policy" {
    source_service_name  = "is"
    source_resource_type = "image"
    target_service_name  = "cloud-object-storage"
    target_resource_instance_id = ibm_resource_instance.cos_instance.guid
    roles                = ["Reader"]
}

######## Create COS bucket
resource "ibm_cos_bucket" "bucket" {
  bucket_name          = var.cos_bucket_name
  resource_instance_id = ibm_resource_instance.cos_instance.id
  region_location      = local.cos_bucket_region
  storage_class        = "standard"
}
