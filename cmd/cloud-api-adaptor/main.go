// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/peer-pod-opensource/cmd"
	"github.com/confidential-containers/peer-pod-opensource/pkg/adaptor/hypervisor"
	daemon "github.com/confidential-containers/peer-pod-opensource/pkg/forwarder"

	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork"
)

const programName = "cloud-api-adaptor"

type daemonConfig struct {
	socketPath        string
	podsDir           string
	helperDaemonRoot  string
	httpTunnelTimeout string
	apiKey            string
	TunnelType        string
	HostInterface     string
	hypervisor.ServiceConfig
}

const DefaultShimTimeout = "60s"

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {
	cmd.Parse(programName, os.Args, func(flags *flag.FlagSet) {
		flags.StringVar(&cfg.socketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
		flags.StringVar(&cfg.podsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
		flags.StringVar(&cfg.apiKey, "api-key", "", "IBM Cloud API key")
		flags.StringVar(&cfg.ProfileName, "profile-name", "", "Profile name")
		flags.StringVar(&cfg.ZoneName, "zone-name", "", "Zone name")
		flags.StringVar(&cfg.ImageID, "image-id", "", "Image ID")
		flags.StringVar(&cfg.PrimarySubnetID, "primary-subnet-id", "", "Primary subnet ID")
		flags.StringVar(&cfg.PrimarySecurityGroupID, "primary-security-group-id", "", "Primary security group ID")
		flags.StringVar(&cfg.SecondarySubnetID, "secondary-subnet-id", "", "Secondary subnet ID")
		flags.StringVar(&cfg.SecondarySecurityGroupID, "secondary-security-group-id", "", "Secondary security group ID")
		flags.StringVar(&cfg.KeyID, "key-id", "", "SSH Key ID")
		flags.StringVar(&cfg.VpcID, "vpc-id", "", "VPC ID")
		flags.StringVar(&cfg.TunnelType, "tunnel-type", "routing", "tunnel type for pod networking")
		flags.StringVar(&cfg.HostInterface, "host-interface", "", "network interface name that is used for network tunnel traffic")
	})

	vpcV1, err := hypervisor.NewVpcV1(cfg.apiKey)
	if err != nil {
		return nil, err
	}

	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface)

	hypervisorServer := hypervisor.NewServer(cfg.socketPath, vpcV1, &cfg.ServiceConfig, workerNode, cfg.podsDir, daemon.DefaultListenPort)

	return cmd.NewStarter(hypervisorServer), nil
}

var config cmd.Config = &daemonConfig{}

func main() {

	starter, err := config.Setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := starter.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}
}
