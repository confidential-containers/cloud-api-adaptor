# Instance Selection Flow Documentation

This document describes the instance selection flow implemented in `src/cloud-providers/util.go`.

## Overview

The instance selection process determines the most appropriate cloud instance type for the Pod VMs based on resource requirements and annotations. The flow prioritises explicit instance type annotations over computed selections based on resource requirements.

## Core Functions

### SelectInstanceTypeToUse

This is the main entry point for instance selection and implements a priority-based selection process:

1. **Instance Type Annotation (Highest Priority)** - If `spec.InstanceType` is specified, it takes the precedence
2. **GPU-based Selection** - If `spec.GPUs` is specified then it takes the precedence
3. **vCPU/Memory Selection** - If `spec.Memory` or `spec.VCPUs` or both are specified, then selection is based on these resources. `spec.Memory` takes the precedence.

`spec.InstanceType` value comes from the  `io.katacontainers.config.hypervisor.machine_type` pod annotation.
`spec.GPUs` value comes from the `io.katacontainers.config.hypervisor.default_gpus` pod annotation.
`spec.Memory` value comes from the `io.katacontainers.config.hypervisor.default_memory` pod annotation.
`spec.VCPUs` value comes from the `io.katacontainers.config.hypervisor.default_vcpus` pod annotation.

The selected instance type is then verified against the list of valid instance types.

## Selection Priority Flow

```sh
spec.InstanceType specified?
├── Yes → Use specified instance type
└── No
    ├── spec.GPUs > 0?
    │   ├── Yes → GetBestFitInstanceTypeWithGPU(...)
    │   └── No
    └── spec.VCPUs != 0 AND spec.Memory != 0?
        ├── Yes → GetBestFitInstanceType(...)
        └── No → Use default instance type
```

## Instance Selection Algorithms

### SortInstanceTypesOnResources

The `SortInstanceTypesOnResources` function sorts cloud instance types by their resource specifications to enable efficient instance selection.
This sorted instance list is the primary input for the instance selection functions.

This function arranges instance types in ascending order to facilitate finding the best-fit instance for resource requirements using binary search algorithms.

The following is the sorting criteria:

1. **GPU Count** (highest priority) - Instances with fewer GPUs come first
2. **Memory** (medium priority) - When GPU count is equal, sort by memory size
3. **vCPUs** (lowest priority) - When both GPU and memory are equal, sort by CPU count

### GetBestFitInstanceTypeWithGPU

1. Uses binary search finds the smallest instance type that satisfies:
   - `gpus >= required_gpus`
   - `memory >= required_memory`
   - `vcpus >= required_vcpus`

### GetBestFitInstanceType

1. Filters out GPU instances from the sorted list
2. Uses binary search to find the smallest instance type that satisfies:
   - `memory >= required_memory`
   - `vcpus >= required_vcpus`
