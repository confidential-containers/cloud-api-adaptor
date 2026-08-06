# Azure multi-NIC pod networking

## Overview

By default a peer-pod VM on Azure has a single NIC that carries both the
VXLAN tunnel back to the worker node (pod/cluster traffic) and any traffic
the pod sends to the outside world. This feature attaches a second NIC to
the Pod VM so that:

- `eth0` (primary NIC) carries pod/cluster traffic over the VXLAN tunnel, as
  before.
- `eth1` (secondary NIC) carries the pod's traffic to destinations outside
  the cluster directly over the Azure VNet, bypassing the worker node.

This builds on the existing `EXTERNAL_NETWORK_VIA_PODVM` mechanism (already
used by the AWS and Alibaba Cloud providers) which discovers a second NIC
inside the Pod VM's network namespace and moves it into the pod's namespace
as its default route.

## Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: peer-pods-cm
  namespace: confidential-containers-system
data:
  AZURE_SUBNET_ID: "/subscriptions/<sub-id>/resourceGroups/<rg>/providers/Microsoft.Network/virtualNetworks/<vnet>/subnets/<primary-subnet>"
  AZURE_SECONDARY_SUBNET_ID: "/subscriptions/<sub-id>/resourceGroups/<rg>/providers/Microsoft.Network/virtualNetworks/<vnet>/subnets/<secondary-subnet>"
  EXTERNAL_NETWORK_VIA_PODVM: "true"
  POD_SUBNET_CIDRS: "10.128.0.0/14,172.30.0.0/16,100.64.0.0/16"
```

- `AZURE_SECONDARY_SUBNET_ID` is required to enable multi-NIC. It must be a
  different subnet than `AZURE_SUBNET_ID`, in the same VNet — Azure requires
  all NICs on a VM to share a VNet.
- `EXTERNAL_NETWORK_VIA_PODVM` is the existing flag (shared with other
  providers) that tells cloud-api-adaptor to move a second NIC into the pod
  namespace.
- `POD_SUBNET_CIDRS` should list the cluster's pod, service, and node CIDRs.
  These stay routed over the VXLAN tunnel (`eth0`); everything else follows
  the pod's default route, which points at the secondary NIC (`eth1`).

If `EXTERNAL_NETWORK_VIA_PODVM` is enabled without `AZURE_SECONDARY_SUBNET_ID`
set, `CreateInstance` fails fast with a clear error instead of creating a
single-NIC VM that would later fail pod networking setup.

## How traffic is routed

The Pod VM's kernel picks the most specific matching route:

| Destination | Device | Notes |
|---|---|---|
| CIDRs in `POD_SUBNET_CIDRS` | `eth0` | routed over the VXLAN tunnel to the worker node |
| everything else (default route) | `eth1` | routed directly over the Azure VNet |

## Secondary NIC discovery

Azure does not assign a default route to a secondary NIC via DHCP, so the
gateway can't be read off the interface directly. `getSecondaryInterfaceDetails`
in `pkg/podnetwork/common.go` picks the first non-primary, non-filtered
interface with a valid IPv4 address using, in order:

1. its own default route (used by other providers whose secondary NIC does
   get one), or
2. the primary interface's gateway, if the secondary shares its subnet, or
3. an inferred gateway at the first usable address of its subnet
   (`x.x.x.1`), Azure's convention for subnets it manages — this is the path
   Azure multi-NIC normally takes, since the primary and secondary NICs sit
   on different subnets by design.

## Known constraints

- Both subnets must be in the same Azure VNet (`SubnetsNotInSameVnet` if
  not).
- Both NICs are declared in the VM creation request up front. Azure does not
  support attaching a NIC to a running VM without first deallocating it, so
  the secondary NIC can't be added after the fact the way AWS attaches its
  addon ENI post-launch.
- `AZURE_USE_PUBLIC_IP`, if enabled together with multi-NIC, attaches the
  public IP to the secondary (external) NIC rather than the primary
  (control-plane) NIC.

## Troubleshooting

```bash
ip addr show
ip route show
ip route get 8.8.8.8          # should go via eth1
ip route get <a pod/service IP>  # should go via eth0
```
