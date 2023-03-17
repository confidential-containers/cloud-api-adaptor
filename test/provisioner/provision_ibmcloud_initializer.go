//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

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
	log "github.com/sirupsen/logrus"
)

type IBMCloudProperties struct {
	ApiKey          string
	Bucket          string
	ClusterName     string
	CosApiKey       string
	CosInstanceID   string
	CosServiceURL   string
	SecurityGroupID string
	IamServiceURL   string
	InstanceProfile string
	KubeVersion     string
	PodvmImageID    string
	PodvmImageArch  string
	PublicGatewayID string
	Region          string
	ResourceGroupID string
	SshKeyContent   string
	SshKeyID        string
	SshKeyName      string
	SubnetName      string
	SubnetID        string
	VpcName         string
	VpcID           string
	VpcServiceURL   string
	WorkerFlavor    string
	WorkerOS        string
	Zone            string

	WorkerCount   int
	IsSelfManaged bool

	VPC        *vpcv1.VpcV1
	ClusterAPI containerv2.Clusters
}

var IBMCloudProps = &IBMCloudProperties{}

func init() {
	initLogger()
}

func initLogger() {
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

func initProperties(properties map[string]string) error {
	IBMCloudProps = &IBMCloudProperties{
		ApiKey:          properties["APIKEY"],
		Bucket:          properties["COS_BUCKET"],
		ClusterName:     properties["CLUSTER_NAME"],
		CosApiKey:       properties["COS_APIKEY"],
		CosInstanceID:   properties["COS_INSTANCE_ID"],
		CosServiceURL:   properties["COS_SERVICE_URL"],
		IamServiceURL:   properties["IAM_SERVICE_URL"],
		InstanceProfile: properties["INSTANCE_PROFILE_NAME"],
		KubeVersion:     properties["KUBE_VERSION"],
		PodvmImageID:    properties["PODVM_IMAGE_ID"],
		PodvmImageArch:  properties["PODVM_IMAGE_ARCH"],
		Region:          properties["REGION"],
		ResourceGroupID: properties["RESOURCE_GROUP_ID"],
		SshKeyName:      properties["SSH_KEY_NAME"],
		SshKeyContent:   properties["SSH_PUBLIC_KEY_CONTENT"],
		SubnetName:      properties["VPC_SUBNET_NAME"],
		VpcName:         properties["VPC_NAME"],
		VpcServiceURL:   properties["VPC_SERVICE_URL"],
		WorkerFlavor:    properties["WORKER_FLAVOR"],
		WorkerOS:        properties["WORKER_OPERATION_SYSTEM"],
		Zone:            properties["ZONE"],
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

	log.Debugf("%+v", IBMCloudProps)

	if len(IBMCloudProps.ApiKey) <= 0 {
		return errors.New("APIKEY was not set.")
	}
	if len(IBMCloudProps.Region) <= 0 {
		return errors.New("REGION was not set.")
	}
	if len(IBMCloudProps.KubeVersion) <= 0 {
		return errors.New("KUBE_VERSION was not set, get it via command: ibmcloud cs versions")
	}
	if len(IBMCloudProps.WorkerOS) <= 0 {
		return errors.New("WORKER_OPERATION_SYSTEM was not set, set it like: UBUNTU_20_64, UBUNTU_18_S390X")
	}

	// IAM_SERVICE_URL can overwrite default IamServiceURL, for example: IAM_SERVICE_URL="https://iam.test.cloud.ibm.com/identity/token"
	if len(IBMCloudProps.IamServiceURL) <= 0 {
		IBMCloudProps.IamServiceURL = "https://iam.cloud.ibm.com/identity/token"
	}
	log.Infof("IamServiceURL is: %s.", IBMCloudProps.IamServiceURL)

	// VPC_SERVICE_URL can overwrite default VpcServiceURL https://{REGION}.iaas.cloud.ibm.com/v1, for example: VPC_SERVICE_URL="https://jp-tok.iaas.test.cloud.ibm.com/v1"
	if len(IBMCloudProps.VpcServiceURL) <= 0 {
		IBMCloudProps.VpcServiceURL = "https://" + IBMCloudProps.Region + ".iaas.cloud.ibm.com/v1"
	}
	log.Infof("VpcServiceURL is: %s.", IBMCloudProps.VpcServiceURL)

	needProvisionStr := os.Getenv("TEST_E2E_PROVISION")
	if strings.EqualFold(needProvisionStr, "yes") || strings.EqualFold(needProvisionStr, "true") {
		if len(IBMCloudProps.ResourceGroupID) <= 0 {
			return errors.New("RESOURCE_GROUP_ID was not set.")
		}
		if len(IBMCloudProps.Zone) <= 0 {
			return errors.New("ZONE was not set.")
		}

		if err := initClustersAPI(); err != nil {
			return err
		}
	}

	podvmImage := os.Getenv("TEST_E2E_PODVM_IMAGE")
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
	}

	if err := initVpcV1(); err != nil {
		return err
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

	cfg := &bx.Config{
		BluemixAPIKey: IBMCloudProps.ApiKey,
		Region:        IBMCloudProps.Region,
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
