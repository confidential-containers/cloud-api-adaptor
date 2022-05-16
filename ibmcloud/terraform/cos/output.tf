##############################################################################
# Outputs
##############################################################################
output "resource_group_id" {
  description = "Resource Group ID"
  value       = var.resource_group_id
}

output "s3_region" {
  description = "S3 Region"
  value       = ibm_cos_bucket.state_bucket.region_location
}

output "s3_endpoint_private" {
  description = "S3 private endpoint"
  value       = ibm_cos_bucket.state_bucket.s3_endpoint_private
}

output "s3_endpoint_public" {
  description = "S3 public endpoint"
  value       = ibm_cos_bucket.state_bucket.s3_endpoint_public
}

output "cos_bucket_name" {
  description = "Bucket Name"
  value       = ibm_cos_bucket.state_bucket.bucket_name
}
