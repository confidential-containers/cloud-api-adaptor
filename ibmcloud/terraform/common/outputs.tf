#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

output "primary_security_group_id" {
    value = ibm_is_security_group.primary.id
}

output "primary_subnet_id" {
    value = ibm_is_subnet.primary.id
}

output "vpc_id" {
    value = ibm_is_vpc.vpc.id
}

output "ssh_security_group_rule_id" {
    value = ibm_is_security_group_rule.primary_ssh.id
}

output "inbound_security_group_rule_id" {
    value = ibm_is_security_group_rule.primary_inbound.id
}

output "outbound_security_group_rule_id" {
    value = ibm_is_security_group_rule.primary_outbound.id
}