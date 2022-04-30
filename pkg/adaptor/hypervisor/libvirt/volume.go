//go:build libvirt
// +build libvirt

package libvirt

// Code copied from https://github.com/openshift/cluster-api-provider-libvirt

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
	"strconv"
	"strings"
	"time"
)

// ErrVolumeNotFound is returned when a domain is not found
var ErrVolumeNotFound = errors.New("Domain not found")

var waitSleepInterval = 1 * time.Second

// waitTimeout time
var waitTimeout = 5 * time.Minute

// waitForSuccess wait for success and timeout after 5 minutes.
func waitForSuccess(errorMessage string, f func() error) error {
	start := time.Now()
	for {
		err := f()
		if err == nil {
			return nil
		}
		logger.Printf("%s. Re-trying.\n", err)

		time.Sleep(waitSleepInterval)
		if time.Since(start) > waitTimeout {
			return fmt.Errorf("%s: %s", errorMessage, err)
		}
	}
}

func newDefVolume(name string) libvirtxml.StorageVolume {
	return libvirtxml.StorageVolume{
		Name: name,
		Target: &libvirtxml.StorageVolumeTarget{
			Format: &libvirtxml.StorageVolumeTargetFormat{
				Type: "qcow2",
			},
			Permissions: &libvirtxml.StorageVolumeTargetPermissions{
				Mode: "644",
			},
		},
		Capacity: &libvirtxml.StorageVolumeSize{
			Unit:  "bytes",
			Value: 1,
		},
	}
}

func newDefBackingStoreFromLibvirt(baseVolume *libvirt.StorageVol) (libvirtxml.StorageVolumeBackingStore, error) {
	baseVolumeDef, err := newDefVolumeFromLibvirt(baseVolume)
	if err != nil {
		return libvirtxml.StorageVolumeBackingStore{}, fmt.Errorf("could not get volume: %s", err)
	}
	baseVolPath, err := baseVolume.GetPath()
	if err != nil {
		return libvirtxml.StorageVolumeBackingStore{}, fmt.Errorf("could not get base image path: %s", err)
	}
	backingStoreDef := libvirtxml.StorageVolumeBackingStore{
		Path: baseVolPath,
		Format: &libvirtxml.StorageVolumeTargetFormat{
			Type: baseVolumeDef.Target.Format.Type,
		},
	}
	return backingStoreDef, nil
}

func newDefVolumeFromLibvirt(volume *libvirt.StorageVol) (libvirtxml.StorageVolume, error) {
	name, err := volume.GetName()
	if err != nil {
		return libvirtxml.StorageVolume{}, fmt.Errorf("could not get name for volume: %s", err)
	}
	volumeDefXML, err := volume.GetXMLDesc(0)
	if err != nil {
		return libvirtxml.StorageVolume{}, fmt.Errorf("could not get XML description for volume %s: %s", name, err)
	}
	volumeDef, err := newDefVolumeFromXML(volumeDefXML)
	if err != nil {
		return libvirtxml.StorageVolume{}, fmt.Errorf("could not get a volume definition from XML for %s: %s", volumeDef.Name, err)
	}
	return volumeDef, nil
}

// Creates a volume definition from a XML
func newDefVolumeFromXML(s string) (libvirtxml.StorageVolume, error) {
	var volumeDef libvirtxml.StorageVolume
	err := xml.Unmarshal([]byte(s), &volumeDef)
	if err != nil {
		return libvirtxml.StorageVolume{}, err
	}
	return volumeDef, nil
}

func timeFromEpoch(str string) time.Time {
	var s, ns int

	ts := strings.Split(str, ".")
	if len(ts) == 2 {
		ns, _ = strconv.Atoi(ts[1])
	}
	s, _ = strconv.Atoi(ts[0])

	return time.Unix(int64(s), int64(ns))
}

func uploadVolume(libvirtClient *libvirtClient, volumeDef libvirtxml.StorageVolume, img image) (string, error) {

	// Refresh the pool of the volume so that libvirt knows it is
	// not longer in use.
	err := waitForSuccess("Error refreshing pool for volume", func() error {
		return libvirtClient.pool.Refresh(0)
	})
	if err != nil {
		return "", fmt.Errorf("timeout when calling waitForSuccess: %v", err)
	}

	volumeDefXML, err := xml.Marshal(volumeDef)
	if err != nil {
		return "", fmt.Errorf("Error serializing libvirt volume: %s", err)
	}
	// create the volume
	volume, err := libvirtClient.pool.StorageVolCreateXML(string(volumeDefXML), 0)
	if err != nil {
		return "", fmt.Errorf("Error creating libvirt volume for device %s: %s", volumeDef.Name, err)
	}
	defer volume.Free()

	// upload ISO file
	err = img.importImage(newCopier(libvirtClient.connection, volume, volumeDef.Capacity.Value), volumeDef)
	if err != nil {
		return "", fmt.Errorf("Error while uploading volume %s: %s", img.string(), err)
	}

	volumeKey, err := volume.GetKey()
	if err != nil {
		return "", fmt.Errorf("Error retrieving volume key: %s", err)
	}
	logger.Printf("Volume ID: %s", volumeKey)
	return volumeKey, nil
}

func newCopier(conn *libvirt.Connect, volume *libvirt.StorageVol, size uint64) func(src io.Reader) error {
	copier := func(src io.Reader) error {
		var bytesCopied int64

		stream, err := conn.NewStream(0)
		if err != nil {
			return err
		}

		defer func() {
			if uint64(bytesCopied) != size {
				stream.Abort()
			} else {
				stream.Finish()
			}
			stream.Free()
		}()

		volume.Upload(stream, 0, size, 0)

		sio := newStreamIO(*stream)

		bytesCopied, err = io.Copy(sio, src)
		if err != nil {
			return err
		}
		logger.Printf("%d bytes uploaded\n", bytesCopied)
		return nil
	}
	return copier
}

func createVolume(volName string, volSize uint64, baseVolName string, libvirtClient *libvirtClient) error {
	volumeDef := newDefVolume(volName)
	volumeDef.Target.Format.Type = "qcow2"

	baseVolume, err := getVolume(libvirtClient, baseVolName)

	if err != nil {
		return fmt.Errorf("Can't retrieve volume %s", baseVolName)
	}
	defer baseVolume.Free()
	var baseVolumeInfo *libvirt.StorageVolInfo
	baseVolumeInfo, err = baseVolume.GetInfo()
	if err != nil {
		return fmt.Errorf("Can't retrieve volume info %s", baseVolName)
	}

	if baseVolumeInfo.Capacity > volSize {
		volumeDef.Capacity.Value = baseVolumeInfo.Capacity
	} else {
		volumeDef.Capacity.Value = volSize
	}

	backingStoreDef, err := newDefBackingStoreFromLibvirt(baseVolume)
	if err != nil {
		return fmt.Errorf("Could not retrieve backing store %s", baseVolName)
	}
	volumeDef.BackingStore = &backingStoreDef

	volumeDefXML, err := xml.Marshal(volumeDef)
	if err != nil {
		return fmt.Errorf("Error serializing libvirt volume: %s", err)
	}

	// create the volume
	// Refresh the pool of the volume so that libvirt knows it is
	// not longer in use.
	err = waitForSuccess("error refreshing pool for volume", func() error {
		return libvirtClient.pool.Refresh(0)
	})
	if err != nil {
		return fmt.Errorf("can't find storage pool '%s'", libvirtClient.poolName)
	}

	volume, err := libvirtClient.pool.StorageVolCreateXML(string(volumeDefXML), 0)
	if err != nil {
		return fmt.Errorf("Error creating libvirt volume: %s", err)
	}
	defer volume.Free()

	// we use the key as the id
	key, err := volume.GetKey()
	if err != nil {
		return fmt.Errorf("Error retrieving volume key: %s", err)
	}

	logger.Printf("Uploaded volume key %s", key)
	return nil

}

func getVolume(libvirtClient *libvirtClient, volumeName string) (*libvirt.StorageVol, error) {
	// Check whether the storage volume exists. Its name needs to be
	// unique.
	volume, err := libvirtClient.pool.LookupStorageVolByName(volumeName)
	if err != nil {
		// Let's try by ID in case of older Installer
		volume, err = libvirtClient.connection.LookupStorageVolByKey(volumeName)
		if err != nil {
			return nil, fmt.Errorf("can't retrieve volume %q: %v", volumeName, err)
		}
	}
	return volume, nil
}

// VolumeExists checks if a volume exists
func volumeExists(libvirtClient *libvirtClient, volumeName string) (bool, error) {

	logger.Printf("Check if %s volume exists", volumeName)
	volume, err := getVolume(libvirtClient, volumeName)
	if err != nil {
		return false, nil
	}
	volume.Free()
	return true, nil
}

func deleteVolumeByPath(libvirtClient *libvirtClient, path string) error {

	// Get volume name from path

	volume, err := libvirtClient.connection.LookupStorageVolByPath(path)
	if err != nil {
		logger.Printf("can't retrieve volume %q: %v", path, err)
		return err
	}

	defer volume.Free()
	// Get name
	name, err := volume.GetName()
	if err != nil {
		logger.Printf("Error retrieving volume name: %s", err)
		return err
	}

	return deleteVolume(libvirtClient, name)

}

func deleteVolume(libvirtClient *libvirtClient, name string) error {
	exists, err := volumeExists(libvirtClient, name)
	if err != nil {
		logger.Printf("Unable to check if volume (%s) exists", name)
		return err
	}
	if !exists {
		logger.Printf("Volume %s does not exists", name)
		return ErrVolumeNotFound
	}
	logger.Printf("Deleting volume %s", name)

	volume, err := getVolume(libvirtClient, name)
	if err != nil {
		return fmt.Errorf("Can't retrieve volume %s", name)
	}
	defer volume.Free()

	// Refresh the pool of the volume so that libvirt knows it is
	// not longer in use.
	volPool, err := volume.LookupPoolByVolume()
	if err != nil {
		return fmt.Errorf("Error retrieving pool for volume: %s", err)
	}
	defer volPool.Free()

	waitForSuccess("Error refreshing pool for volume", func() error {
		return volPool.Refresh(0)
	})

	err = volume.Delete(0)
	if err != nil {
		return fmt.Errorf("Can't delete volume %s: %s", name, err)
	}

	return nil
}
