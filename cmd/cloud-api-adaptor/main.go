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
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/azure"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/libvirt"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/registry"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/vsphere"
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

var vspherecfg vsphere.Config
var azurecfg azure.Config
var libvirtcfg libvirt.Config
var hypcfg hypervisor.Config

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {

	if len(os.Args) < 2 {
		fmt.Printf("%s aws|azure|ibmcloud|libvirt <options>\n", os.Args[0])
		cmd.Exit(1)
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

	// TODO: following lines will be removed when refactoring is done

	switch os.Args[1] {
	case "azure":
		cmd.Parse("azure", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&azurecfg.ClientId, "clientid", "", "Client Id, defaults to `AZURE_CLIENT_ID`")
			flags.StringVar(&azurecfg.ClientSecret, "secret", "", "Client Secret, defaults to `AZURE_CLIENT_SECRET`")
			flags.StringVar(&azurecfg.TenantId, "tenantid", "", "Tenant Id, defaults to `AZURE_TENANT_ID`")
			flags.StringVar(&azurecfg.ResourceGroupName, "resourcegroup", "", "Resource Group")
			flags.StringVar(&azurecfg.Zone, "zone", "", "Zone")
			flags.StringVar(&azurecfg.Region, "region", "", "Region")
			flags.StringVar(&azurecfg.SubnetId, "subnetid", "", "Network Subnet Id")
			flags.StringVar(&azurecfg.SecurityGroupId, "securitygroupid", "", "Security Group Id")
			flags.StringVar(&azurecfg.Size, "instance-size", "", "Instance size")
			flags.StringVar(&azurecfg.ImageId, "imageid", "", "Image Id")
			flags.StringVar(&azurecfg.SubscriptionId, "subscriptionid", "", "Subscription ID")
			flags.StringVar(&azurecfg.SSHKeyPath, "ssh-key-path", "$HOME/.ssh/id_rsa.pub", "Path to SSH public key")
			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "azure", "Hypervisor provider")
			flags.StringVar(&hypcfg.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&hypcfg.PauseImage, "pause-image", "", "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&azurecfg.ClientId, "AZURE_CLIENT_ID")
		defaultToEnv(&azurecfg.ClientSecret, "AZURE_CLIENT_SECRET")
		defaultToEnv(&azurecfg.TenantId, "AZURE_TENANT_ID")

	case "libvirt":
		cmd.Parse("libvirt", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&libvirtcfg.URI, "uri", "qemu:///system", "libvirt URI")
			flags.StringVar(&libvirtcfg.PoolName, "pool-name", "default", "libvirt storage pool")
			flags.StringVar(&libvirtcfg.NetworkName, "network-name", "default", "libvirt network pool")
			flags.StringVar(&libvirtcfg.DataDir, "data-dir", "/var/lib/libvirt/images", "libvirt storage dir")
			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "libvirt", "Hypervisor provider")
			flags.StringVar(&hypcfg.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&hypcfg.PauseImage, "pause-image", "", "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})

	case "vsphere":
		cmd.Parse("vsphere", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&vspherecfg.VcenterURL, "vcenter-url", "", "URL of vCenter instance to connect to")
			flags.StringVar(&vspherecfg.UserName, "user-name", "", "Username, defaults to `GOVC_USERNAME`")
			flags.StringVar(&vspherecfg.Password, "password", "", "Password, defaults to `GOVC_PASSWORD`")
			// GOVC_THUMBPRINT
			flags.StringVar(&vspherecfg.Thumbprint, "thumbprint", "", "SHA1 thumbprint of the vcenter certificate. Enable verification of certificate chain and host name.")

			flags.StringVar(&vspherecfg.Template, "template", "podvm-template", "vCenter template to deploy")
			// GOVC_DATACENTER
			flags.StringVar(&vspherecfg.Datacenter, "data-center", "", "vCenter desination datacenter name")
			// GOVC_CLUSTER
			flags.StringVar(&vspherecfg.Vcluster, "vcluster-name", "", "vCenter desination cluster name for DRS placement")
			// GOVC_DATASTORE
			flags.StringVar(&vspherecfg.Datastore, "data-store", "", "vCenter datastore")
			// GOVC_RESOURCE_POOL
			flags.StringVar(&vspherecfg.Resourcepool, "resource-pool", "", "vCenter desination resource pool")
			// GOVC_FOLDER
			flags.StringVar(&vspherecfg.Deployfolder, "deploy-folder", "", "vCenter vm desintation folder relative to the vm inventory path (your-data-center/vm). \nExample '-deploy-folder peerods' will create or use the existing folder peerpods as the \ndeploy-folder in /datacenter/vm/peerpods")

			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "vsphere", "Hypervisor provider")
			flags.StringVar(&hypcfg.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&hypcfg.PauseImage, "pause-image", "", "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&vspherecfg.UserName, "GOVC_USERNAME")
		defaultToEnv(&vspherecfg.Password, "GOVC_PASSWORD")
		defaultToEnv(&vspherecfg.Thumbprint, "GOVC_THUMBPRINT")

	default:
		os.Exit(1)
	}

	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface, cfg.VXLANPort, cfg.VXLANMinID)

	var hypervisorServer hypervisor.Server

	if hypcfg.HypProvider == "libvirt" {
		hypervisorServer = registry.NewServer(hypcfg, libvirtcfg, workerNode, daemon.DefaultListenPort)
	} else if hypcfg.HypProvider == "azure" {
		hypervisorServer = registry.NewServer(hypcfg, azurecfg, workerNode, daemon.DefaultListenPort)
	} else if hypcfg.HypProvider == "vsphere" {
		hypervisorServer = registry.NewServer(hypcfg, vspherecfg, workerNode, daemon.DefaultListenPort)
	}

	return cmd.NewStarter(hypervisorServer), nil
}

func defaultToEnv(field *string, env string) {
	if *field == "" {
		*field = os.Getenv(env)
	}
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
