package tlsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls-material.json")

	ca1, cert1, key1, err := LoadOrCreateTLSMaterial(path)
	require.NoError(t, err)
	require.NotNil(t, ca1)
	require.NotEmpty(t, cert1)
	require.NotEmpty(t, key1)

	ca2, cert2, key2, err := LoadOrCreateTLSMaterial(path)
	require.NoError(t, err)

	assert.Equal(t, ca1.RootCertificate(), ca2.RootCertificate())
	assert.Equal(t, cert1, cert2)
	assert.Equal(t, key1, key2)
}

func TestCorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls-material.json")

	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	ca, cert, key, err := LoadOrCreateTLSMaterial(path)
	require.NoError(t, err)
	assert.NotNil(t, ca)
	assert.NotEmpty(t, cert)
	assert.NotEmpty(t, key)
}

func TestFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls-material.json")

	_, _, _, err := LoadOrCreateTLSMaterial(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
