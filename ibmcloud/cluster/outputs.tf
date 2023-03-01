output "vpc_id" { value = module.vpc.vpc_id }
output "ssh_key_id" { value = data.ibm_is_ssh_key.ssh_key.id }
output "subnet_id" { value = module.vpc.subnet_id }
output "node_name" { value = "${var.cluster_name}-node-${length(module.nodes)-1}" }
output "security_group_id" { value = module.vpc.security_group_id }
