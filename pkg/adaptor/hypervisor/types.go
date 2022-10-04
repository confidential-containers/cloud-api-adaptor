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
	DefaultPauseImage = "k8s.gcr.io/pause:3.7"
)

type Config struct {
	SocketPath    string
	CriSocketPath string
	PauseImage    string
	PodsDir       string
	HypProvider   string
}
