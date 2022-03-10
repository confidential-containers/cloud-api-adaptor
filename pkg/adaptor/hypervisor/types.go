package hypervisor

import (
     "context"
)

type Server interface {
	Start(ctx context.Context) error
	Shutdown() error
	Ready() chan struct{}
}

type Config interface {}

const (
        DefaultSocketPath = "/run/peerpod/hypervisor.sock"
        DefaultPodsDir    = "/run/peerpod/pods"
)

type ServiceConfig struct {
        ProfileName              string
        ZoneName                 string
        ImageID                  string
        PrimarySubnetID          string
        PrimarySecurityGroupID   string
        SecondarySubnetID        string
        SecondarySecurityGroupID string
        KeyID                    string
        VpcID                    string
}
