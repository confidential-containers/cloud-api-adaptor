// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/pageblob"
	log "github.com/sirupsen/logrus"
)

const (
	mib                = 1024 * 1024
	pageBlobChunkSize  = 4 * mib
	defaultStorageHost = "blob.core.windows.net"
)

// ConvertQcow2ToRaw converts a qcow2 image to raw using qemu-img.
func ConvertQcow2ToRaw(qcow2, raw string) error {
	cmd := exec.Command("qemu-img", "convert", "-O", "raw", qcow2, raw)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("converting %s to raw: %w", qcow2, err)
	}
	return nil
}

// ResizeRawImageToMiB rounds the raw image up to the nearest MiB boundary.
// Azure expects fixed-format VHDs aligned to MiB before conversion.
func ResizeRawImageToMiB(rawPath string) error {
	type qemuImgInfo struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	out, err := exec.Command("qemu-img", "info", "-f", "raw", "--output", "json", rawPath).Output()
	if err != nil {
		return fmt.Errorf("inspecting raw image %s: %w", rawPath, err)
	}
	var info qemuImgInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return fmt.Errorf("parsing qemu-img info output: %w", err)
	}

	rounded := ((info.VirtualSize + mib - 1) / mib) * mib
	if info.VirtualSize >= rounded {
		log.Infof("Raw image already MiB-aligned: %d bytes", info.VirtualSize)
		return nil
	}

	log.Infof("Resizing raw image %s from %d to %d bytes", rawPath, info.VirtualSize, rounded)
	cmd := exec.Command("qemu-img", "resize", "-f", "raw", rawPath, fmt.Sprintf("%d", rounded))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("resizing raw image to %d bytes: %w", rounded, err)
	}
	return nil
}

// ConvertRawToVHD converts a MiB-aligned raw image to a fixed-format VHD using qemu-img.
func ConvertRawToVHD(raw, vhd string) error {
	cmd := exec.Command("qemu-img", "convert",
		"-f", "raw",
		"-o", "subformat=fixed,force_size",
		"-O", "vpc",
		raw, vhd,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("converting %s to vhd: %w", raw, err)
	}
	return nil
}

// UploadVHDToPageBlob uploads a fixed-format VHD file to an Azure page blob,
// creating the container if it does not already exist. All-zero chunks are
// skipped to avoid wasted requests. Returns the resulting blob URL.
func UploadVHDToPageBlob(ctx context.Context, cred azcore.TokenCredential, accountName, containerName, blobName, vhdPath string) (string, error) {
	f, err := os.Open(vhdPath)
	if err != nil {
		return "", fmt.Errorf("opening vhd %s: %w", vhdPath, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat vhd %s: %w", vhdPath, err)
	}
	size := stat.Size()
	if size%512 != 0 {
		return "", fmt.Errorf("vhd size %d is not a multiple of 512 bytes", size)
	}

	containerURL := fmt.Sprintf("https://%s.%s/%s", accountName, defaultStorageHost, containerName)
	containerClient, err := container.NewClient(containerURL, cred, nil)
	if err != nil {
		return "", fmt.Errorf("creating container client: %w", err)
	}
	if _, err := containerClient.Create(ctx, nil); err != nil {
		var respErr *azcore.ResponseError
		if !errors.As(err, &respErr) || respErr.ErrorCode != string(bloberror.ContainerAlreadyExists) {
			return "", fmt.Errorf("creating container %q: %w", containerName, err)
		}
	}

	blobURL := fmt.Sprintf("%s/%s", containerURL, blobName)
	pbClient, err := pageblob.NewClient(blobURL, cred, nil)
	if err != nil {
		return "", fmt.Errorf("creating page blob client: %w", err)
	}

	log.Infof("Creating page blob %s (size: %d bytes)", blobURL, size)
	if _, err := pbClient.Create(ctx, size, nil); err != nil {
		return "", fmt.Errorf("creating page blob: %w", err)
	}

	buf := make([]byte, pageBlobChunkSize)
	var offset int64
	for offset < size {
		toRead := int64(pageBlobChunkSize)
		if remaining := size - offset; remaining < toRead {
			toRead = remaining
		}
		n, err := io.ReadFull(f, buf[:toRead])
		if err != nil {
			return "", fmt.Errorf("reading vhd at offset %d: %w", offset, err)
		}
		if !isAllZero(buf[:n]) {
			body := streaming.NopCloser(bytes.NewReader(buf[:n]))
			rng := blob.HTTPRange{Offset: offset, Count: int64(n)}
			if _, err := pbClient.UploadPages(ctx, body, rng, nil); err != nil {
				return "", fmt.Errorf("uploading pages at offset %d: %w", offset, err)
			}
		}
		offset += int64(n)
	}

	log.Infof("Uploaded VHD to %s", blobURL)
	return blobURL, nil
}

func isAllZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// CreateGalleryImageVersionFromVHD creates a gallery image version backed by
// the given VHD blob and returns the resource ID of the new image version.
//
// replicationRegions is the set of regions the image version should be
// replicated to. The home `location` is always included; duplicates are
// de-duplicated (case-insensitive).
func CreateGalleryImageVersionFromVHD(
	ctx context.Context,
	cred azcore.TokenCredential,
	subscriptionID, location, resourceGroup, galleryName, imageDefinition, version string,
	storageAccountID, vhdBlobURL string,
	replicationRegions []string,
) (string, error) {
	client, err := armcompute.NewGalleryImageVersionsClient(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("creating gallery image versions client: %w", err)
	}

	regions := make([]*armcompute.TargetRegion, 0, len(replicationRegions)+1)
	add := func(name string) {
		regions = append(regions, &armcompute.TargetRegion{
			Name:                 to.Ptr(name),
			RegionalReplicaCount: to.Ptr[int32](1),
			StorageAccountType:   to.Ptr(armcompute.StorageAccountTypeStandardLRS),
		})
	}

	add(location)
	for _, r := range replicationRegions {
		add(r)
	}

	imageVersion := armcompute.GalleryImageVersion{
		Location: to.Ptr(location),
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				TargetRegions: regions,
			},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				OSDiskImage: &armcompute.GalleryOSDiskImage{
					HostCaching: to.Ptr(armcompute.HostCachingReadOnly),
					Source: &armcompute.GalleryDiskImageSource{
						StorageAccountID: to.Ptr(storageAccountID),
						URI:              to.Ptr(vhdBlobURL),
					},
				},
			},
		},
	}

	regionNames := append([]string{location}, replicationRegions...)
	log.Infof("Creating gallery image version %s/%s/%s in resource group %s (regions: %v)",
		galleryName, imageDefinition, version, resourceGroup, regionNames)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, imageDefinition, version, imageVersion, nil)
	if err != nil {
		return "", fmt.Errorf("beginning create gallery image version: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("waiting for gallery image version creation: %w", err)
	}
	if resp.ID == nil {
		return "", fmt.Errorf("gallery image version created without an ID")
	}
	return *resp.ID, nil
}

// UploadPodvmImageToGallery converts a qcow2 podvm image to a fixed-format VHD,
// uploads it to an Azure page blob, and registers a gallery image version
// pointing at it. Returns the resulting gallery image version resource ID.
func UploadPodvmImageToGallery(
	ctx context.Context,
	qcow2Path string,
	subscriptionID, location, resourceGroup,
	storageAccountName, containerName, blobName,
	galleryName, imageDefinition, version string,
	replicationRegions []string,
) (string, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("creating azure credential: %w", err)
	}

	rawFile, err := os.CreateTemp("", "podvm.*.raw")
	if err != nil {
		return "", fmt.Errorf("creating temp raw file: %w", err)
	}
	rawPath := rawFile.Name()
	rawFile.Close()
	defer os.Remove(rawPath)

	vhdFile, err := os.CreateTemp("", "podvm.*.vhd")
	if err != nil {
		return "", fmt.Errorf("creating temp vhd file: %w", err)
	}
	vhdPath := vhdFile.Name()
	vhdFile.Close()
	defer os.Remove(vhdPath)

	log.Infof("Converting qcow2 %s to raw %s", qcow2Path, rawPath)
	if err := ConvertQcow2ToRaw(qcow2Path, rawPath); err != nil {
		return "", err
	}

	if err := ResizeRawImageToMiB(rawPath); err != nil {
		return "", err
	}

	log.Infof("Converting raw %s to fixed-format VHD %s", rawPath, vhdPath)
	if err := ConvertRawToVHD(rawPath, vhdPath); err != nil {
		return "", err
	}

	blobURL, err := UploadVHDToPageBlob(ctx, cred, storageAccountName, containerName, blobName, vhdPath)
	if err != nil {
		return "", err
	}

	storageAccountID := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s",
		subscriptionID, resourceGroup, storageAccountName,
	)
	return CreateGalleryImageVersionFromVHD(
		ctx, cred,
		subscriptionID, location, resourceGroup,
		galleryName, imageDefinition, version,
		storageAccountID, blobURL,
		replicationRegions,
	)
}
