// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type MetadataRetriever struct {
	client *imds.Client
	mac    string // save it as it commonly used in path
}

// no error return, if something is wrong, will fail on get()
func newMetadataRetriever() *MetadataRetriever {
	var r MetadataRetriever
	r.client = imds.New(imds.Options{ClientEnableState: imds.ClientDefaultEnableState}) // use imds.ClientEnabled to enforce enabling
	mac, err := r.get("mac")
	if err != nil {
		logger.Printf("NewMetadataRetriever is initialized without mac (%v)", err)
		return &r
	}
	r.mac = mac
	return &r
}

func (r MetadataRetriever) get(path string) (string, error) {
	output, err := r.client.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: path,
	})
	if err != nil {
		return "", err
	}
	defer output.Content.Close()
	bytes, err := io.ReadAll(output.Content)
	if err != nil {
		return "", err
	}
	return string(bytes), err
}

func retrieveMissingConfig(cfg *Config) error {
	mdr := newMetadataRetriever()
	if cfg.SubnetID == "" {
		logger.Printf("SubnetId was not provided, trying to fetch it from IMDS")
		subnetIDPath := fmt.Sprintf("network/interfaces/macs/%s/subnet-id", mdr.mac)
		subnetID, err := mdr.get(subnetIDPath)
		if err != nil {
			return err
		}
		cfg.SubnetID = subnetID
		logger.Printf("\"%s\" SubnetId retrieved from IMDS", subnetID)
	}
	if cfg.Region == "" {
		logger.Printf("Region was not provided, trying to fetch it from IMDS")
		region, err := mdr.get("placement/region")
		if err != nil {
			return err
		}
		cfg.Region = region
		logger.Printf("\"%s\" Region retrieved from IMDS", region)
	}
	if cfg.KeyName == "" {
		logger.Printf("KeyName was not provided, trying to fetch it from IMDS")
		rawKey, err := mdr.get("public-keys")
		if err != nil {
			logger.Printf("failed to retrieve key, skipped: %v", err)
		}
		var keyName string
		n, err := fmt.Sscanf(rawKey, "0=%s", &keyName)
		if err != nil || n < 1 {
			logger.Printf("failed to retrieve key, skipped")
		} else {
			cfg.KeyName = keyName
			logger.Printf("\"%s\" KeyName retrieved from IMDS", keyName)
		}
	}
	if len(cfg.SecurityGroupIds) < 1 {
		logger.Printf("SecurityGroupIds was not provided, trying to fetch it from IMDS")
		securityGroupIdsPath := fmt.Sprintf("network/interfaces/macs/%s/security-group-ids", mdr.mac)
		securityGroupIds, err := mdr.get(securityGroupIdsPath)
		if err != nil {
			return err
		}
		cfg.SecurityGroupIds = strings.Fields(securityGroupIds)
		logger.Printf("\"%s\" SecurityGroupIds retrieved from IMDS", &cfg.SecurityGroupIds)
	}
	return nil
}
