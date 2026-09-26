// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withCDHDetection(t *testing.T, result bool) {
	t.Helper()
	orig := isCoCoPodVMFunc
	isCoCoPodVMFunc = func() bool { return result }
	t.Cleanup(func() { isCoCoPodVMFunc = orig })
}

func policyWithVolumes() *storagePolicy {
	return &storagePolicy{
		RequireEncryption: true,
		Volumes: []volumeDeclaration{
			{MountPoint: "/data", KeyID: "kbs:///default/storage/data-key", EncryptType: "luks", FsType: "ext4", FsGroup: "1000"},
			{MountPoint: "/logs", KeyID: "kbs:///default/storage/logs-key", EncryptType: "luks"},
		},
	}
}

func TestIsCoCoPodVM_BinaryExists(t *testing.T) {
	dir := t.TempDir()
	fakeCDH := filepath.Join(dir, "confidential-data-hub")
	require.NoError(t, os.WriteFile(fakeCDH, []byte("#!/bin/sh"), 0o755))

	orig := isCoCoPodVMFunc
	isCoCoPodVMFunc = func() bool {
		_, err := os.Stat(fakeCDH)
		return err == nil
	}
	defer func() { isCoCoPodVMFunc = orig }()
	assert.True(t, isCoCoPodVMFunc())
}

func TestIsCoCoPodVM_BinaryAbsent(t *testing.T) {
	orig := isCoCoPodVMFunc
	isCoCoPodVMFunc = func() bool {
		_, err := os.Stat("/nonexistent/path/confidential-data-hub")
		return err == nil
	}
	defer func() { isCoCoPodVMFunc = orig }()
	assert.False(t, isCoCoPodVMFunc())
}

func TestLoadStoragePolicy_ExplicitRequire(t *testing.T) {
	withCDHDetection(t, false)
	dir := t.TempDir()
	path := filepath.Join(dir, "storage-policy.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
require_encryption = true
allowed_key_prefixes = ["default/storage-keys/"]
`), 0o644))

	sp := loadStoragePolicy(path)
	assert.True(t, sp.RequireEncryption)
	assert.Equal(t, []string{"default/storage-keys/"}, sp.AllowedKeyPrefixes)
}

func TestLoadStoragePolicy_ExplicitOptOut(t *testing.T) {
	withCDHDetection(t, true)
	dir := t.TempDir()
	path := filepath.Join(dir, "storage-policy.toml")
	require.NoError(t, os.WriteFile(path, []byte(`require_encryption = false`), 0o644))

	sp := loadStoragePolicy(path)
	assert.False(t, sp.RequireEncryption)
}

func TestLoadStoragePolicy_Corrupted_FailSecure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage-policy.toml")
	require.NoError(t, os.WriteFile(path, []byte("not valid {{{ toml"), 0o644))

	sp := loadStoragePolicy(path)
	assert.True(t, sp.RequireEncryption)
}

func TestLoadStoragePolicy_NoPolicy_CDH_RequiresEncryption(t *testing.T) {
	withCDHDetection(t, true)
	sp := loadStoragePolicy("/nonexistent/policy.toml")
	assert.True(t, sp.RequireEncryption)
}

func TestLoadStoragePolicy_NoPolicy_NoCDH_AllowsPlaintext(t *testing.T) {
	withCDHDetection(t, false)
	sp := loadStoragePolicy("/nonexistent/policy.toml")
	assert.False(t, sp.RequireEncryption)
}

func TestLoadStoragePolicy_WithVolumeDeclarations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage-policy.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
require_encryption = true

[[volumes]]
mount_point = "/data"
key_id = "kbs:///default/storage/my-key"
encrypt_type = "luks"
fs_type = "ext4"
fs_group = "1000"

[[volumes]]
mount_point = "/logs"
key_id = "kbs:///default/storage/logs-key"
encrypt_type = "luks"
`), 0o644))

	sp := loadStoragePolicy(path)
	require.Len(t, sp.Volumes, 2)
	assert.Equal(t, "/data", sp.Volumes[0].MountPoint)
	assert.Equal(t, "ext4", sp.Volumes[0].FsType)
	assert.Equal(t, "1000", sp.Volumes[0].FsGroup)
	assert.Equal(t, "", sp.Volumes[1].FsType)
}

func TestEnforcePolicy_EncryptionStripped(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: true}
	require.Error(t, sp.enforcePolicy("vol-0", "", "key"))
}

func TestEnforcePolicy_KeyIDStripped(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: true}
	require.Error(t, sp.enforcePolicy("vol-0", "luks", ""))
}

func TestEnforcePolicy_KeySwapped(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: true, AllowedKeyPrefixes: []string{"default/"}}
	require.Error(t, sp.enforcePolicy("vol-0", "luks", "attacker/key"))
}

func TestEnforcePolicy_Valid(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: true, AllowedKeyPrefixes: []string{"default/"}}
	assert.NoError(t, sp.enforcePolicy("vol-0", "luks", "default/my-key"))
}

func TestEnforcePolicy_OptedOut(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: false}
	assert.NoError(t, sp.enforcePolicy("vol-0", "", ""))
}

func TestResolveVolume_HostSwapsKey(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/data", "luks", "attacker/evil-key", "ext4", "")
	require.NoError(t, err)
	assert.Equal(t, "kbs:///default/storage/data-key", rv.KeyID)
}

func TestResolveVolume_HostStripsEncryption(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/data", "", "", "ext4", "")
	require.NoError(t, err)
	assert.Equal(t, "luks", rv.EncryptType)
	assert.Equal(t, "kbs:///default/storage/data-key", rv.KeyID)
}

func TestResolveVolume_HostInjectsVolume(t *testing.T) {
	sp := policyWithVolumes()
	_, err := sp.resolveVolume("/secrets", "luks", "kbs:///key", "ext4", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not declared")
}

func TestResolveVolume_HostChangesEncryptType(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/data", "plain", "kbs:///key", "ext4", "")
	require.NoError(t, err)
	assert.Equal(t, "luks", rv.EncryptType)
}

func TestResolveVolume_HostChangesMountPoint(t *testing.T) {
	sp := policyWithVolumes()
	_, err := sp.resolveVolume("/tmp/evil", "luks", "kbs:///key", "ext4", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not declared")
}

func TestResolveVolume_HostSwapsFsType(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/data", "luks", "kbs:///key", "xfs", "")
	require.NoError(t, err)
	assert.Equal(t, "ext4", rv.FsType)
}

func TestResolveVolume_HostSwapsFsGroup(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/data", "luks", "kbs:///key", "ext4", "0")
	require.NoError(t, err)
	assert.Equal(t, "1000", rv.FsGroup)
}

func TestCheckAllVolumesMounted_HostRemovesVolume(t *testing.T) {
	sp := policyWithVolumes()
	err := sp.checkAllVolumesMounted(map[string]bool{"/data": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/logs")
}

func TestCheckAllVolumesMounted_AnnotationStripped(t *testing.T) {
	sp := policyWithVolumes()
	err := sp.checkAllVolumesMounted(map[string]bool{})
	require.Error(t, err)
}

func TestCheckAllVolumesMounted_AllPresent(t *testing.T) {
	sp := policyWithVolumes()
	assert.NoError(t, sp.checkAllVolumesMounted(map[string]bool{"/data": true, "/logs": true}))
}

func TestResolveVolume_FsTypeFallsBackWhenNotDeclared(t *testing.T) {
	sp := policyWithVolumes()
	rv, err := sp.resolveVolume("/logs", "luks", "kbs:///key", "xfs", "2000")
	require.NoError(t, err)
	assert.Equal(t, "xfs", rv.FsType)
	assert.Equal(t, "2000", rv.FsGroup)
}

func TestResolveVolume_NoDeclarations_FallsBackToHost(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: false}
	rv, err := sp.resolveVolume("/data", "luks", "kbs:///key", "xfs", "1000")
	require.NoError(t, err)
	assert.Equal(t, "luks", rv.EncryptType)
	assert.Equal(t, "kbs:///key", rv.KeyID)
	assert.Equal(t, "xfs", rv.FsType)
}

func TestResolveVolume_NoDeclarations_EnforcePolicyApplies(t *testing.T) {
	sp := &storagePolicy{RequireEncryption: true}
	_, err := sp.resolveVolume("/data", "", "", "ext4", "")
	require.Error(t, err)
}

func TestCheckAllVolumesMounted_NoDeclarations_Passes(t *testing.T) {
	sp := &storagePolicy{}
	assert.NoError(t, sp.checkAllVolumesMounted(map[string]bool{}))
}

func TestResolveVolume_MixedEncryption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage-policy.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[volumes]]
mount_point = "/data"
key_id = "kbs:///default/storage/data-key"
encrypt_type = "luks"
fs_type = "ext4"

[[volumes]]
mount_point = "/scratch"
key_id = ""
encrypt_type = ""
`), 0o644))

	sp := loadStoragePolicy(path)

	rv, err := sp.resolveVolume("/data", "", "", "xfs", "")
	require.NoError(t, err)
	assert.Equal(t, "luks", rv.EncryptType)
	assert.Equal(t, "ext4", rv.FsType)

	rv, err = sp.resolveVolume("/scratch", "luks", "kbs:///evil", "ext4", "")
	require.NoError(t, err)
	assert.Equal(t, "", rv.EncryptType)
}

func TestResolveVolume_RequireEncryptionIgnoredForDeclaredVolumes(t *testing.T) {
	sp := &storagePolicy{
		RequireEncryption: true,
		Volumes: []volumeDeclaration{
			{MountPoint: "/scratch", KeyID: "", EncryptType: ""},
		},
	}
	rv, err := sp.resolveVolume("/scratch", "luks", "kbs:///evil", "ext4", "")
	require.NoError(t, err)
	assert.Equal(t, "", rv.EncryptType)
	assert.Equal(t, "", rv.KeyID)
}

func TestEnforcePolicy_PrefixedKeyID_DoesNotMatchBarePrefix(t *testing.T) {
	sp := &storagePolicy{
		RequireEncryption:  true,
		AllowedKeyPrefixes: []string{"default/"},
	}
	err := sp.enforcePolicy("vol-0", "luks", "kbs:///default/my-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestNormalizeKBSKeyURI_RawPath(t *testing.T) {
	assert.Equal(t, "kbs:///default/key", normalizeKBSKeyURI("default/key"))
}

func TestNormalizeKBSKeyURI_AlreadyPrefixed(t *testing.T) {
	assert.Equal(t, "kbs:///default/key", normalizeKBSKeyURI("kbs:///default/key"))
}

func TestNormalizeKBSKeyURI_InitdataFormat(t *testing.T) {
	assert.Equal(t, "kbs:///org/team/vol-key", normalizeKBSKeyURI("kbs:///org/team/vol-key"))
}
