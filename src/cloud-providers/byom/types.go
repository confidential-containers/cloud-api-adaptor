// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// vmPoolIPs represents a flag for VM pool IP addresses
type vmPoolIPs []string

// String returns the string representation of the vmPoolIPs
func (v *vmPoolIPs) String() string {
	return strings.Join(*v, ",")
}

// Set parses the input string and sets the vmPoolIPs value
func (v *vmPoolIPs) Set(value string) error {
	if len(value) == 0 {
		*v = make(vmPoolIPs, 0)
		return nil
	}

	entries := strings.Split(value, ",")
	var allIPs []string
	validIPs := make(map[string]struct{})
	uniqueIPs := make([]string, 0, len(allIPs))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue // Skip empty strings
		}

		// Handle range of IPs
		if start, end, found := strings.Cut(entry, "-"); found {
			startIP, err1 := netip.ParseAddr(start)
			endIP, err2 := netip.ParseAddr(end)
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid IP range %q: %v, %v", entry, err1, err2)
			}

			if startIP.Compare(endIP) >= 0 {
				return fmt.Errorf("invalid IP range (startIP <= endIP): %s", entry)
			}

			count := 0
			for ip := startIP; ; ip = ip.Next() {
				count++
				if count > maxRangeIPs {
					log.Printf("IP range %s exceeds maximum limit range, using only the first %d IPs for VM pool", entry, maxRangeIPs)
					break
				}
				allIPs = append(allIPs, ip.String())
				if ip == endIP {
					break
				}
			}
		} else {
			// Validate single IP entries
			ip, err := netip.ParseAddr(entry)
			if err != nil {
				return fmt.Errorf("invalid IP address %q: %w", entry, err)
			}
			allIPs = append(allIPs, ip.String())
		}
	}

	// Handle deduplication of IPs
	for _, ip := range allIPs {
		if _, exists := validIPs[ip]; !exists {
			validIPs[ip] = struct{}{}
			uniqueIPs = append(uniqueIPs, ip)
		}
	}

	*v = uniqueIPs
	return nil
}

// Config holds the BYOM provider configuration
type Config struct {
	VMPoolIPs              vmPoolIPs // VM pool IP addresses (required)
	SSHUserName            string    // SSH username for VM access
	SSHPubKeyPath          string    // SSH public key file path
	SSHPrivKeyPath         string    // SSH private key file path
	SSHPubKey              string    // SSH public key content (populated from file)
	SSHPrivKey             string    // SSH private key content (populated from file)
	SSHTimeout             int       // SSH connection timeout in seconds
	SSHHostKeyAllowlistDir string    // Directory containing allowed SSH host key files (enables allowlist mode if set)

	// Pool management configuration
	PoolNamespace     string // Namespace for ConfigMap storage (default: auto-detect from running pod)
	PoolConfigMapName string // ConfigMap name for state storage (default: "byom-ip-pool-state")
}

// Redact returns a copy of the config with sensitive information redacted
func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "SSHPrivKey").(*Config)
}

// GlobalVMPoolConfig holds configuration for the global VM pool manager
type GlobalVMPoolConfig struct {
	// Kubernetes client configuration
	Namespace     string
	ConfigMapName string

	// Pool configuration
	PoolIPs []string

	// Retry configuration
	MaxRetries    int
	RetryInterval time.Duration

	// Timeout configuration
	OperationTimeout time.Duration

	// Test configuration
	SkipVMReadiness bool // Skip VM readiness checks (for testing)
}

// GlobalVMPoolManager defines the interface for global VM pool state management
type GlobalVMPoolManager interface {
	// AllocateIP allocates an IP from the global pool
	AllocateIP(ctx context.Context, allocationID string, podName string) (netip.Addr, error)

	// DeallocateIP returns an IP to the global pool
	DeallocateIP(ctx context.Context, allocationID string) error

	// GetIPfromAllocationID returns the IP allocated to a specific allocation ID
	GetIPfromAllocationID(ctx context.Context, allocationID string) (netip.Addr, bool, error)

	// GetAllocationIDfromIP returns the allocation ID for a given IP address
	GetAllocationIDfromIP(ctx context.Context, ip netip.Addr) (string, bool, error)

	// GetPoolStatus returns current pool statistics
	GetPoolStatus(ctx context.Context) (total, available, inUse int, err error)

	// RecoverState initializes state from persistent storage
	RecoverState(ctx context.Context, vmCleanupFunc func(context.Context, netip.Addr) error) error

	// ListAllocatedIPs returns all currently allocated IPs
	ListAllocatedIPs(ctx context.Context) (map[string]IPAllocation, error)
}

// IPAllocation represents an allocated IP address
type IPAllocation struct {
	AllocationID string      `json:"allocationID"`
	IP           string      `json:"ip"`
	NodeName     string      `json:"nodeName"` // Track which node allocated this IP
	PodName      string      `json:"podName"`  // For better tracking and debugging
	AllocatedAt  metav1.Time `json:"allocatedAt"`
}

// IPAllocationState represents the global allocation state stored in ConfigMap
type IPAllocationState struct {
	AllocatedIPs map[string]IPAllocation `json:"allocatedIPs"`
	AvailableIPs []string                `json:"availableIPs"`
	LastUpdated  metav1.Time             `json:"lastUpdated"`
	Version      int64                   `json:"version"` // For optimistic locking
}
