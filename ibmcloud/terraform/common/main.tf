#
# (C) Copyright IBM Corp. 2022.
# SPDX-License-Identifier: Apache-2.0
#

resource "ibm_is_vpc" "vpc" {
  name = var.vpc_name
}

resource "ibm_is_public_gateway" "gateway" {
  name        = var.public_gateway_name
  vpc         = ibm_is_vpc.vpc.id
  zone        = var.zone_name
  floating_ip = {
    id = ibm_is_floating_ip.gateway.id
  }
}

resource "ibm_is_floating_ip" "gateway" {
    name = var.floating_ip_name
    zone = var.zone_name
}

resource "ibm_is_subnet" "primary" {
  name                     = var.primary_subnet_name
  vpc                      = ibm_is_vpc.vpc.id
  zone                     = var.zone_name
  total_ipv4_address_count = 256
  public_gateway           = ibm_is_public_gateway.gateway.id
}

resource "ibm_is_security_group" "primary" {
  name = var.primary_security_group_name
  vpc  = ibm_is_vpc.vpc.id
}

resource "ibm_is_security_group_rule" "primary_outbound" {
  group      = ibm_is_security_group.primary.id
  direction  = "outbound"
  remote     = "0.0.0.0/0"
}

resource "ibm_is_security_group_rule" "primary_inbound" {
  group      = ibm_is_security_group.primary.id
  direction  = "inbound"
  remote     = ibm_is_security_group.primary.id
}

resource "ibm_is_security_group_rule" "primary_ssh" {
  group      = ibm_is_security_group.primary.id
  direction  = "inbound"
  remote     = "0.0.0.0/0"

  tcp {
    port_min = 22
    port_max = 22
  }
}

resource "ibm_is_security_group_rule" "primary_ping" {
  group      = ibm_is_security_group.primary.id
  direction  = "inbound"
  remote     = "0.0.0.0/0"

  icmp {
    code = 0
    type = 8
  }
}
