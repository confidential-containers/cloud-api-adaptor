// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/cmd"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/aws"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/ibmcloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/registry"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
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
var awscfg aws.Config
var hypcfg hypervisor.Config

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {

	if len(os.Args) < 2 {
		fmt.Printf("%s aws|ibmcloud <options>\n", os.Args[0])
		cmd.Exit(1)
	}

	//TODO: transition to better CLI library

	switch os.Args[1] {
	case "aws":
		cmd.Parse("aws", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&awscfg.AccessKeyId, "aws-access-key-id", "", "Access Key ID")
			flags.StringVar(&awscfg.SecretKey, "aws-secret-key", "", "Secret Key")
			flags.StringVar(&awscfg.Region, "aws-region", "ap-south-1", "Region")
			flags.StringVar(&awscfg.LoginProfile, "aws-profile", "test", "AWS Login Profile")
			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "aws", "Hypervisor provider")
		        flags.StringVar(&cfg.TunnelType, "tunnel-type", "routing", "Tunnel provider")
		        flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")

		})

	case "ibmcloud":
		cmd.Parse("ibmcloud", os.Args[1:], func(flags *flag.FlagSet) {
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
			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "ibmcloud", "Hypervisor provider")
		        flags.StringVar(&cfg.TunnelType, "tunnel-type", "routing", "Tunnel provider")
		        flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
		})

	default:
		os.Exit(1)
	}

	fmt.Printf("cfg: %#v; hypcfg: %#v\n", cfg, hypcfg)
	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface)

	var hypervisorServer hypervisor.Server

	if hypcfg.HypProvider == "ibmcloud" {
		hypervisorServer = registry.NewServer(hypcfg, ibmcfg, workerNode, daemon.DefaultListenPort)
	} else if hypcfg.HypProvider == "aws" {
		hypervisorServer = registry.NewServer(hypcfg, awscfg, workerNode, daemon.DefaultListenPort)
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
