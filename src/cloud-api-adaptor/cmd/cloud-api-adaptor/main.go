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
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/kubemgr"
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
		disableTLS             bool
		tlsConfig              tlsutil.TLSConfig
		secureComms            bool
		secureCommsNoTrustee   bool
		secureCommsInbounds    string
		secureCommsOutbounds   string
		secureCommsPpInbounds  string
		secureCommsPpOutbounds string
		secureCommsKbsAddr     string
	)

	cmd.Parse(programName, os.Args[1:], func(flags *flag.FlagSet) {

		flags.Usage = func() {
			fmt.Fprintf(flags.Output(), "Usage: %s %s [options]\n\n", programName, cloudName)
			fmt.Fprintf(flags.Output(), "The options for %q are:\n", cloudName)
			flags.PrintDefaults()
		}

		flags.StringVar(&cfg.serverConfig.SocketPath, "socket", adaptor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
		flags.StringVar(&cfg.serverConfig.PodsDir, "pods-dir", adaptor.DefaultPodsDir, "base directory for pod directories")
		flags.StringVar(&cfg.serverConfig.PauseImage, "pause-image", "", "pause image to be used for the pods")
		flags.StringVar(&cfg.serverConfig.ForwarderPort, "forwarder-port", daemon.DefaultListenPort, "port number of agent protocol forwarder")
		flags.StringVar(&tlsConfig.CAFile, "ca-cert-file", "", "CA cert file")
		flags.StringVar(&tlsConfig.CertFile, "cert-file", "", "cert file")
		flags.StringVar(&tlsConfig.KeyFile, "cert-key", "", "cert key")
		flags.BoolVar(&tlsConfig.SkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification - use it only for testing")
		flags.BoolVar(&disableTLS, "disable-tls", false, "Disable TLS encryption - use it only for testing")
		flags.BoolVar(&secureComms, "secure-comms", false, "Use SSH to secure communication between cluster and peer pods")
		flags.BoolVar(&secureCommsNoTrustee, "secure-comms-no-trustee", false, "Deliver the keys to peer pods using userdata instead of Trustee")
		flags.StringVar(&secureCommsInbounds, "secure-comms-inbounds", "", "WN Inbound tags for secure communication tunnels")
		flags.StringVar(&secureCommsOutbounds, "secure-comms-outbounds", "", "WN Outbound tags for secure communication tunnels")
		flags.StringVar(&secureCommsPpInbounds, "secure-comms-pp-inbounds", "", "PP Inbound tags for secure communication tunnels")
		flags.StringVar(&secureCommsPpOutbounds, "secure-comms-pp-outbounds", "", "PP Outbound tags for secure communication tunnels")
		flags.StringVar(&secureCommsKbsAddr, "secure-comms-kbs", "kbs-service.trustee-operator-system:8080", "Address of a Trustee Service for Secure-Comms")
		flags.DurationVar(&cfg.serverConfig.ProxyTimeout, "proxy-timeout", proxy.DefaultProxyTimeout, "Maximum timeout in minutes for establishing agent proxy connection")

		flags.StringVar(&cfg.networkConfig.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
		flags.StringVar(&cfg.networkConfig.HostInterface, "host-interface", "", "Host Interface")
		flags.IntVar(&cfg.networkConfig.VXLAN.Port, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
		flags.IntVar(&cfg.networkConfig.VXLAN.MinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		flags.BoolVar(&cfg.networkConfig.ExternalNetViaPodVM, "ext-network-via-podvm", false, "[EXPERIMENTAL] Enable external networking via pod VM")
		// Local pod subnets. This will be used by APF to create routes for local pod subnets when using external networking via pod VM
		flags.Var(&cfg.networkConfig.PodSubnetCIDRs, "pod-subnet-cidrs", "[EXPERIMENTAL] Comma separated CIDRs for local pod subnets")
		flags.StringVar(&cfg.serverConfig.Initdata, "initdata", "", "Default initdata for all Pods")
		flags.BoolVar(&cfg.serverConfig.EnableCloudConfigVerify, "cloud-config-verify", false, "Enable cloud config verify - should use it for production")
		flags.IntVar(&cfg.serverConfig.PeerPodsLimitPerNode, "peerpods-limit-per-node", 10, "peer pods limit per node (default=10)")
		flags.BoolVar(&cfg.serverConfig.EnableScratchDisk, "enable-scratch-disk", false, "Enable scratch disk for pod VMs")
		flags.BoolVar(&cfg.serverConfig.EnableScratchEncryption, "enable-scratch-encryption", false, "Enable encryption for scratch disk")

		cloud.ParseCmd(flags)
	})

	cmd.ShowVersion(programName)

	fmt.Printf("%s: starting Cloud API Adaptor daemon for %q\n", programName, cloudName)

	if secureComms {
		err := kubemgr.InitKubeMgrInVivo()
		if err != nil {
			return nil, fmt.Errorf("secure comms failed to initialize KubeMgr: %w", err)
		}

		cfg.serverConfig.SecureComms = true
		cfg.serverConfig.SecureCommsTrustee = !secureCommsNoTrustee
		cfg.serverConfig.SecureCommsInbounds = secureCommsInbounds
		cfg.serverConfig.SecureCommsOutbounds = secureCommsOutbounds
		cfg.serverConfig.SecureCommsPpInbounds = secureCommsPpInbounds
		cfg.serverConfig.SecureCommsPpOutbounds = secureCommsPpOutbounds
		cfg.serverConfig.SecureCommsKbsAddress = secureCommsKbsAddr
	} else {
		if !disableTLS {
			cfg.serverConfig.TLSConfig = &tlsConfig
		}
	}

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
