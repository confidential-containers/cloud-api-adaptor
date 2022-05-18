// +build libvirt

package registry

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/libvirt"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

func newServer(cfg hypervisor.Config, cloudConfig interface{}, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return libvirt.NewServer(cfg, cloudConfig.(libvirt.Config), workerNode, daemonPort)
}
