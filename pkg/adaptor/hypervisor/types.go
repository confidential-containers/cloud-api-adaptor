package hypervisor

import (
     "context"
)

type Server interface {
	Start(ctx context.Context) error
	Shutdown() error
	Ready() chan struct{}
}

const (
        DefaultSocketPath = "/run/peerpod/hypervisor.sock"
        DefaultPodsDir    = "/run/peerpod/pods"
)

type Config struct {
        SocketPath        string
        PodsDir           string
        HypProvider       string
}
