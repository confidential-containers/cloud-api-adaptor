// +build aws

package registry

import (
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/aws"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

func newServer(cfg hypervisor.Config, cloudConfig interface{}, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return aws.NewServer(cfg, cloudConfig.(aws.Config), workerNode, daemonPort)
}
