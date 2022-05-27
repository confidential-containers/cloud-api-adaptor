//go:build libvirt
// +build libvirt

package libvirt

// Code copied from https://github.com/openshift/cluster-api-provider-libvirt

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	libvirtxml "libvirt.org/go/libvirtxml"
)

type image interface {
	size() (uint64, error)
	importImage(func(io.Reader) error, libvirtxml.StorageVolume) error
	string() string
}

type httpImage struct {
	url *url.URL
}

func (i *httpImage) string() string {
	return i.url.String()
}

func (i *httpImage) size() (uint64, error) {
	response, err := http.Head(i.url.String())
	if err != nil {
		return 0, err
	}
	if response.StatusCode != 200 {
		return 0,
			fmt.Errorf(
				"Error accessing remote resource: %s - %s",
				i.url.String(),
				response.Status)
	}

	length, err := strconv.Atoi(response.Header.Get("Content-Length"))
	if err != nil {
		err = fmt.Errorf(
			"Error while getting Content-Length of %q: %v - got %s",
			i.url.String(),
			err,
			response.Header.Get("Content-Length"))
		return 0, err
	}
	return uint64(length), nil
}

func (i *httpImage) importImage(copier func(io.Reader) error, vol libvirtxml.StorageVolume) error {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", i.url.String(), nil)

	if vol.Target.Timestamps != nil && vol.Target.Timestamps.Mtime != "" {
		req.Header.Set("If-Modified-Since", timeFromEpoch(vol.Target.Timestamps.Mtime).UTC().Format(http.TimeFormat))
	}
	response, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("Error while downloading %s: %s", i.url.String(), err)
	}

	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return nil
	}

	return copier(response.Body)
}

type localImage struct {
	path string
}

func newImage(source string) (image, error) {
	url, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("can't parse source %q as url: %v", source, err)
	}

	if strings.HasPrefix(url.Scheme, "http") {
		return &httpImage{url: url}, nil
	} else if url.Scheme == "file" || url.Scheme == "" {
		return &localImage{path: url.Path}, nil
	} else {
		return nil, fmt.Errorf("don't know how to read from %q: %s", url.String(), err)
	}
}

func (i *localImage) string() string {
	return i.path
}

func (i *localImage) size() (uint64, error) {
	fi, err := os.Stat(i.path)
	if err != nil {
		return 0, err
	}
	return uint64(fi.Size()), nil
}

func (i *localImage) importImage(copier func(io.Reader) error, vol libvirtxml.StorageVolume) error {
	file, err := os.Open(i.path)
	if err != nil {
		return fmt.Errorf("Error while opening %s: %s", i.path, err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}
	// we can skip the upload if the modification times are the same
	if vol.Target.Timestamps != nil && vol.Target.Timestamps.Mtime != "" {
		if fi.ModTime() == timeFromEpoch(vol.Target.Timestamps.Mtime) {
			logger.Printf("Modification time is the same: skipping image copy")
			return nil
		}
	}

	return copier(file)
}
