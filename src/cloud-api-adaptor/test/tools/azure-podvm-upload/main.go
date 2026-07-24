// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

// azure-podvm-upload is a temporary CLI that exercises
// azure.UploadPodvmImageToGallery. It converts a podvm qcow2 image to a
// fixed-format VHD, uploads it to an Azure page blob, and registers a
// gallery image version pointing at it.
//
// Required environment variables:
//
//	AZURE_SUBSCRIPTION_ID
//	AZURE_RESOURCE_GROUP
//	AZURE_LOCATION
//	AZURE_STORAGE_ACCOUNT
//	AZURE_STORAGE_CONTAINER
//	AZURE_BLOB_NAME
//	AZURE_GALLERY_NAME
//	AZURE_GALLERY_IMAGE_DEFINITION
//	AZURE_GALLERY_IMAGE_VERSION
//
// Optional:
//
//	AZURE_REPLICATION_REGIONS (comma-separated; overridden by -replication-regions)
//
// Required flag:
//
//	-image <path-to-podvm.qcow2>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/azure"
	log "github.com/sirupsen/logrus"
)

type config struct {
	qcow2Path          string
	subscriptionID     string
	resourceGroup      string
	location           string
	storageAccountName string
	storageContainer   string
	blobName           string
	galleryName        string
	imageDefinition    string
	imageVersion       string
	replicationRegions []string
}

func loadConfig() (*config, error) {
	imagePath := flag.String("image", "", "path to the podvm qcow2 image (required)")
	replicationRegionsFlag := flag.String("replication-regions", "", "comma-separated list of additional Azure regions to replicate the gallery image version to (the home location is always included); overrides AZURE_REPLICATION_REGIONS")
	flag.Parse()

	regionsRaw := *replicationRegionsFlag
	if regionsRaw == "" {
		regionsRaw = os.Getenv("AZURE_REPLICATION_REGIONS")
	}
	var regions []string
	for _, r := range strings.Split(regionsRaw, ",") {
		if r = strings.TrimSpace(r); r != "" {
			regions = append(regions, r)
		}
	}

	cfg := &config{
		qcow2Path:          *imagePath,
		subscriptionID:     os.Getenv("AZURE_SUBSCRIPTION_ID"),
		resourceGroup:      os.Getenv("AZURE_RESOURCE_GROUP"),
		location:           os.Getenv("AZURE_LOCATION"),
		storageAccountName: os.Getenv("AZURE_STORAGE_ACCOUNT"),
		storageContainer:   os.Getenv("AZURE_STORAGE_CONTAINER"),
		blobName:           os.Getenv("AZURE_BLOB_NAME"),
		galleryName:        os.Getenv("AZURE_GALLERY_NAME"),
		imageDefinition:    os.Getenv("AZURE_GALLERY_IMAGE_DEFINITION"),
		imageVersion:       os.Getenv("AZURE_GALLERY_IMAGE_VERSION"),
		replicationRegions: regions,
	}

	required := map[string]string{
		"-image":                         cfg.qcow2Path,
		"AZURE_SUBSCRIPTION_ID":          cfg.subscriptionID,
		"AZURE_RESOURCE_GROUP":           cfg.resourceGroup,
		"AZURE_LOCATION":                 cfg.location,
		"AZURE_STORAGE_ACCOUNT":          cfg.storageAccountName,
		"AZURE_STORAGE_CONTAINER":        cfg.storageContainer,
		"AZURE_BLOB_NAME":                cfg.blobName,
		"AZURE_GALLERY_NAME":             cfg.galleryName,
		"AZURE_GALLERY_IMAGE_DEFINITION": cfg.imageDefinition,
		"AZURE_GALLERY_IMAGE_VERSION":    cfg.imageVersion,
	}
	for name, value := range required {
		if value == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}

	if _, err := os.Stat(cfg.qcow2Path); err != nil {
		return nil, fmt.Errorf("stat image %s: %w", cfg.qcow2Path, err)
	}
	return cfg, nil
}

func main() {
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		if lvl, err := log.ParseLevel(levelStr); err == nil {
			log.SetLevel(lvl)
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Errorf("invalid configuration: %v", err)
		flag.Usage()
		os.Exit(2)
	}

	id, err := azure.UploadPodvmImageToGallery(
		context.Background(),
		cfg.qcow2Path,
		cfg.subscriptionID, cfg.location, cfg.resourceGroup,
		cfg.storageAccountName, cfg.storageContainer, cfg.blobName,
		cfg.galleryName, cfg.imageDefinition, cfg.imageVersion,
		cfg.replicationRegions,
	)
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	log.Infof("Gallery image version created: %s", id)
	fmt.Println(id)
}
