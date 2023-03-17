// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// export LOG_LEVEL="trace|debug"
// export CLOUD_PROVIDER="ibmcloud"
// export TEST_E2E_PROVISION_FILE="/root/provision_ibmcloud.properties"
// export TEST_E2E_PODVM_IMAGE="/root/e2e-test-image-amd64-20230308.qcow2
// export TEST_E2E_PROVISION="yes"
// cd test/e2e
// ../../cluster-provisioner -action=provision | deprovision | uploadimage
// TODO revise provisioner to enable run cluster-provisioner in any folder.
func main() {
	cloudProvider := os.Getenv("CLOUD_PROVIDER")
	provisionPropsFile := os.Getenv("TEST_E2E_PROVISION_FILE")
	podvmImage := os.Getenv("TEST_E2E_PODVM_IMAGE")
	cfg := envconf.New()

	provisioner, err := pv.GetCloudProvisioner(cloudProvider, provisionPropsFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	action := flag.String("action", "provision", "string")
	flag.Parse()

	if *action == "provision" {
		fmt.Println("Creating VPC...")
		if err := provisioner.CreateVPC(context.TODO(), cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Creating Cluster...")
		if err := provisioner.CreateCluster(context.TODO(), cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if podvmImage != "" {
			fmt.Println("Uploading PodVM Image...")
			if _, err := os.Stat(podvmImage); os.IsNotExist(err) {
				fmt.Println(err)
				os.Exit(1)
			}
			if err := provisioner.UploadPodvm(podvmImage, context.TODO(), cfg); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		cloudAPIAdaptor, err := pv.NewCloudAPIAdaptor(cloudProvider)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := cloudAPIAdaptor.Deploy(context.TODO(), cfg, provisioner.GetProperties(context.TODO(), cfg)); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if *action == "deprovision" {
		fmt.Println("Deleting Cluster...")
		if err := provisioner.DeleteCluster(context.TODO(), cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Deleting VPC...")
		if err := provisioner.DeleteVPC(context.TODO(), cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if *action == "uploadimage" {
		fmt.Println("Uploading PodVM Image...")
		if _, err := os.Stat(podvmImage); os.IsNotExist(err) {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := provisioner.UploadPodvm(podvmImage, context.TODO(), cfg); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
