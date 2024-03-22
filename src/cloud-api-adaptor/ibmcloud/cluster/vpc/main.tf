#
# (C) Copyright IBM Corp. 2023.
# SPDX-License-Identifier: Apache-2.0
#

resource "ibm_is_vpc" "vpc" {
  name                        = "${var.cluster_name}-vpc"
  default_security_group_name = "${var.cluster_name}-security-group"
}

resource "ibm_is_floating_ip" "gateway" {
  name = "${var.cluster_name}-gateway-ip"
  zone = var.zone
}

resource "ibm_is_public_gateway" "gateway" {
  name = "${var.cluster_name}-gateway"
  vpc  = ibm_is_vpc.vpc.id
  zone = var.zone
  floating_ip = {
    id = ibm_is_floating_ip.gateway.id
  }
}

resource "ibm_is_subnet" "primary" {
  name                     = "${var.cluster_name}-subnet"
  vpc                      = ibm_is_vpc.vpc.id
  zone                     = var.zone
  total_ipv4_address_count = 256
  public_gateway           = ibm_is_public_gateway.gateway.id
}

resource "ibm_is_security_group_rule" "primary_outbound" {
  group     = ibm_is_vpc.vpc.default_security_group
  direction = "outbound"
  remote    = "0.0.0.0/0"
}

resource "ibm_is_security_group_rule" "primary_inbound" {
  group     = ibm_is_vpc.vpc.default_security_group
  direction = "inbound"
  remote    = ibm_is_vpc.vpc.default_security_group
}

resource "ibm_is_security_group_rule" "primary_ssh" {
  group     = ibm_is_vpc.vpc.default_security_group
  direction = "inbound"
  remote    = "0.0.0.0/0"

  tcp {
    port_min = 22
    port_max = 22
  }
}

resource "ibm_is_security_group_rule" "primary_ping" {
  group     = ibm_is_vpc.vpc.default_security_group
  direction = "inbound"
  remote    = "0.0.0.0/0"

  icmp {
    code = 0
    type = 8
  }
}

resource "ibm_is_security_group_rule" "primary_api_server" {
  group     = ibm_is_vpc.vpc.default_security_group
  direction = "inbound"
  remote    = "0.0.0.0/0"

  tcp {
    port_min = 6443
    port_max = 6443
  }
}
