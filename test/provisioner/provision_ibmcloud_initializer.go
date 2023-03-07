// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	bx "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	bxsession "github.com/IBM-Cloud/bluemix-go/session"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
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
	IksVersion      string
	InstanceProfile string
	PodvmImageID    string
	PodvmImageArch  string
	Region          string
	ResourceGroupID string
	SshKeyID        string
	SubnetName      string
	SubnetID        string
	VpcName         string
	VpcID           string
	VpcServiceURL   string
	WorkerFlavor    string
	Zone            string

	WorkerCount     int
	IsSelfManaged   bool
	IsProvNewVPC    bool
	IsProvNewSubnet bool
	IsDebug         bool

	VPC        *vpcv1.VpcV1
	ClusterAPI containerv2.Clusters
}

var IBMCloudProps = &IBMCloudProperties{}

func initProperties(properties map[string]string) error {
	IBMCloudProps = &IBMCloudProperties{
		ApiKey:        properties["APIKEY"],
		Bucket:        properties["COS_BUCKET"],
		ClusterName:   properties["CLUSTER_NAME"],
		CosApiKey:     properties["COS_APIKEY"],
		CosInstanceID: properties["COS_INSTANCE_ID"],
		CosServiceURL: properties["COS_SERVICE_URL"],
		IamServiceURL: properties["IAM_SERVICE_URL"],
		IksVersion:    properties["IKS_VERSION"],
		// IsSelfManaged    : properties["IS_SELF_MANAGED_CLUSTER"]
		InstanceProfile: properties["INSTANCE_PROFILE_NAME"],
		PodvmImageID:    properties["PODVM_IMAGE_ID"],
		PodvmImageArch:  properties["PODVM_IMAGE_ARCH"],
		Region:          properties["REGION"],
		ResourceGroupID: properties["RESOURCE_GROUP_ID"],
		SecurityGroupID: properties["SECURITY_GROUP_ID"],
		SshKeyID:        properties["SSH_KEY_ID"],
		SubnetName:      properties["VPC_SUBNET_NAME"],
		SubnetID:        properties["VPC_SUBNET_ID"],
		VpcName:         properties["VPC_NAME"],
		VpcID:           properties["VPC_ID"],
		VpcServiceURL:   properties["VPC_SERVICE_URL"],
		WorkerFlavor:    properties["WORKER_FLAVOR"],
		// WorkerCount   : properties["WORKERS_COUNT"]
		Zone: properties["ZONE"],
	}

	if len(IBMCloudProps.ClusterName) <= 0 {
		IBMCloudProps.ClusterName = "e2e-test-cluster"
	}
	if len(IBMCloudProps.VpcName) <= 0 && len(IBMCloudProps.VpcID) <= 0 {
		IBMCloudProps.IsProvNewVPC = true
		IBMCloudProps.VpcName = IBMCloudProps.ClusterName + "-vpc"
	}
	if len(IBMCloudProps.SubnetName) <= 0 && len(IBMCloudProps.SubnetID) <= 0 {
		IBMCloudProps.IsProvNewSubnet = true
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

	debugStr := os.Getenv("DEBUG")
	if strings.EqualFold(debugStr, "yes") || strings.EqualFold(debugStr, "true") {
		IBMCloudProps.IsDebug = true
	}
	if IBMCloudProps.IsDebug {
		fmt.Printf("%+v\n", IBMCloudProps)
	}

	if len(IBMCloudProps.ApiKey) <= 0 {
		return errors.New("APIKEY was not set.")
	}
	if len(IBMCloudProps.IamServiceURL) <= 0 {
		return errors.New("IAM_SERVICE_URL was not set, example: https://iam.cloud.ibm.com/identity/token")
	}
	if len(IBMCloudProps.VpcServiceURL) <= 0 {
		return errors.New("VPC_SERVICE_URL was not set, example: https://us-south.iaas.cloud.ibm.com/v1")
	}

	needProvisionStr := os.Getenv("TEST_E2E_PROVISION")
	if strings.EqualFold(needProvisionStr, "yes") || strings.EqualFold(needProvisionStr, "true") {
		if len(IBMCloudProps.ResourceGroupID) <= 0 {
			return errors.New("RESOURCE_GROUP_ID was not set.")
		}
		if len(IBMCloudProps.Region) <= 0 {
			return errors.New("REGION was not set.")
		}
		if len(IBMCloudProps.Zone) <= 0 {
			return errors.New("ZONE was not set.")
		}
		if len(IBMCloudProps.IksVersion) <= 0 {
			return errors.New("IKS_VERSION was not set.")
		}

		if err := initClustersAPI(); err != nil {
			return err
		}
	}

	podvmImage := os.Getenv("TEST_E2E_PODVM_IMAGE")
	if len(podvmImage) >= 0 {
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
	ibmcloudTrace("initVpcV1()")

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
	ibmcloudTrace("initClustersAPI()")

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
