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

func main() {
	cloudProvider := os.Getenv("CLOUD_PROVIDER")
	provisionPropsFile := os.Getenv("TEST_E2E_PROVISION_FILE")

	provisioner, _ := pv.GetCloudProvisioner(cloudProvider, provisionPropsFile)

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

	if *action == "deprovision" {
		if err := provisioner.DeleteCluster(context.TODO(), nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := provisioner.DeleteVPC(context.TODO(), nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	}

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
