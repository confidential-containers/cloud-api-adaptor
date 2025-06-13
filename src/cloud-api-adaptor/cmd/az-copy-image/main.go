// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/google/uuid"
)

func defaultSubscriptionID(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "az", "account", "show", "--query", "id", "-o", "tsv").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func groupLocation(ctx context.Context, group string) (string, error) {
	out, err := exec.CommandContext(ctx, "az", "group", "show", "-n", group, "--query", "location", "-o", "tsv").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func main() {
	var (
		subscriptionID        string
		resourceGroup         string
		location              string
		imageGallery          string
		imageDefinition       string
		imageVersion          string
		communityGalleryImage string
		targetRegionsStr      string
	)

	flag.StringVar(&subscriptionID, "subscription-id", os.Getenv("AZURE_SUBSCRIPTION_ID"), "Azure subscription ID")
	flag.StringVar(&resourceGroup, "resource-group", os.Getenv("AZURE_RESOURCE_GROUP"), "Resource group name")
	flag.StringVar(&location, "location", os.Getenv("AZURE_LOCATION"), "Azure location")
	flag.StringVar(&imageGallery, "image-gallery", "", "Image gallery name")
	flag.StringVar(&imageDefinition, "image-definition", "", "Image definition name")
	flag.StringVar(&imageVersion, "image-version", "0.0.1", "Image version (major.minor.patch)")
	flag.StringVar(&communityGalleryImage, "community-image-id", "", "Community gallery image version resource ID")
	flag.StringVar(&targetRegionsStr, "target-regions", "eastus2,northeurope,westeurope,eastus", "Comma separated target regions")

	flag.Parse()

	re := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !re.MatchString(imageVersion) {
		log.Fatalf("–version %q is invalid; expected something like 1.2.3", imageVersion)
	}

	ctx := context.Background()

	if subscriptionID == "" {
		var err error
		subscriptionID, err = defaultSubscriptionID(ctx)
		if err != nil {
			log.Fatalf("cannot determine subscription ID: %v", err)
		}
	}

	if resourceGroup == "" || communityGalleryImage == "" {
		fmt.Fprintln(os.Stderr, "required parameters: resource-group,community-image-id,image-gallery,image-definition")
		os.Exit(1)
	}

	if location == "" {
		var err error
		location, err = groupLocation(ctx, resourceGroup)
		if err != nil {
			log.Fatalf("cannot determine location: %v", err)
		}
	}

	managedDiskName := "tmp-disk-" + uuid.New().String()
	userImageName := "tmp-img-" + uuid.New().String()

	regions := strings.Split(targetRegionsStr, ",")
	var targets []*armcompute.TargetRegion
	for _, r := range regions {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		targets = append(targets, &armcompute.TargetRegion{Name: to.Ptr(r)})
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("creating credential: %v", err)
	}

	diskClient, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating disks client: %v", err)
	}

	log.Printf("creating managed disk %s from community image", managedDiskName)
	diskPoller, err := diskClient.BeginCreateOrUpdate(ctx, resourceGroup, managedDiskName, armcompute.Disk{
		Location: to.Ptr(location),
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionFromImage),
				GalleryImageReference: &armcompute.ImageDiskReference{
					CommunityGalleryImageID: to.Ptr(communityGalleryImage),
				},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating disk: %v", err)
	}
	if _, err = diskPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for disk: %v", err)
	}

	defer func() {
		log.Printf("deleting temporary managed disk %s", managedDiskName)
		delDiskPoller, err := diskClient.BeginDelete(ctx, resourceGroup, managedDiskName, nil)
		if err == nil {
			_, _ = delDiskPoller.PollUntilDone(ctx, nil)
		}
	}()

	diskID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", subscriptionID, resourceGroup, managedDiskName)

	imagesClient, err := armcompute.NewImagesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating images client: %v", err)
	}

	log.Printf("creating managed image %s from managed disk", userImageName)
	imgPoller, err := imagesClient.BeginCreateOrUpdate(ctx, resourceGroup, userImageName, armcompute.Image{
		Location: to.Ptr(location),
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:      to.Ptr(armcompute.OperatingSystemTypesLinux),
					OSState:     to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
					ManagedDisk: &armcompute.SubResource{ID: to.Ptr(diskID)},
				},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating managed image: %v", err)
	}
	if _, err = imgPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for managed image: %v", err)
	}

	defer func() {
		log.Printf("deleting temporary managed image %s", userImageName)
		delImgPoller, err := imagesClient.BeginDelete(ctx, resourceGroup, userImageName, nil)
		if err == nil {
			_, _ = delImgPoller.PollUntilDone(ctx, nil)
		}
	}()

	imageID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s", subscriptionID, resourceGroup, userImageName)

	galleriesClient, err := armcompute.NewGalleriesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating galleries client: %v", err)
	}

	galPoller, err := galleriesClient.BeginCreateOrUpdate(ctx, resourceGroup, imageGallery, armcompute.Gallery{Location: to.Ptr(location)}, nil)
	if err != nil {
		log.Fatalf("creating gallery: %v", err)
	}
	if _, err = galPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for gallery: %v", err)
	}

	imgDefClient, err := armcompute.NewGalleryImagesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating gallery images client: %v", err)
	}
	imgDefPoller, err := imgDefClient.BeginCreateOrUpdate(ctx, resourceGroup, imageGallery, imageDefinition, armcompute.GalleryImage{
		Location: to.Ptr(location),
		Properties: &armcompute.GalleryImageProperties{
			OSType:           to.Ptr(armcompute.OperatingSystemTypesLinux),
			OSState:          to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: to.Ptr("cvm-publisher"),
				Offer:     to.Ptr("cvm-offer"),
				SKU:       to.Ptr("cvm-sku"),
			},
			Features: []*armcompute.GalleryImageFeature{{
				Name:  to.Ptr("SecurityType"),
				Value: to.Ptr("ConfidentialVmSupported"),
			}},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating image definition: %v", err)
	}
	if _, err = imgDefPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for image definition: %v", err)
	}

	versionClient, err := armcompute.NewGalleryImageVersionsClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("creating gallery image versions client: %v", err)
	}
	log.Printf("creating gallery image version %s from managed image", imageVersion)
	verPoller, err := versionClient.BeginCreateOrUpdate(ctx, resourceGroup, imageGallery, imageDefinition, imageVersion, armcompute.GalleryImageVersion{
		Location: to.Ptr(location),
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{TargetRegions: targets},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				Source: &armcompute.GalleryArtifactVersionFullSource{ID: to.Ptr(imageID)},
			},
		},
	}, nil)
	if err != nil {
		log.Fatalf("creating image version: %v", err)
	}
	if _, err = verPoller.PollUntilDone(ctx, nil); err != nil {
		log.Fatalf("waiting for image version: %v", err)
	}

	log.Printf("image %s/%s/%s created successfully", imageGallery, imageDefinition, imageVersion)
}
