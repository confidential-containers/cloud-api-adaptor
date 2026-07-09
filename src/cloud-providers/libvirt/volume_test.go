//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"
)

const (
	defVolFormat   = "qcow2"
	defVolPermMode = "644"
	defVolCapUnit  = "bytes"

	testOperation = "test operation"
	errPersistent = "persistent error"
	errOverflow   = "overflows uint64"

	alwaysFails = -1
)

// setWaitTimers overrides the package-level waitTimeout and waitSleepInterval
// for the duration of a single test and restores them via t.Cleanup.
func setWaitTimers(t *testing.T, timeout, interval time.Duration) {
	t.Helper()
	orig, origInterval := waitTimeout, waitSleepInterval
	waitTimeout = timeout
	waitSleepInterval = interval
	t.Cleanup(func() {
		waitTimeout = orig
		waitSleepInterval = origInterval
	})
}

// assertVolumeDefShape asserts the fixed structural properties that every
// volume created by newDefVolume must satisfy.
func assertVolumeDefShape(t *testing.T, vol libvirtxml.StorageVolume, wantName, wantCapUnit string, wantCapValue uint64) {
	t.Helper()
	assert.Equal(t, wantName, vol.Name)
	require.NotNil(t, vol.Target)
	require.NotNil(t, vol.Target.Format)
	require.NotNil(t, vol.Target.Permissions)
	assert.Equal(t, defVolFormat, vol.Target.Format.Type)
	assert.Equal(t, defVolPermMode, vol.Target.Permissions.Mode)
	require.NotNil(t, vol.Capacity)
	assert.Equal(t, wantCapUnit, vol.Capacity.Unit)
	assert.Equal(t, wantCapValue, vol.Capacity.Value)
}

func TestNewDefVolume(t *testing.T) {
	tests := []struct {
		name       string
		volumeName string
	}{
		{name: "simple name", volumeName: "test-vol"},
		{name: "name with extension", volumeName: "test-vol.qcow2"},
		{name: "name with dashes", volumeName: "test-vol-123-abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertVolumeDefShape(t, newDefVolume(tt.volumeName), tt.volumeName, defVolCapUnit, 1)
		})
	}
}

func TestNewDefVolumeFromXML(t *testing.T) {
	xmlCapacity := strconv.FormatUint(bytesPerGiB, 10)

	tests := []struct {
		name      string
		xmlData   string
		wantErr   string // non-empty: expect error containing this substring
		checkFunc func(*testing.T, libvirtxml.StorageVolume)
	}{
		{
			name: "valid volume XML",
			xmlData: `<volume>
				<name>` + testVolumeName + `</name>
				<capacity unit="bytes">` + xmlCapacity + `</capacity>
				<target>
					<format type="qcow2"/>
					<permissions>
						<mode>0644</mode>
					</permissions>
				</target>
			</volume>`,
			checkFunc: func(t *testing.T, vol libvirtxml.StorageVolume) {
				assert.Equal(t, testVolumeName, vol.Name)
				assert.Equal(t, bytesPerGiB, vol.Capacity.Value)
				assert.Equal(t, defVolFormat, vol.Target.Format.Type)
			},
		},
		{
			name: "volume with backing store",
			xmlData: `<volume>
				<name>` + testVolumeName + `</name>
				<capacity unit="bytes">` + xmlCapacity + `</capacity>
				<target>
					<format type="qcow2"/>
				</target>
				<backingStore>
					<path>/var/lib/libvirt/images/base.qcow2</path>
					<format type="qcow2"/>
				</backingStore>
			</volume>`,
			checkFunc: func(t *testing.T, vol libvirtxml.StorageVolume) {
				assert.Equal(t, testVolumeName, vol.Name)
				assert.Equal(t, bytesPerGiB, vol.Capacity.Value)
				assert.NotNil(t, vol.BackingStore)
				assert.Contains(t, vol.BackingStore.Path, "base.qcow2")
			},
		},
		{
			name: "volume with GiB capacity unit",
			xmlData: `<volume>
				<name>` + testVolumeName + `</name>
				<capacity unit="GiB">10</capacity>
				<target>
					<format type="raw"/>
				</target>
			</volume>`,
			checkFunc: func(t *testing.T, vol libvirtxml.StorageVolume) {
				assert.Equal(t, "GiB", vol.Capacity.Unit)
				assert.Equal(t, uint64(10), vol.Capacity.Value)
			},
		},
		{
			name:    "invalid element — not a volume",
			xmlData: `<invalid>xml</invalid>`,
			wantErr: "expected element type <volume>",
		},
		{
			name:    "malformed XML — unclosed tag",
			xmlData: `<volume><name>test</name>`,
			wantErr: "unexpected EOF",
		},
		{
			name:    "empty XML",
			xmlData: ``,
			wantErr: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeDef, err := newDefVolumeFromXML(tt.xmlData)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, volumeDef)
			}
		})
	}
}

func TestWaitForSuccess(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		interval      time.Duration
		failCount     int // number of failures before success; use alwaysFails to never succeed
		wantErr       bool
		wantCallCount int
	}{
		{
			name:          "succeeds immediately",
			timeout:       200 * time.Millisecond,
			interval:      20 * time.Millisecond,
			failCount:     0,
			wantErr:       false,
			wantCallCount: 1,
		},
		{
			name:          "succeeds after 2 retries",
			timeout:       200 * time.Millisecond,
			interval:      20 * time.Millisecond,
			failCount:     2,
			wantErr:       false,
			wantCallCount: 3,
		},
		{
			name:          "succeeds after 5 retries",
			timeout:       200 * time.Millisecond,
			interval:      20 * time.Millisecond,
			failCount:     5,
			wantErr:       false,
			wantCallCount: 6,
		},
		{
			name:      "times out — error contains operation name and last error",
			timeout:   100 * time.Millisecond,
			interval:  10 * time.Millisecond,
			failCount: alwaysFails,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWaitTimers(t, tt.timeout, tt.interval)

			callCount := 0
			err := waitForSuccess(testOperation, func() error {
				callCount++
				if tt.failCount == alwaysFails {
					return errors.New(errPersistent)
				}
				if callCount <= tt.failCount {
					return fmt.Errorf("attempt %d failed", callCount)
				}
				return nil
			})

			if tt.wantErr {
				assert.ErrorContains(t, err, testOperation+": "+errPersistent)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCallCount, callCount)
			}
		})
	}
}

func TestVolumeCapacityBytesTable(t *testing.T) {
	tests := []struct {
		name                 string
		volSizeGiB           uint64
		backingCapacityBytes uint64
		wantBytes            uint64
		wantErr              string // non-empty: expect error containing this substring
	}{
		{
			name:                 "requested larger than backing — uses requested",
			volSizeGiB:           20,
			backingCapacityBytes: 10 * bytesPerGiB,
			wantBytes:            20 * bytesPerGiB,
		},
		{
			name:                 "backing larger than requested — uses backing",
			volSizeGiB:           5,
			backingCapacityBytes: 10 * bytesPerGiB,
			wantBytes:            10 * bytesPerGiB,
		},
		{
			name:                 "requested equal to backing",
			volSizeGiB:           10,
			backingCapacityBytes: 10 * bytesPerGiB,
			wantBytes:            10 * bytesPerGiB,
		},
		{
			name:       "1 GiB converts correctly",
			volSizeGiB: 1,
			wantBytes:  bytesPerGiB,
		},
		{
			name:       "10 GiB converts correctly",
			volSizeGiB: 10,
			wantBytes:  10 * bytesPerGiB,
		},
		{
			name:       "1000 GiB — large but valid",
			volSizeGiB: 1000,
			wantBytes:  1000 * bytesPerGiB,
		},
		{
			name:                 "zero requested — uses backing capacity",
			volSizeGiB:           0,
			backingCapacityBytes: 5 * bytesPerGiB,
			wantBytes:            5 * bytesPerGiB,
		},
		{
			name:                 "both zero",
			volSizeGiB:           0,
			backingCapacityBytes: 0,
			wantBytes:            0,
		},
		{
			name:       "max safe GiB — no overflow",
			volSizeGiB: ^uint64(0) / bytesPerGiB,
			wantBytes:  (^uint64(0) / bytesPerGiB) * bytesPerGiB,
		},
		{
			name:       "max uint64 GiB — overflows",
			volSizeGiB: ^uint64(0),
			wantErr:    errOverflow,
		},
		{
			name:       "just above max safe GiB — overflows",
			volSizeGiB: ^uint64(0)/bytesPerGiB + 1,
			wantErr:    errOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := volumeCapacityBytes(tt.volSizeGiB, tt.backingCapacityBytes)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantBytes, result)
		})
	}
}
