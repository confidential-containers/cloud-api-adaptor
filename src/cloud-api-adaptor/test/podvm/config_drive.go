// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package podvm

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/kdomanski/iso9660"
)

const testForwarderConfig = `{
	"pod-network": {
		"podip": "10.244.1.21/24",
		"pod-hw-addr": "32:b9:59:6b:f0:d5",
		"interface": "eth0",
		"worker-node-ip": "10.224.0.5/16",
		"tunnel-type": "vxlan",
		"routes": [
			{
				"dst": "0.0.0.0/0",
				"gw": "10.244.1.1",
				"dev": "eth0",
				"protocol": "boot"
			},
			{
				"dst": "10.244.1.0/24",
				"dev": "eth0",
				"protocol": "kernel",
				"scope": "link"
			}
		],
		"mtu": 1500,
		"index": 2,
		"vxlan-port": 8472,
		"vxlan-id": 555002,
		"dedicated": false
	},
	"pod-namespace": "default",
	"pod-name": "podvm-test"
}`

func createConfigDrive(path string, config cloudinit.CloudConfigGenerator) (err error) {
	userData, err := config.Generate()
	if err != nil {
		return fmt.Errorf("generate user data: %w", err)
	}

	writer, err := iso9660.NewWriter()
	if err != nil {
		return fmt.Errorf("create ISO9660 writer: %w", err)
	}
	defer func() {
		err = errors.Join(err, writer.Cleanup())
	}()

	if err := writer.AddFile(strings.NewReader(userData), "user-data"); err != nil {
		return fmt.Errorf("add user-data to config drive: %w", err)
	}
	if err := writer.AddFile(strings.NewReader(""), "meta-data"); err != nil {
		return fmt.Errorf("add meta-data to config drive: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create config drive: %w", err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	if err := writer.WriteTo(file, "cidata"); err != nil {
		return fmt.Errorf("write config drive: %w", err)
	}
	return nil
}
