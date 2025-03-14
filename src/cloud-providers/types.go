// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

type Provider interface {
	CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec InstanceTypeSpec) (instance *Instance, err error)
	DeleteInstance(ctx context.Context, instanceID string) error
	Teardown() error
	ConfigVerifier() error
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
}

type InstanceTypeSpec struct {
	InstanceType string
	VCPUs        int64
	Memory       int64
	Arch         string
	GPUs         int64
	Image        string
	MultiNic     bool
}
