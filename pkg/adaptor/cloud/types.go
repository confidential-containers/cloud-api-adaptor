// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

type Provider interface {
	CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec InstanceTypeSpec) (instance *Instance, err error)
	DeleteInstance(ctx context.Context, instanceID string) error
	Teardown() error
}

type Instance struct {
	ID   string
	Name string
	IPs  []netip.Addr
}

type Service interface {
	pb.HypervisorService
	GetInstanceID(ctx context.Context, podNamespace, podName string, wait bool) (string, error)
	Teardown() error
}

type cloudService struct {
	provider     Provider
	proxyFactory proxy.Factory
	workerNode   podnetwork.WorkerNode
	sandboxes    map[sandboxID]*sandbox
	cond         *sync.Cond
	podsDir      string
	daemonPort   string
	mutex        sync.Mutex
	ppService    *k8sops.PeerPodService
}

type InstanceTypeSpec struct {
	InstanceType string
	VCPUs        int64
	Memory       int64
	Arch         string
	GPUs         int64
}

type sandboxID string

type sandbox struct {
	agentProxy   proxy.AgentProxy
	podNetwork   *tunneler.Config
	cloudConfig  *cloudinit.CloudConfig
	id           sandboxID
	podName      string
	podNamespace string
	instanceName string
	instanceID   string
	netNSPath    string
	spec         InstanceTypeSpec
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
