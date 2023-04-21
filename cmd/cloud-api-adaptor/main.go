// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/cmd"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/cloudmgr"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/tlsutil"

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

func printHelp(out io.Writer) {
	fmt.Fprintf(out, "Usage: %s <provider-name> [options] | help | version\n", programName)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Supported cloud providers are:")

	for _, name := range cloudmgr.List() {
		fmt.Fprintf(out, "\t%s\n", name)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Use \"%s <provider-name> -help\" to show options for a cloud provider\n", programName)
}

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {

	if len(os.Args) < 2 {
		printHelp(os.Stderr)
		cmd.Exit(1)
	}

	//TODO: transition to better CLI library

	cloudName := os.Args[1]

	switch cloudName {
	case "version":
		cmd.ShowVersion(programName)
		cmd.Exit(0)
	case "help":
		printHelp(os.Stdout)
		cmd.Exit(0)
	}

	if len(cloudName) == 0 || cloudName[0] == '-' {
		fmt.Fprintf(os.Stderr, "%s: Unknown option: %q\n\n", programName, cloudName)
		printHelp(os.Stderr)
		cmd.Exit(1)
	}

	cloud := cloudmgr.Get(cloudName)

	if cloud == nil {
		fmt.Fprintf(os.Stderr, "%s: Unsupported cloud provider: %s\n\n", programName, cloudName)
		printHelp(os.Stderr)
		cmd.Exit(1)
	}

	var (
		disableTLS bool
		tlsConfig  tlsutil.TLSConfig
	)

	cmd.Parse(programName, os.Args[1:], func(flags *flag.FlagSet) {

		flags.Usage = func() {
			fmt.Fprintf(flags.Output(), "Usage: %s %s [options]\n\n", programName, cloudName)
			fmt.Fprintf(flags.Output(), "The options for %q are:\n", cloudName)
			flags.PrintDefaults()
		}

		flags.StringVar(&cfg.serverConfig.SocketPath, "socket", adaptor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
		flags.StringVar(&cfg.serverConfig.PodsDir, "pods-dir", adaptor.DefaultPodsDir, "base directory for pod directories")
		flags.StringVar(&cfg.serverConfig.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
		flags.StringVar(&cfg.serverConfig.PauseImage, "pause-image", "", "pause image to be used for the pods")
		flags.StringVar(&cfg.serverConfig.ForwarderPort, "forwarder-port", daemon.DefaultListenPort, "port number of agent protocol forwarder")
		flags.StringVar(&tlsConfig.CAFile, "ca-cert-file", "", "CA cert file")
		flags.StringVar(&tlsConfig.CertFile, "cert-file", "", "cert file")
		flags.StringVar(&tlsConfig.KeyFile, "cert-key", "", "cert key")
		flags.BoolVar(&tlsConfig.SkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification - use it only for testing")
		flags.BoolVar(&disableTLS, "disable-tls", false, "Disable TLS encryption - use it only for testing")
		flags.DurationVar(&cfg.serverConfig.ProxyTimeout, "proxy-timeout", proxy.DefaultProxyTimeout, "Maximum timeout in minutes for establishing agent proxy connection")

		flags.StringVar(&cfg.networkConfig.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
		flags.StringVar(&cfg.networkConfig.HostInterface, "host-interface", "", "Host Interface")
		flags.IntVar(&cfg.networkConfig.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
		flags.IntVar(&cfg.networkConfig.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")

		cloud.ParseCmd(flags)
	})

	cmd.ShowVersion(programName)

	fmt.Printf("%s: starting Cloud API Adaptor daemon for %q\n", programName, cloudName)

	if !disableTLS {
		cfg.serverConfig.TLSConfig = &tlsConfig
	}

	cloud.LoadEnv()

	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface, cfg.VXLANPort, cfg.VXLANMinID)

	provider, err := cloud.NewProvider()
	if err != nil {
		return nil, err
	}

	server := adaptor.NewServer(provider, &cfg.serverConfig, workerNode)

	return cmd.NewStarter(server), nil
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
