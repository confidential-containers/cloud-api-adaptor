//go:build vsphere
// +build vsphere

package registry

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/vsphere"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

func newServer(cfg hypervisor.Config, cloudConfig interface{}, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return vsphere.NewServer(cfg, cloudConfig.(vsphere.Config), workerNode, daemonPort)
}
