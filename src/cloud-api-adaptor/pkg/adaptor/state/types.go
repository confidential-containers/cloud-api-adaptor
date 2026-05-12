package state

import (
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
)

type SandboxState struct {
	Version      int       `json:"version"`
	SandboxID    string    `json:"sandboxId"`
	PodName      string    `json:"podName"`
	PodNamespace string    `json:"podNamespace"`
	NetNSPath    string    `json:"netNsPath"`
	Running      bool      `json:"running"`
	CreatedAt    time.Time `json:"createdAt"`

	// Set after instance creation
	InstanceID   string     `json:"instanceId,omitempty"`
	InstanceName string     `json:"instanceName,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`

	// For Phase 3 reconnection
	InstanceIPs   []string `json:"instanceIPs,omitempty"`
	ServerName    string   `json:"serverName,omitempty"`
	ForwarderPort string   `json:"forwarderPort,omitempty"`

	// Network configuration for teardown
	PodNetwork *tunneler.Config `json:"podNetwork,omitempty"`
}
