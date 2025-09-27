// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import "errors"

// BYOM Provider Error Definitions
//
// This file contains sentinel errors for the BYOM (Bring Your Own Machine) provider.
// Sentinel errors provide stable error types for testing and error handling while
// allowing error messages to evolve independently.

// Pool Management Errors
var (
	// ErrCreatingPoolMgr indicates a failure to create the VM pool manager
	ErrCreatingPoolMgr = errors.New("failed to create VM pool manager")

	// ErrNoAvailableIPs indicates that no IPs are available in the pool for allocation
	ErrNoAvailableIPs = errors.New("no available IPs in pool")

	// ErrRetrievingPoolState indicates an error related to the pool state
	ErrRetrievingPoolState = errors.New("failed to retrieve pool state")

	// ErrUpdatingPoolState indicates an error related to updating the pool state
	ErrUpdatingPoolState = errors.New("failed to update pool state")

	// ErrRetrievingConfigMap indicates an error related to retrieving the pool state ConfigMap
	ErrRetrievingConfigMap = errors.New("failed to retrieve the pool state configmap")

	// ErrUpdatingConfigMap indicates an error related to updating the pool state ConfigMap
	ErrUpdatingConfigMap = errors.New("failed to update the pool state configmap")
)

// Configuration Validation Errors
var (
	// ErrInvalidClient indicates that the Kubernetes client is nil or invalid
	ErrInvalidClient = errors.New("kubernetes client cannot be nil")

	// ErrNilConfig indicates that the configuration cannot be nil
	ErrNilConfig = errors.New("configuration cannot be nil")

	// ErrEmptyPoolIPs indicates that no IP addresses were provided for the pool
	ErrEmptyPoolIPs = errors.New("pool IPs cannot be empty")

	// ErrInvalidIPAddress indicates that an IP address format is invalid
	ErrInvalidIPAddress = errors.New("invalid IP address")
)

// Node Detection Errors
var (
	// ErrNodeNameDetection indicates failure to determine the current node name
	ErrNodeNameDetection = errors.New("unable to determine node name")
)

// Operation Retry Errors
var (

	// ErrConflict indicates a conflict occurred during concurrent operations
	ErrConflict = errors.New("conflict")

	// ErrAllocationRetryExhausted indicates that IP allocation failed after all retries
	ErrAllocationRetryExhausted = errors.New("failed to allocate IP after retries")

	// ErrDeallocationRetryExhausted indicates that IP deallocation failed after all retries
	ErrDeallocationRetryExhausted = errors.New("failed to deallocate IP after retries")
)

// Data Validation Errors
var (
	// ErrInvalidAllocatedIP indicates that an allocated IP address is in invalid format
	ErrInvalidAllocatedIP = errors.New("invalid allocated IP")
)
