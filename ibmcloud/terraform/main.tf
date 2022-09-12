#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

locals {
    podvm_image_name_with_arch = length(regexall("s390x", var.image_name)) == 1 ? "${var.podvm_image_name}-s390x" : "${var.podvm_image_name}-amd64"
}

module "common" {
    source = "./common"
    ibmcloud_api_key = var.ibmcloud_api_key
    floating_ip_name = var.floating_ip_name
    primary_security_group_name = var.primary_security_group_name
    primary_subnet_name = var.primary_subnet_name
    public_gateway_name = var.public_gateway_name
    region_name = var.region_name
    vpc_name = var.vpc_name
    zone_name = var.zone_name
}

module "cos" {
    source = "./cos"
    ibmcloud_api_key = var.ibmcloud_api_key
    region_name = var.region_name
    cos_bucket_name = var.cos_bucket_name
    cos_service_instance_name = var.cos_service_instance_name
    cos_bucket_region = var.cos_bucket_region
}

module "cluster" {
    source = "./cluster"
    ibmcloud_api_key = var.ibmcloud_api_key
    cluster_name = var.cluster_name
    ssh_key_name = var.ssh_key_name
    image_name = var.image_name
    instance_profile_name = var.instance_profile_name
    primary_subnet_id = module.common.primary_subnet_id
    primary_security_group_id = module.common.primary_security_group_id
    region_name = var.region_name
    ssh_pub_key = var.ssh_pub_key
    vpc_id = module.common.vpc_id
    zone_name = var.zone_name
    ansible_dir = "./cluster/ansible"
    scripts_dir = "./cluster/scripts"
    cloud_api_adaptor_repo = var.cloud_api_adaptor_repo
    cloud_api_adaptor_branch = var.cloud_api_adaptor_branch
    kata_containers_repo = var.kata_containers_repo
    kata_containers_branch = var.kata_containers_branch
    containerd_repo = var.containerd_repo
    containerd_branch = var.containerd_branch
}

module "podvm_build" {
    source = "./podvm-build"
    ibmcloud_api_key = var.ibmcloud_api_key
    ibmcloud_user_id = var.ibmcloud_user_id
    cos_bucket_name = module.cos.cos_bucket_name
    cos_service_instance_id = module.cos.cos_instance_id
    cos_bucket_region = module.cos.cos_bucket_region
    primary_subnet_name = var.primary_subnet_name
    region_name = var.region_name
    use_ibmcloud_test = var.use_ibmcloud_test
    vpc_name = var.vpc_name
    worker_ip = module.cluster.worker_ip
    bastion_ip = module.cluster.bastion_ip
    podvm_image_name = local.podvm_image_name_with_arch
    skip_verify_console = var.skip_verify_console
    ansible_dir = "./podvm-build/ansible"
}

module "start_cloud_api_adaptor" {
    source = "./start-cloud-api-adaptor"
    ibmcloud_api_key = var.ibmcloud_api_key
    ssh_key_id = module.cluster.ssh_key_id
    worker_ip = module.cluster.worker_ip
    bastion_ip = module.cluster.bastion_ip
    podvm_image_id = module.podvm_build.podvm_image_id
    region_name = var.region_name
    vpc_id = module.common.vpc_id
    primary_subnet_id = module.common.primary_subnet_id
    primary_security_group_id = module.common.primary_security_group_id
    instance_profile_name = var.instance_profile_name
    ssh_security_group_rule_id = module.common.ssh_security_group_rule_id
    inbound_security_group_rule_id = module.common.inbound_security_group_rule_id
    outbound_security_group_rule_id = module.common.outbound_security_group_rule_id
    ansible_dir = "./start-cloud-api-adaptor/ansible"
}

module "run_nginx_demo" {
    source = "./run-nginx-demo"
    ibmcloud_api_key = var.ibmcloud_api_key
    worker_ip = module.cluster.worker_ip
    bastion_ip = module.cluster.bastion_ip
    region_name = var.region_name
    podvm_image_id = module.start_cloud_api_adaptor.cloud_api_adaptor_podvm_image_id
    vpc_id = module.common.vpc_id
    ssh_security_group_rule_id = module.common.ssh_security_group_rule_id
    inbound_security_group_rule_id = module.common.inbound_security_group_rule_id
    outbound_security_group_rule_id = module.common.outbound_security_group_rule_id
    ansible_dir = "./run-nginx-demo/ansible"
}
