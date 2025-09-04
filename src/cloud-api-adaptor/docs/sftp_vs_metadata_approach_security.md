# SSH/SFTP Threat Analysis for Cloud API Adaptor

## Overview

This document analyzes the threat vectors and security considerations for using SSH/SFTP for initial configuration delivery in confidential computing environments, comparing it with the existing metadata service approach.

## Trust Model

### Key Assumptions

- **Cloud Provider Infrastructure is UNTRUSTED**: This includes:
  - Cloud API Adaptor (CAA)
  - Metadata service and its clients
  - Network infrastructure
  - Hypervisor and host OS
- **Pod VM and its components are TRUSTED**: This includes:
  - SSH/SFTP server running in pod VM
  - process-user-data component
  - Workload applications
  - Pod VM image (OS rootfs)
- **Confidential secrets are obtained separately**: Sensitive secrets are fetched via remote attestation, not through initial config

### Trust Boundary

```text
┌─────────────────────────────────────────────────────────┐
│ UNTRUSTED CLOUD PROVIDER ENVIRONMENT                    │
│                                                         │
│ ┌─────────────────┐    ┌─────────────────────────────┐  │
│ │ Cloud API       │    │ Metadata Service            │  │
│ │ Adaptor (CAA)   │    │ • IMDS/Cloud-init endpoints │  │
│ │ • SSH/SFTP      │    │                             │  │
│ │   Client        │    │                             │  │
│ └─────────────────┘    └─────────────────────────────┘  │
│           │                         │                   │
└───────────┼─────────────────────────┼───────────────────┘
            │                         │
            │ SSH/SFTP Connection     │ HTTP/HTTPS Requests
            │                         │
            ▼                         ▼
┌─────────────────────────────────────────────────────────┐
│ TRUSTED POD VM ENVIRONMENT                              │
│                                                         │
│ ┌─────────────────┐    ┌─────────────────────────────┐  │
│ │ SSH/SFTP Server │    │ Metadata Service Client     │  │
│ │                 │    │                             │  │
│ └─────────────────┘    └─────────────────────────────┘  │
│           │                         │                   │
│           └─────────┬───────────────┘                   │
│                     ▼                                   │
│            ┌─────────────────┐                          │
│            │ process-user-   │                          │
│            │ data            │                          │
│            └─────────────────┘                          │
└─────────────────────────────────────────────────────────┘
```

## Threat Vectors from Untrusted Cloud Provider

### 1. Malicious Configuration Injection

#### SSH/SFTP Approach

**Threat**: Untrusted CAA sends malicious configuration data via SSH/SFTP

- **Attack Vector**: CAA (controlled by cloud provider) sends crafted configuration files
- **Impact**: Denial of service, resource exhaustion, configuration corruption
- **Risk Level**: MEDIUM (mitigated by input validation in process-user-data)

#### Metadata Service Approach

**Threat**: Untrusted metadata service provides malicious configuration

- **Attack Vector**: Cloud provider's metadata service returns crafted responses
- **Impact**: Denial of service, resource exhaustion, configuration corruption  
- **Risk Level**: MEDIUM (mitigated by input validation in process-user-data)

**Mitigations**:

- Robust input validation and sanitization in process-user-data
- Configuration schema enforcement
- Resource limits and timeouts
- Fail-safe defaults

### 2. Pod VM Impersonation

#### Both Approaches

**Threat**: Cloud provider redirects connections to malicious pod VM

- **Attack Vector**: DNS manipulation, network routing, or providing wrong IP addresses
- **Impact**: Workload runs on compromised infrastructure
- **Risk Level**: MEDIUM
- **Detection**: Remote attestation should detect wrong TEE/measurements

**Mitigations**:

- **Remote attestation**: Ensure secrets are delivered only after successful remote attestation

### 3. Authentic Pod VM with Malicious Intent

#### Both Approaches

**Threat**: Cloud provider uses correct pod VM image but with malicious pod

- **Attack Vector**: Using malicious pod to extract the sensitive secret post successful remote attestation using a valid pod VM image  
- **Impact**: Untrusted entity able to retrieve sensitive secret
- **Risk Level**: CRITICAL
- **Detection**: None

**Mitigations**:

- **Kata agent policy**: Using restricted kata-agent policy to explicitly allow specific images and operations
- **Trusted container images**: Ensure you only use trusted container images verified via policy attestation

## Comparative Risk Analysis

In an untrusted cloud provider scenario, both approaches have equivalent trust profiles.