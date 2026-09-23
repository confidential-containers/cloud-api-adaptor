// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const cdhBinaryPath = "/usr/local/bin/confidential-data-hub"

type volumeDeclaration struct {
	MountPoint  string `toml:"mount_point"`
	KeyID       string `toml:"key_id"`
	EncryptType string `toml:"encrypt_type"`
	FsType      string `toml:"fs_type,omitempty"`
	FsGroup     string `toml:"fs_group,omitempty"`
}

type storagePolicy struct {
	RequireEncryption  bool                `toml:"require_encryption"`
	AllowedKeyPrefixes []string            `toml:"allowed_key_prefixes"`
	Volumes            []volumeDeclaration `toml:"volumes"`
}

type resolvedVolume struct {
	EncryptType string
	KeyID       string
	FsType      string
	FsGroup     string
}

var isCoCoPodVMFunc = func() bool {
	_, err := os.Stat(cdhBinaryPath)
	return err == nil
}

func loadStoragePolicy(path string) *storagePolicy {
	data, err := os.ReadFile(path)
	if err == nil {
		var sp storagePolicy
		if err := toml.Unmarshal(data, &sp); err != nil {
			logger.Printf("WARNING: corrupted storage policy at %s: %v; requiring encryption (fail secure)", path, err)
			return &storagePolicy{RequireEncryption: true}
		}
		logger.Printf("Loaded storage policy from initdata: require_encryption=%v, volumes=%d",
			sp.RequireEncryption, len(sp.Volumes))
		return &sp
	}

	if isCoCoPodVMFunc() {
		logger.Printf("no storage-policy.toml, CDH present at %s, requiring encryption", cdhBinaryPath)
		return &storagePolicy{RequireEncryption: true}
	}

	logger.Printf("no storage-policy.toml and no CDH binary, allowing plaintext")
	return &storagePolicy{RequireEncryption: false}
}

func (sp *storagePolicy) enforcePolicy(volName, encryptType, keyID string) error {
	if !sp.RequireEncryption {
		return nil
	}

	if encryptType == "" {
		return fmt.Errorf(
			"cloud volume %s requires encryption but encrypt_type is empty",
			volName)
	}

	if keyID == "" {
		return fmt.Errorf(
			"cloud volume %s requires encryption but key_id is empty",
			volName)
	}

	if len(sp.AllowedKeyPrefixes) > 0 {
		for _, prefix := range sp.AllowedKeyPrefixes {
			if strings.HasPrefix(keyID, prefix) {
				return nil
			}
		}
		return fmt.Errorf(
			"cloud volume %s: key_id %q does not match allowed prefixes %v",
			volName, keyID, sp.AllowedKeyPrefixes)
	}

	return nil
}

func (sp *storagePolicy) resolveVolume(mountPoint, hostEncryptType, hostKeyID, hostFsType, hostFsGroup string) (*resolvedVolume, error) {
	if len(sp.Volumes) == 0 {
		if sp.RequireEncryption {
			logger.Printf("WARNING: no [[volumes]] in storage policy, using host-supplied params")
		}
		if err := sp.enforcePolicy(mountPoint, hostEncryptType, hostKeyID); err != nil {
			return nil, err
		}
		return &resolvedVolume{
			EncryptType: hostEncryptType,
			KeyID:       hostKeyID,
			FsType:      hostFsType,
			FsGroup:     hostFsGroup,
		}, nil
	}

	for _, v := range sp.Volumes {
		if v.MountPoint == mountPoint {
			logger.Printf("cloud volume at %s: using params from storage policy", mountPoint)
			fsType := v.FsType
			if fsType == "" {
				fsType = hostFsType
			}
			fsGroup := v.FsGroup
			if fsGroup == "" {
				fsGroup = hostFsGroup
			}
			return &resolvedVolume{
				EncryptType: v.EncryptType,
				KeyID:       v.KeyID,
				FsType:      fsType,
				FsGroup:     fsGroup,
			}, nil
		}
	}

	return nil, fmt.Errorf(
		"cloud volume at mount_point %q not declared in storage policy (declared: %v)",
		mountPoint, sp.declaredMountPoints())
}

func (sp *storagePolicy) checkAllVolumesMounted(mounted map[string]bool) error {
	for _, v := range sp.Volumes {
		if !mounted[v.MountPoint] {
			return fmt.Errorf(
				"declared volume at mount_point %q was not provided by host",
				v.MountPoint)
		}
	}
	return nil
}

func (sp *storagePolicy) declaredMountPoints() []string {
	pts := make([]string, len(sp.Volumes))
	for i, v := range sp.Volumes {
		pts[i] = v.MountPoint
	}
	return pts
}
