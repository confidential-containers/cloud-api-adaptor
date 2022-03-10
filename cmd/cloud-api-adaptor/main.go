// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adapter/cmd"
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor/ibmcloud"
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor/registry"
	daemon "github.com/confidential-containers/cloud-api-adapter/pkg/forwarder"

	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork"
)

const programName = "cloud-api-adaptor"

type daemonConfig struct {
        helperDaemonRoot  string
        httpTunnelTimeout string
        TunnelType        string
        HostInterface     string
}


const DefaultShimTimeout = "60s"

var ibmcfg ibmcloud.Config
var hypcfg hypervisor.Config

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {
	cmd.Parse(programName, os.Args, func(flags *flag.FlagSet) {
		flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
		flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
		flags.StringVar(&ibmcfg.ApiKey, "api-key", "", "IBM Cloud API key")
		flags.StringVar(&ibmcfg.ProfileName, "profile-name", "", "Profile name")
		flags.StringVar(&ibmcfg.ZoneName, "zone-name", "", "Zone name")
		flags.StringVar(&ibmcfg.ImageID, "image-id", "", "Image ID")
		flags.StringVar(&ibmcfg.PrimarySubnetID, "primary-subnet-id", "", "Primary subnet ID")
		flags.StringVar(&ibmcfg.PrimarySecurityGroupID, "primary-security-group-id", "", "Primary security group ID")
		flags.StringVar(&ibmcfg.SecondarySubnetID, "secondary-subnet-id", "", "Secondary subnet ID")
		flags.StringVar(&ibmcfg.SecondarySecurityGroupID, "secondary-security-group-id", "", "Secondary security group ID")
		flags.StringVar(&ibmcfg.KeyID, "key-id", "", "SSH Key ID")
		flags.StringVar(&ibmcfg.VpcID, "vpc-id", "", "VPC ID")
		flags.StringVar(&cfg.TunnelType, "tunnel-type", "routing", "tunnel type for pod networking")
		flags.StringVar(&cfg.HostInterface, "host-interface", "", "network interface name that is used for network tunnel traffic")
		flags.StringVar(&hypcfg.HypProvider, "provider", "ibmcloud", "Hypervisor provider")
	})

	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface)

        var hypervisorServer hypervisor.Server
 
        if hypcfg.HypProvider == "ibmcloud" {
                 hypervisorServer = registry.NewServer(hypcfg, ibmcfg, workerNode, daemon.DefaultListenPort)
        }
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
