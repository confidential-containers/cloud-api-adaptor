// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/provider"
	"github.com/confidential-containers/cloud-api-adaptor/provider/util/cloudinit"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

type Service interface {
	pb.HypervisorService
	GetInstanceID(ctx context.Context, podNamespace, podName string, wait bool) (string, error)
	ConfigVerifier() error
	Teardown() error
}

type cloudService struct {
	provider     provider.Provider
	proxyFactory proxy.Factory
	workerNode   podnetwork.WorkerNode
	sandboxes    map[sandboxID]*sandbox
	cond         *sync.Cond
	podsDir      string
	daemonPort   string
	mutex        sync.Mutex
	ppService    *k8sops.PeerPodService
	aaKBCParams  string
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
	spec         provider.InstanceTypeSpec
}
