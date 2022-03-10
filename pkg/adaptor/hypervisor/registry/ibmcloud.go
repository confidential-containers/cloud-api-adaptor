// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor/ibmcloud"
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork"
)

func newServer(cfg hypervisor.Config, cloudConfig interface{}, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {
	return ibmcloud.NewServer(cfg, cloudConfig.(ibmcloud.Config), workerNode, daemonPort)
}
