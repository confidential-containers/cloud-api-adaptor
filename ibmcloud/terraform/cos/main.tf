data "ibm_resource_group" "group" {
  name = var.resource_group_name
}

########## Create a COS instance
resource "ibm_resource_instance" "cos_instance" {
  name              = var.cos_service_instance_name
  service           = "cloud-object-storage"
  plan              = "standard"
  location          = "global"
  resource_group_id = data.ibm_resource_group.group.id
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
  region_location      = var.region_name
  storage_class        = "standard"
}
