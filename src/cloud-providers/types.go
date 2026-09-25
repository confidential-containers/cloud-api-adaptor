// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

type Provider interface {
	CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec InstanceTypeSpec) (instance *Instance, err error)
	DeleteInstance(ctx context.Context, instanceID string) error
	Teardown() error
	ConfigVerifier() error
}

const ClusterUIDTagKey = "caa-cluster-uid"

// ClusterUID holds the kube-system namespace UID for this cluster.
// Set once at startup by main.go; providers read it in CreateInstance
// to tag VMs for orphan GC discovery.
var ClusterUID string

type ListInstancesInput struct {
	ClusterUID string
}

// InstanceLister is an optional interface that providers can implement to
// support orphan VM garbage collection. ListInstances returns all instances
// tagged as belonging to this cluster.
type InstanceLister interface {
	ListInstances(ctx context.Context, input ListInstancesInput) ([]*Instance, error)
}

// keyValueFlag represents a flag of key-value pairs
type KeyValueFlag map[string]string

// String returns the string representation of the keyValueFlag
func (k *KeyValueFlag) String() string {
	var pairs []string
	for key, value := range *k {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, ", ")
}

// Set parses the input string and sets the keyValueFlag value
func (k *KeyValueFlag) Set(value string) error {
	// Check if keyValueFlag is initialized. If not initialize it
	if *k == nil {
		*k = make(KeyValueFlag, 0)
	}
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) != 2 {
			return errors.New("invalid key-value pair: " + pair)
		}
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		// Append the key, value to the map
		(*k)[key] = value

	}

	return nil
}

type Instance struct {
	ID   string
	Name string
	IPs  []netip.Addr
	// CreatedAt is the instance creation timestamp from the cloud provider API
	// (UTC). Available for informational logging; not used for grace period
	// decisions (which use local discovery time instead to avoid cross-clock
	// dependency in disconnected environments).
	CreatedAt time.Time
}

type InstanceTypeSpec struct {
	InstanceType string
	VCPUs        int64
	Memory       int64
	Arch         string
	GPUs         int64
	Image        string
	MultiNic     bool
	Volumes      []CloudVolume
}

type CloudVolume struct {
	DiskID string
}
