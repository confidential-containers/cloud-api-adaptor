// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/cmd"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/proxy"
	daemon "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/initdata"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/probe"
)

const (
	programName = "cloud-api-adaptor"
)

type daemonConfig struct {
	serverConfig  cloud.ServerConfig
	networkConfig tunneler.NetworkConfig
}

func printHelp(out io.Writer) {
	fmt.Fprintf(out, "Usage: %s <provider-name> [options] | help | version\n", programName)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Supported cloud providers are:")

	for _, name := range provider.List() {
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

	cloud := provider.Get(cloudName)

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

		reg := provider.NewFlagRegistrar(flags)

		// Common flags with environment variable support
		reg.StringWithEnv(&cfg.serverConfig.SocketPath, "socket", adaptor.DefaultSocketPath, "REMOTE_HYPERVISOR_ENDPOINT", "Unix domain socket path of remote hypervisor service")
		reg.StringWithEnv(&cfg.serverConfig.PodsDir, "pods-dir", adaptor.DefaultPodsDir, "PODS_DIR", "base directory for pod directories")
		reg.StringWithEnv(&cfg.serverConfig.PauseImage, "pause-image", "", "PAUSE_IMAGE", "pause image to be used for the pods")
		reg.StringWithEnv(&cfg.serverConfig.ForwarderPort, "forwarder-port", daemon.DefaultListenPort, "FORWARDER_PORT", "port number of agent protocol forwarder")
		reg.StringWithEnv(&tlsConfig.CAFile, "ca-cert-file", "", "CACERT_FILE", "CA cert file")
		reg.StringWithEnv(&tlsConfig.CertFile, "cert-file", "", "CERT_FILE", "cert file")
		reg.StringWithEnv(&tlsConfig.KeyFile, "cert-key", "", "CERT_KEY", "cert key")
		reg.BoolWithEnv(&tlsConfig.SkipVerify, "tls-skip-verify", false, "TLS_SKIP_VERIFY", "Skip TLS certificate verification - use it only for testing")
		reg.DurationWithEnv(&cfg.serverConfig.ProxyTimeout, "proxy-timeout", proxy.DefaultProxyTimeout, "PROXY_TIMEOUT", "Maximum timeout in minutes for establishing agent proxy connection")
		reg.StringWithEnv(&cfg.networkConfig.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "TUNNEL_TYPE", "Tunnel provider")
		reg.IntWithEnv(&cfg.networkConfig.VXLAN.Port, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN_PORT", "VXLAN UDP port number (VXLAN tunnel mode only")
		reg.StringWithEnv(&cfg.serverConfig.Initdata, "initdata", "", "INITDATA", "Default initdata for all Pods")
		reg.BoolWithEnv(&cfg.serverConfig.EnableCloudConfigVerify, "cloud-config-verify", false, "CLOUD_CONFIG_VERIFY", "Enable cloud config verify - should use it for production")
		reg.IntWithEnv(&cfg.serverConfig.PeerPodsLimitPerNode, "peerpods-limit-per-node", 10, "PEERPODS_LIMIT_PER_NODE", "peer pods limit per node (default=10)")
		reg.BoolWithEnv(&cfg.serverConfig.EnableScratchSpace, "enable-scratch-space", false, "ENABLE_SCRATCH_SPACE", "Enable encrypted scratch space for pod VMs")

		// Flags without environment variable support
		flags.BoolVar(&disableTLS, "disable-tls", false, "Disable TLS encryption - use it only for testing")
		flags.StringVar(&cfg.networkConfig.HostInterface, "host-interface", "", "Host Interface")
		flags.IntVar(&cfg.networkConfig.VXLAN.MinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		flags.BoolVar(&cfg.networkConfig.ExternalNetViaPodVM, "ext-network-via-podvm", false, "[EXPERIMENTAL] Enable external networking via pod VM")
		flags.Var(&cfg.networkConfig.PodSubnetCIDRs, "pod-subnet-cidrs", "[EXPERIMENTAL] Comma separated CIDRs for local pod subnets")

		cloud.ParseCmd(flags)
	})

	cmd.ShowVersion(programName)

	fmt.Printf("%s: starting Cloud API Adaptor daemon for %q\n", programName, cloudName)

	if !disableTLS {
		cfg.serverConfig.TLSConfig = &tlsConfig
	}

	// DEPRECATED: LoadEnv() is now a no-op for all providers.
	// Environment variables are loaded during ParseCmd() via FlagRegistrar.
	// This call will be removed in a future release.
	cloud.LoadEnv()

	workerNode, err := podnetwork.NewWorkerNode(&cfg.networkConfig)
	if err != nil {
		return nil, err
	}

	provider, err := cloud.NewProvider()
	if err != nil {
		return nil, err
	}

	if cfg.serverConfig.Initdata != "" {
		idReader := strings.NewReader(cfg.serverConfig.Initdata)
		_, err = initdata.Parse(idReader)
		if err != nil {
			return nil, fmt.Errorf("failed to parse global initdata: %w", err)
		}
	}

	server := adaptor.NewServer(provider, &cfg.serverConfig, workerNode)

	return cmd.NewStarter(server), nil
}

var config = &daemonConfig{}

func main() {
	starter, err := config.Setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go probe.Start(config.serverConfig.SocketPath)

	if err := starter.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		cmd.Exit(1)
	}
}
