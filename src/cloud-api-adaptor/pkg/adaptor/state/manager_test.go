package state

import (
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestState(sandboxID string) *SandboxState {
	return &SandboxState{
		Version:      1,
		SandboxID:    sandboxID,
		PodName:      "test-pod",
		PodNamespace: "default",
		NetNSPath:    "/var/run/netns/test",
		CreatedAt:    time.Now().Truncate(time.Second),
	}
}

func setupManager(t *testing.T, sandboxID string) *Manager {
	t.Helper()
	podsDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(podsDir, sandboxID), 0o755))
	m := NewManager(podsDir)
	return m
}

func TestSaveAndLoad(t *testing.T) {
	m := setupManager(t, "sandbox-1")
	st := newTestState("sandbox-1")
	st.InstanceID = "i-abc123"
	st.InstanceIPs = []string{"10.0.0.1"}
	st.ServerName = "server-1"
	st.ForwarderPort = "15150"

	require.NoError(t, m.Save(st))

	loaded, err := m.Load("sandbox-1")
	require.NoError(t, err)

	assert.Equal(t, st.SandboxID, loaded.SandboxID)
	assert.Equal(t, st.PodName, loaded.PodName)
	assert.Equal(t, st.PodNamespace, loaded.PodNamespace)
	assert.Equal(t, st.NetNSPath, loaded.NetNSPath)
	assert.Equal(t, st.InstanceID, loaded.InstanceID)
	assert.Equal(t, st.InstanceIPs, loaded.InstanceIPs)
	assert.Equal(t, st.ServerName, loaded.ServerName)
	assert.Equal(t, st.ForwarderPort, loaded.ForwarderPort)
	assert.Equal(t, st.CreatedAt, loaded.CreatedAt)
}

func TestLoadNonexistent(t *testing.T) {
	m := NewManager(t.TempDir())

	_, err := m.Load("does-not-exist")
	assert.Error(t, err)
}

func TestDelete(t *testing.T) {
	m := setupManager(t, "sandbox-1")
	st := newTestState("sandbox-1")
	require.NoError(t, m.Save(st))

	require.NoError(t, m.Delete("sandbox-1"))

	_, err := m.Load("sandbox-1")
	assert.Error(t, err)
}

func TestList(t *testing.T) {
	podsDir := t.TempDir()
	m := NewManager(podsDir)

	ids := []string{"sandbox-a", "sandbox-b", "sandbox-c"}
	for _, id := range ids {
		require.NoError(t, os.MkdirAll(filepath.Join(podsDir, id), 0o755))
		require.NoError(t, m.Save(newTestState(id)))
	}

	listed, err := m.List()
	require.NoError(t, err)

	sort.Strings(listed)
	sort.Strings(ids)
	assert.Equal(t, ids, listed)
}

func TestListEmptyDir(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "nonexistent"))

	listed, err := m.List()
	assert.NoError(t, err)
	assert.Nil(t, listed)
}

func TestSetReady(t *testing.T) {
	m := setupManager(t, "sandbox-1")
	st := newTestState("sandbox-1")
	require.NoError(t, m.Save(st))

	podNet := &tunneler.Config{
		InterfaceName: "eth0",
		Index:         5,
		VXLANPort:     4789,
		VXLANID:       555005,
	}
	require.NoError(t, m.SetReady("sandbox-1", podNet))

	loaded, err := m.Load("sandbox-1")
	require.NoError(t, err)
	assert.True(t, loaded.Running)
	assert.Equal(t, podNet.Index, loaded.PodNetwork.Index)
	assert.Equal(t, podNet.VXLANID, loaded.PodNetwork.VXLANID)
}

func TestUpdateInstance(t *testing.T) {
	m := setupManager(t, "sandbox-1")
	st := newTestState("sandbox-1")
	require.NoError(t, m.Save(st))

	ips := []netip.Addr{netip.MustParseAddr("192.168.1.10")}
	require.NoError(t, m.UpdateInstance("sandbox-1", "i-xyz", "vm-xyz", ips))

	loaded, err := m.Load("sandbox-1")
	require.NoError(t, err)
	assert.Equal(t, "i-xyz", loaded.InstanceID)
	assert.Equal(t, "vm-xyz", loaded.InstanceName)
	assert.Equal(t, []string{"192.168.1.10"}, loaded.InstanceIPs)
	assert.NotNil(t, loaded.StartedAt)
}

func TestAtomicWrite(t *testing.T) {
	podsDir := t.TempDir()
	sandboxID := "sandbox-1"
	require.NoError(t, os.MkdirAll(filepath.Join(podsDir, sandboxID), 0o755))
	m := NewManager(podsDir)

	require.NoError(t, m.Save(newTestState(sandboxID)))

	tmpPath := filepath.Join(podsDir, sandboxID, stateFileName+".tmp")
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "temp file should not remain after save")

	statePath := filepath.Join(podsDir, sandboxID, stateFileName)
	_, err = os.Stat(statePath)
	assert.NoError(t, err, "state file should exist")
}
