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
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/azure"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/ibmcloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/libvirt"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/registry"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor/vsphere"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler/vxlan"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
)

const programName = "cloud-api-adaptor"

type daemonConfig struct {
	helperDaemonRoot  string
	httpTunnelTimeout string
	TunnelType        string
	HostInterface     string
	VXLANPort         int
	VXLANMinID        int
}

const DefaultShimTimeout = "60s"

var vspherecfg vsphere.Config
var ibmcfg ibmcloud.Config
var awscfg aws.Config
var azurecfg azure.Config
var libvirtcfg libvirt.Config
var hypcfg hypervisor.Config

func (cfg *daemonConfig) Setup() (cmd.Starter, error) {

	if len(os.Args) < 2 {
		fmt.Printf("%s aws|azure|ibmcloud|libvirt <options>\n", os.Args[0])
		cmd.Exit(1)
	}

	//TODO: transition to better CLI library

	switch os.Args[1] {
	case "aws":
		cmd.Parse("aws", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&awscfg.AccessKeyId, "aws-access-key-id", "", "Access Key ID, defaults to `AWS_ACCESS_KEY_ID`")
			flags.StringVar(&awscfg.SecretKey, "aws-secret-key", "", "Secret Key, defaults to `AWS_SECRET_ACCESS_KEY`")
			flags.StringVar(&awscfg.Region, "aws-region", "", "Region")
			flags.StringVar(&awscfg.LoginProfile, "aws-profile", "test", "AWS Login Profile")
			flags.StringVar(&awscfg.LaunchTemplateName, "aws-lt-name", "kata", "AWS Launch Template Name")
			flags.BoolVar(&awscfg.UseLaunchTemplate, "use-lt", false, "Use EC2 Launch Template for the Pod VMs")
			flags.StringVar(&awscfg.ImageId, "imageid", "", "Pod VM ami id")
			flags.StringVar(&awscfg.InstanceType, "instance-type", "t3.small", "Pod VM instance type")
			flags.Var(&awscfg.SecurityGroupIds, "securitygroupids", "Security Group Ids to be used for the Pod VM, comma separated")
			flags.StringVar(&awscfg.KeyName, "keyname", "", "SSH Keypair name to be used with the Pod VM")
			flags.StringVar(&awscfg.SubnetId, "subnetid", "", "Subnet ID to be used for the Pod VMs")
			flags.StringVar(&hypcfg.SocketPath, "socket", hypervisor.DefaultSocketPath, "Unix domain socket path of remote hypervisor service")
			flags.StringVar(&hypcfg.PodsDir, "pods-dir", hypervisor.DefaultPodsDir, "base directory for pod directories")
			flags.StringVar(&hypcfg.HypProvider, "provider", "aws", "Hypervisor provider")
			flags.StringVar(&hypcfg.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&hypcfg.PauseImage, "pause-image", hypervisor.DefaultPauseImage, "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&awscfg.AccessKeyId, "AWS_ACCESS_KEY_ID")
		defaultToEnv(&awscfg.SecretKey, "AWS_SECRET_ACCESS_KEY")

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
			flags.StringVar(&hypcfg.PauseImage, "pause-image", hypervisor.DefaultPauseImage, "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&azurecfg.ClientId, "AZURE_CLIENT_ID")
		defaultToEnv(&azurecfg.ClientSecret, "AZURE_CLIENT_SECRET")
		defaultToEnv(&azurecfg.TenantId, "AZURE_TENANT_ID")

	case "ibmcloud":
		cmd.Parse("ibmcloud", os.Args[1:], func(flags *flag.FlagSet) {
			flags.StringVar(&ibmcfg.ApiKey, "api-key", "", "IBM Cloud API key, defaults to `IBMCLOUD_API_KEY`")
			flags.StringVar(&ibmcfg.IamServiceURL, "iam-service-url", "https://iam.cloud.ibm.com/identity/token", "IBM Cloud IAM Service URL")
			flags.StringVar(&ibmcfg.VpcServiceURL, "vpc-service-url", "https://jp-tok.iaas.cloud.ibm.com/v1", "IBM Cloud VPC Service URL")
			flags.StringVar(&ibmcfg.ResourceGroupID, "resource-group-id", "", "Resource Group ID")
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
			flags.StringVar(&hypcfg.CriSocketPath, "cri-runtime-endpoint", "", "cri runtime uds endpoint")
			flags.StringVar(&hypcfg.PauseImage, "pause-image", hypervisor.DefaultPauseImage, "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&ibmcfg.ApiKey, "IBMCLOUD_API_KEY")

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
			flags.StringVar(&hypcfg.PauseImage, "pause-image", hypervisor.DefaultPauseImage, "pause image to be used for the pods")
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
			flag.BoolVar(&vspherecfg.Insecure, "insecure", true, "Disable certificate verification")

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
			flags.StringVar(&hypcfg.PauseImage, "pause-image", hypervisor.DefaultPauseImage, "pause image to be used for the pods")
			flags.StringVar(&cfg.TunnelType, "tunnel-type", podnetwork.DefaultTunnelType, "Tunnel provider")
			flags.StringVar(&cfg.HostInterface, "host-interface", "", "Host Interface")
			flags.IntVar(&cfg.VXLANPort, "vxlan-port", vxlan.DefaultVXLANPort, "VXLAN UDP port number (VXLAN tunnel mode only")
			flags.IntVar(&cfg.VXLANMinID, "vxlan-min-id", vxlan.DefaultVXLANMinID, "Minimum VXLAN ID (VXLAN tunnel mode only")
		})
		defaultToEnv(&vspherecfg.UserName, "GOVC_USERNAME")
		defaultToEnv(&vspherecfg.Password, "GOVC_PASSWORD")

	default:
		os.Exit(1)
	}

	workerNode := podnetwork.NewWorkerNode(cfg.TunnelType, cfg.HostInterface, cfg.VXLANPort, cfg.VXLANMinID)

	var hypervisorServer hypervisor.Server

	if hypcfg.HypProvider == "ibmcloud" {
		hypervisorServer = registry.NewServer(hypcfg, ibmcfg, workerNode, daemon.DefaultListenPort)
	} else if hypcfg.HypProvider == "aws" {
		hypervisorServer = registry.NewServer(hypcfg, awscfg, workerNode, daemon.DefaultListenPort)
	} else if hypcfg.HypProvider == "libvirt" {
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
