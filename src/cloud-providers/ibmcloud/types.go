// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"strings"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
)

type instanceProfiles []string

func (i *instanceProfiles) String() string {
	return strings.Join(*i, ", ")
}

func (i *instanceProfiles) Set(value string) error {
	*i = append(*i, toList(value, ",")...)
	return nil
}

type Images []Image
type Image struct {
	ID   string
	Arch string
	OS   string
}

func (i *Images) String() string {
	switch len(*i) {
	case 0:
		return ""
	case 1:
		return (*i)[0].ID
	}
	var b strings.Builder
	b.WriteString((*i)[0].ID)
	for _, image := range (*i)[1:] {
		b.WriteString(",")
		b.WriteString(image.ID)
	}
	return b.String()
}

func (i *Images) Set(value string) error {
	IDs := toList(value, ",")
	for _, id := range IDs {
		*i = append(*i, Image{ID: id})
	}
	return nil
}

func toList(value, sep string) []string {
	if len(value) == 0 {
		return make([]string, 0)
	}
	return strings.Split(value, sep)
}

type tags []string

func (i *tags) String() string {
	return strings.Join(*i, ", ")
}

func (i *tags) Set(value string) error {
	*i = append(*i, toList(value, ",")...)
	return nil
}

type dedicatedHostIDs []string

func (i *dedicatedHostIDs) String() string {
	return strings.Join(*i, ", ")
}

func (i *dedicatedHostIDs) Set(value string) error {
	*i = append(*i, toList(value, ",")...)
	return nil
}

type dedicatedHostGroupIDs []string

func (i *dedicatedHostGroupIDs) String() string {
	return strings.Join(*i, ", ")
}

func (i *dedicatedHostGroupIDs) Set(value string) error {
	*i = append(*i, toList(value, ",")...)
	return nil
}

type Config struct {
	ApiKey                   string
	IAMProfileID             string
	CRTokenFileName          string
	IamServiceURL            string
	VpcServiceURL            string
	ResourceGroupID          string
	ProfileName              string
	ZoneName                 string
	Images                   Images
	PrimarySubnetID          string
	PrimarySecurityGroupID   string
	SecondarySubnetID        string
	SecondarySecurityGroupID string
	KeyID                    string
	VpcID                    string
	InstanceProfiles         instanceProfiles
	InstanceProfileSpecList  []provider.InstanceTypeSpec
	DisableCVM               bool
	ClusterID                string
	Tags                     tags
	DedicatedHostIDs         dedicatedHostIDs
	DedicatedHostGroupIDs    dedicatedHostGroupIDs

	selectedDedicatedHostID      string
	selectedDedicatedHostGroupID string
}

func (c Config) Redact() Config {
	return *util.RedactStruct(&c, "ApiKey").(*Config)
}
