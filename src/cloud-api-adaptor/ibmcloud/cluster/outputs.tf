output "vpc_id" { value = local.vpc_id }
output "ssh_key_id" { value = data.ibm_is_ssh_key.ssh_key.id }
output "subnet_id" { value = local.subnet_id }
output "node_name" { value = "${var.cluster_name}-node-${length(module.nodes) - 1}" }
output "security_group_id" { value = local.security_group_id }
output "region" { value = var.region }
output "zone" { value = var.zone }
output "resource_group_id" { value = data.ibm_resource_group.default_group.id }
