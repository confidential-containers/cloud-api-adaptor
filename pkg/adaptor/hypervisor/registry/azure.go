//go:build azure
// +build azure

package registry

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/azure"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

//nolint:typecheck
func newServer(cfg hypervisor.Config, cloudConfig interface{}, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return azure.NewServer(cfg, cloudConfig.(azure.Config), workerNode, daemonPort)
}
