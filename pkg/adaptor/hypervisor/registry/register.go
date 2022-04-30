package registry

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
)


func NewServer(cfg hypervisor.Config, cloudConfig interface{},  workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return newServer(cfg, cloudConfig, workerNode, daemonPort)
}
