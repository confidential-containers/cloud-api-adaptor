// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
)

// export DEBUG="true"
// export CLOUD_PROVIDER="ibmcloud"
// export TEST_E2E_PROVISION_FILE="/root/provision_ibmcloud.properties"
// export TEST_E2E_PODVM_IMAGE="/root/e2e-test-image-amd64-20230308.qcow2
// export TEST_E2E_PROVISION="yes"
// ./cluster-provisioner -action=provision | uploadimage
func main() {
	cloudProvider := os.Getenv("CLOUD_PROVIDER")
	provisionPropsFile := os.Getenv("TEST_E2E_PROVISION_FILE")

	provisioner, err := pv.GetCloudProvisioner(cloudProvider, provisionPropsFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	action := flag.String("action", "provision", "string")
	flag.Parse()

	if *action == "provision" {
		if err := provisioner.CreateVPC(context.TODO(), nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := provisioner.CreateCluster(context.TODO(), nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// The design is when cluster/vpc/subnet name provided, we'll create them and then delete them after e2e test.
	// While if only vpc/subnet ID provided, it means the e2e test is using exiting one, we should not delete them after the test.
	// Looks the algorithm does not align with the cluster-provisioner CLI.
	//if *action == "deprovision" {
	//	if err := provisioner.DeleteCluster(context.TODO(), nil); err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}
	//
	//	if err := provisioner.DeleteVPC(context.TODO(), nil); err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}
	//
	//}

	if *action == "uploadimage" {
		podvmImage := os.Getenv("TEST_E2E_PODVM_IMAGE")
		if _, err := os.Stat(podvmImage); os.IsNotExist(err) {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := provisioner.UploadPodvm(podvmImage, context.TODO(), nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
