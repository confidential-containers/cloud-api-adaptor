// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"errors"
	"os"
	"strconv"
	"strings"

	bx "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	bxsession "github.com/IBM-Cloud/bluemix-go/session"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
)

type IBMCloudProperties struct {
	IBMCloudProvider      string
	ApiKey                string
	IamProfileID          string
	Bucket                string
	CaaImageTag           string
	ClusterName           string
	ContainerRuntime      string
	CosApiKey             string
	CosInstanceID         string
	CosServiceURL         string
	SecurityGroupID       string
	IamServiceURL         string
	IksServiceURL         string
	InitData              string
	InstanceProfile       string
	KubeVersion           string
	PodvmImageID          string
	PodvmImageArch        string
	PublicGatewayName     string
	PublicGatewayID       string
	Region                string
	ResourceGroupID       string
	SshKeyContent         string
	SshKeyID              string
	SshKeyName            string
	SubnetName            string
	SubnetID              string
	VpcName               string
	VpcID                 string
	VpcServiceURL         string
	WorkerFlavor          string
	WorkerOS              string
	Zone                  string
	TunnelType            string
	VxlanPort             string
	ClusterID             string
	Tags                  string
	DedicatedHostIDs      string
	DedicatedHostGroupIDs string

	WorkerCount   int
	IsSelfManaged bool
	DisableCVM    bool

	VPC        *vpcv1.VpcV1
	ClusterAPI containerv2.Clusters
}

var IBMCloudProps = &IBMCloudProperties{}

func InitIBMCloudProperties(properties map[string]string) error {
	containerRuntime := "crio"
	if properties["CONTAINER_RUNTIME"] != "" {
		containerRuntime = properties["CONTAINER_RUNTIME"]
	}

	IBMCloudProps = &IBMCloudProperties{
		IBMCloudProvider:      properties["IBMCLOUD_PROVIDER"],
		ApiKey:                properties["APIKEY"],
		IamProfileID:          properties["IAM_PROFILE_ID"],
		Bucket:                properties["COS_BUCKET"],
		CaaImageTag:           properties["CAA_IMAGE_TAG"],
		ClusterName:           properties["CLUSTER_NAME"],
		ContainerRuntime:      containerRuntime,
		CosApiKey:             properties["COS_APIKEY"],
		CosInstanceID:         properties["COS_INSTANCE_ID"],
		CosServiceURL:         properties["COS_SERVICE_URL"],
		IamServiceURL:         properties["IAM_SERVICE_URL"],
		IksServiceURL:         properties["IKS_SERVICE_URL"],
		InitData:              properties["INITDATA"],
		InstanceProfile:       properties["INSTANCE_PROFILE_NAME"],
		KubeVersion:           properties["KUBE_VERSION"],
		PodvmImageID:          properties["PODVM_IMAGE_ID"],
		PodvmImageArch:        properties["PODVM_IMAGE_ARCH"],
		PublicGatewayName:     properties["PUBLIC_GATEWAY_NAME"],
		Region:                properties["REGION"],
		ResourceGroupID:       properties["RESOURCE_GROUP_ID"],
		SshKeyName:            properties["SSH_KEY_NAME"],
		SshKeyContent:         properties["SSH_PUBLIC_KEY_CONTENT"],
		SubnetName:            properties["VPC_SUBNET_NAME"],
		VpcName:               properties["VPC_NAME"],
		VpcServiceURL:         properties["VPC_SERVICE_URL"],
		WorkerFlavor:          properties["WORKER_FLAVOR"],
		WorkerOS:              properties["WORKER_OPERATION_SYSTEM"],
		Zone:                  properties["ZONE"],
		SshKeyID:              properties["SSH_KEY_ID"],
		SubnetID:              properties["VPC_SUBNET_ID"],
		SecurityGroupID:       properties["VPC_SECURITY_GROUP_ID"],
		VpcID:                 properties["VPC_ID"],
		TunnelType:            properties["TUNNEL_TYPE"],
		VxlanPort:             properties["VXLAN_PORT"],
		ClusterID:             properties["CLUSTER_ID"],
		Tags:                  properties["TAGS"],
		DedicatedHostIDs:      properties["DEDICATED_HOST_IDS"],
		DedicatedHostGroupIDs: properties["DEDICATED_HOST_GROUP_IDS"],
	}

	if len(IBMCloudProps.IBMCloudProvider) <= 0 {
		IBMCloudProps.IBMCloudProvider = "ibmcloud"
	}
	if len(IBMCloudProps.ClusterName) <= 0 {
		IBMCloudProps.ClusterName = "e2e-test-cluster"
	}
	if len(IBMCloudProps.VpcName) <= 0 && len(IBMCloudProps.VpcID) <= 0 {
		IBMCloudProps.VpcName = IBMCloudProps.ClusterName + "-vpc"
	}
	if len(IBMCloudProps.SubnetName) <= 0 && len(IBMCloudProps.SubnetID) <= 0 {
		IBMCloudProps.SubnetName = IBMCloudProps.VpcName + "-subnet"
	}
	if len(IBMCloudProps.PublicGatewayName) <= 0 && len(IBMCloudProps.PublicGatewayID) <= 0 {
		IBMCloudProps.PublicGatewayName = IBMCloudProps.VpcName + "-gateway"
	}
	if len(IBMCloudProps.InstanceProfile) <= 0 {
		IBMCloudProps.InstanceProfile = "bx2-2x8"
	}
	if len(IBMCloudProps.WorkerFlavor) <= 0 {
		IBMCloudProps.WorkerFlavor = "bx2.2x8"
	}

	workerCountStr := properties["WORKERS_COUNT"]
	if len(workerCountStr) <= 0 {
		IBMCloudProps.WorkerCount = 1
	} else {
		count, err := strconv.Atoi(workerCountStr)
		if err != nil {
			IBMCloudProps.WorkerCount = 1
		} else {
			IBMCloudProps.WorkerCount = count
		}
	}
	selfManagedStr := properties["IS_SELF_MANAGED_CLUSTER"]
	if strings.EqualFold(selfManagedStr, "yes") || strings.EqualFold(selfManagedStr, "true") {
		IBMCloudProps.IsSelfManaged = true
	}
	confidentialComputingStr := properties["DISABLECVM"]
	if strings.EqualFold(confidentialComputingStr, "yes") || strings.EqualFold(confidentialComputingStr, "true") {
		IBMCloudProps.DisableCVM = true
	}

	if len(IBMCloudProps.ResourceGroupID) <= 0 {
		log.Info("[warning] RESOURCE_GROUP_ID was not set.")
	}
	if len(IBMCloudProps.Zone) <= 0 {
		log.Info("[warning] ZONE was not set.")
	}

	// IAM_SERVICE_URL can overwrite default IamServiceURL, for example: IAM_SERVICE_URL="https://iam.test.cloud.ibm.com/identity/token"
	if len(IBMCloudProps.IamServiceURL) <= 0 {
		IBMCloudProps.IamServiceURL = "https://iam.cloud.ibm.com/identity/token"
	}
	log.Infof("IamServiceURL is: %s.", IBMCloudProps.IamServiceURL)

	// VPC_SERVICE_URL can overwrite default VpcServiceURL https://{REGION}.iaas.cloud.ibm.com/v1, for example: VPC_SERVICE_URL="https://jp-tok.iaas.test.cloud.ibm.com/v1"
	if len(IBMCloudProps.VpcServiceURL) <= 0 {
		if len(IBMCloudProps.Region) > 0 {
			IBMCloudProps.VpcServiceURL = "https://" + IBMCloudProps.Region + ".iaas.cloud.ibm.com/v1"
		} else {
			log.Info("[warning] REGION was not set.")
		}
	}
	log.Infof("VpcServiceURL is: %s.", IBMCloudProps.VpcServiceURL)

	// IKS_SERVICE_URL can overwrite the default IksServiceURL IKS_SERVICE_URL=https://containers.cloud.ibm.com/global, for example IKS_SERVICE_URL="https://containers.test.cloud.ibm.com/global"
	if len(IBMCloudProps.IksServiceURL) <= 0 {
		IBMCloudProps.IksServiceURL = "https://containers.cloud.ibm.com/global"
	}
	log.Infof("IksServiceURL is: %s.", IBMCloudProps.IksServiceURL)

	needProvisionStr := os.Getenv("TEST_PROVISION")
	if strings.EqualFold(needProvisionStr, "yes") || strings.EqualFold(needProvisionStr, "true") || pv.Action == "uploadimage" {
		if len(IBMCloudProps.ApiKey) <= 0 {
			return errors.New("APIKEY is required for provisioning")
		}
		if len(IBMCloudProps.Region) <= 0 {
			return errors.New("REGION was not set.")
		}
	}

	if strings.EqualFold(needProvisionStr, "yes") || strings.EqualFold(needProvisionStr, "true") {
		if len(IBMCloudProps.KubeVersion) <= 0 {
			return errors.New("KUBE_VERSION was not set, get it via command: ibmcloud cs versions")
		}
		if len(IBMCloudProps.WorkerOS) <= 0 {
			return errors.New("WORKER_OPERATION_SYSTEM was not set, set it like: UBUNTU_20_64, UBUNTU_18_S390X")
		}
	} else {
		if len(IBMCloudProps.SshKeyID) <= 0 {
			log.Info("[warning] SSH_KEY_ID was not set.")
		}
		if len(IBMCloudProps.SubnetID) <= 0 {
			log.Info("[warning] VPC_SUBNET_ID was not set.")
		}
		if len(IBMCloudProps.SecurityGroupID) <= 0 {
			log.Info("[warning] VPC_SECURITY_GROUP_ID was not set.")
		}
		if len(IBMCloudProps.VpcID) <= 0 {
			log.Info("[warning] VPC_ID was not set.")
		}
	}

	podvmImage := os.Getenv("TEST_PODVM_IMAGE")
	if len(podvmImage) > 0 {
		if len(IBMCloudProps.CosApiKey) <= 0 {
			return errors.New("COS_APIKEY was not set.")
		}
		if len(IBMCloudProps.CosInstanceID) <= 0 {
			return errors.New("COS_INSTANCE_ID was not set.")
		}
		if len(IBMCloudProps.Bucket) <= 0 {
			return errors.New("COS_BUCKET was not set.")
		}
		if len(IBMCloudProps.CosServiceURL) <= 0 {
			return errors.New("COS_SERVICE_URL was not set, example: s3.us.cloud-object-storage.appdomain.cloud")
		}
	} else if len(IBMCloudProps.PodvmImageID) <= 0 {
		return errors.New("PODVM_IMAGE_ID was not set, set it with existing custom image id in VPC")
	}

	if len(IBMCloudProps.ApiKey) <= 0 && len(IBMCloudProps.IamProfileID) <= 0 {
		return errors.New("APIKEY or IAM_PROFILE_ID must be set")
	}

	if len(IBMCloudProps.ApiKey) > 0 {
		if err := initClustersAPI(); err != nil {
			return err
		}

		if err := initVpcV1(); err != nil {
			return err
		}
	}

	return nil
}

func initVpcV1() error {
	log.Trace("initVpcV1()")

	if IBMCloudProps.VPC != nil {
		return nil
	}

	vpcService, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: IBMCloudProps.ApiKey,
			URL:    IBMCloudProps.IamServiceURL,
		},
		URL: IBMCloudProps.VpcServiceURL,
	})

	if err != nil {
		return err
	}
	IBMCloudProps.VPC = vpcService

	return nil
}

func initClustersAPI() error {
	log.Trace("initClustersAPI()")

	iamServiceURLParts := strings.Split(IBMCloudProps.IamServiceURL, "/")
	if len(iamServiceURLParts) < 3 || len(iamServiceURLParts[1]) != 0 {
		return errors.New("IAM service endpoint is malformed")
	}

	tokenProviderEndpoint := iamServiceURLParts[0] + "//" + iamServiceURLParts[2]
	log.Tracef("IAM token provider endpoint for bx config is %s.", tokenProviderEndpoint)

	cfg := &bx.Config{
		BluemixAPIKey:         IBMCloudProps.ApiKey,
		Region:                IBMCloudProps.Region,
		Endpoint:              &IBMCloudProps.IksServiceURL,
		TokenProviderEndpoint: &tokenProviderEndpoint,
	}

	sess, err := bxsession.New(cfg)
	if err != nil {
		return err
	}
	clusterClient, err := containerv2.New(sess)
	if err != nil {
		return err
	}
	IBMCloudProps.ClusterAPI = clusterClient.Clusters()

	return nil
}
