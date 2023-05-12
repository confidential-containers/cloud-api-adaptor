// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"os"

	pv "github.com/confidential-containers/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func init() {
	initLogger()
}

func initLogger() {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	level, err := log.ParseLevel(levelStr)
	if err != nil {
		level = log.InfoLevel
	}

	log.SetLevel(level)
}

func main() {
	cloudProvider := os.Getenv("CLOUD_PROVIDER")
	provisionPropsFile := os.Getenv("TEST_PROVISION_FILE")
	podvmImage := os.Getenv("TEST_PODVM_IMAGE")

	installDirectory := os.Getenv("INSTALL_DIR")
	// If not set assume we are in the test/tools directory
	if installDirectory == "" {
		installDirectory = "../../install"
	}

	cfg := envconf.New()

	provisioner, err := pv.GetCloudProvisioner(cloudProvider, provisionPropsFile)
	if err != nil {
		log.Fatal(err)
	}

	action := flag.String("action", "provision", "string")
	flag.Parse()

	if *action == "provision" {
		log.Info("Creating VPC...")
		if err := provisioner.CreateVPC(context.TODO(), cfg); err != nil {
			log.Fatal(err)
		}

		log.Info("Creating Cluster...")
		if err := provisioner.CreateCluster(context.TODO(), cfg); err != nil {
			log.Fatal(err)
		}

		if podvmImage != "" {
			log.Info("Uploading PodVM Image...")
			if _, err := os.Stat(podvmImage); os.IsNotExist(err) {
				log.Fatal(err)
			}
			if err := provisioner.UploadPodvm(podvmImage, context.TODO(), cfg); err != nil {
				log.Fatal(err)
			}
		}

		cloudAPIAdaptor, err := pv.NewCloudAPIAdaptor(cloudProvider, installDirectory)
		if err != nil {
			log.Fatal(err)
		}
		if err := cloudAPIAdaptor.Deploy(context.TODO(), cfg, provisioner.GetProperties(context.TODO(), cfg)); err != nil {
			log.Fatal(err)
		}
	}

	if *action == "deprovision" {
		log.Info("Deleting Cluster...")
		if err := provisioner.DeleteCluster(context.TODO(), cfg); err != nil {
			log.Fatal(err)
		}

		log.Info("Deleting VPC...")
		if err := provisioner.DeleteVPC(context.TODO(), cfg); err != nil {
			log.Fatal(err)
		}
	}

	if *action == "uploadimage" {
		log.Info("Uploading PodVM Image...")
		if _, err := os.Stat(podvmImage); os.IsNotExist(err) {
			log.Fatal(err)
		}
		if err := provisioner.UploadPodvm(podvmImage, context.TODO(), cfg); err != nil {
			log.Fatal(err)
		}
	}

	if *action == "install" {
		log.Info("Installing CoCo operator and cloud-api-adaptor resources")
		deployer, err := pv.NewCloudAPIAdaptor(cloudProvider, installDirectory)
		if err != nil {
			log.Fatal(err)
		}

		kubeconfigPath := kconf.ResolveKubeConfigFile()
		if kubeconfigPath == "" {
			log.Fatal("Unabled to find a kubeconfig file")
		}
		cfg := envconf.NewWithKubeConfig(kubeconfigPath)

		err = deployer.Deploy(context.TODO(), cfg, provisioner.GetProperties(context.TODO(), cfg))
		if err != nil {
			log.Fatal(err)
		}
	}

	if *action == "uninstall" {
		log.Info("Uninstalling CoCo operator and cloud-api-adaptor resources")
		deployer, err := pv.NewCloudAPIAdaptor(cloudProvider, installDirectory)
		if err != nil {
			log.Fatal(err)
		}

		kubeconfigPath := kconf.ResolveKubeConfigFile()
		if kubeconfigPath == "" {
			log.Fatal("Unabled to find a kubeconfig file")
		}
		cfg := envconf.NewWithKubeConfig(kubeconfigPath)

		err = deployer.Delete(context.TODO(), cfg)
		if err != nil {
			log.Fatal(err)
		}
	}
}
