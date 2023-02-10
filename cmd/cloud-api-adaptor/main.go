// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/cmd"
	"github.com/confidential-containers/cloud-api-adaptor/cmd/cloud-api-adaptor/cloudmgr"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

const programName = "cloud-api-adaptor"

type daemonConfig struct {
	serverConfig adaptor.ServerConfig
	networkConfig
}

type networkConfig struct {
	TunnelType    string
	HostInterface string
	VXLANPort     int
	VXLANMinID    int
}

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {

	if len(os.Args) < 2 {
		fmt.Printf("%s \n<aws|azure|ibmcloud|libvirt|vsphere|version> [options]\n", os.Args[0])
		cmd.Exit(1)
	}

	if os.Args[1] == "version" {
		cmd.ShowVersion(programName)
		cmd.Exit(0)
	}

	//TODO: transition to better CLI library

	cloudName := os.Args[1]

	if cloud := cloudmgr.Get(cloudName); cloud != nil {

		fmt.Printf("%s: starting Cloud API Adaptor daemon for %q\n", programName, cloudName)

		cmd.Parse(programName, os.Args[1:], func(flags *flag.FlagSet) {

			flags.StringVar(&cfg.serverConfig.SocketPath, "socket", adaptor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&cfg.serverConfig.PodsDir, "pods-dir", adaptor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&cfg.serverConfig.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&cfg.serverConfig.PauseImage, "pause-image", "", "pause image to be used for the pods")
			flags.StringVar(&cfg.serverConfig.ForwarderPort, "forwarder-port", daemon.DefaultListenPort, "port number of agent protocol forwarder")

			flags.StringVar(&cfg.networkConfig.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.networkConfig.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.networkConfig.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.networkConfig.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")

			cloud.ParseCmd(flags)
		})

		cloud.LoadEnv()

		workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface, cfg.VXLANPort, cfg.VXLANMinID)

		provider, err := cloud.NewProvider()
		if err != nil {
			return nil, err
		}

		server := adaptor.NewServer(provider, &cfg.serverConfig, workerNode)

		return cmd.NewStarter(server), nil
	}

	return nil, fmt.Errorf("Unsupported cloud provider: %s", cloudName)
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
