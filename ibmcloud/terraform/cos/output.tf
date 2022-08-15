##############################################################################
# Outputs
##############################################################################
output "resource_group_id" {
  description = "Resource Group ID"
  value       = local.resource_group_id
}

output "s3_region" {
  description = "S3 Region"
  value       = ibm_cos_bucket.bucket.region_location
}

output "s3_endpoint_private" {
  description = "S3 private endpoint"
  value       = ibm_cos_bucket.bucket.s3_endpoint_private
}

output "s3_endpoint_public" {
  description = "S3 public endpoint"
  value       = ibm_cos_bucket.bucket.s3_endpoint_public
}

output "cos_bucket_name" {
  description = "ID of the created COS bucket"
  value       = ibm_cos_bucket.bucket.bucket_name
}

output "cos_instance_id" {
  description = "ID of the created COS instance"
  value       = ibm_resource_instance.cos_instance.id
}

output "cos_bucket_region" {
  description = "Region of the created COS bucket"
  value       = ibm_cos_bucket.bucket.region_location
}
