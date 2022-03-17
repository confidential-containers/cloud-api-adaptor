// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package tunneler

import (
	"fmt"
	"net"
)

type Tunneler interface {
	Setup(nsPath string, podNodeIPs []net.IP, config *Config) error
	Teardown(nsPath, hostInterface string, config *Config) error
}

type Config struct {
	PodIP         string
	Routes        []*Route `json:"routes"`
	InterfaceName string   `json:"interface"`
	MTU           int      `json:"mtu"`
	WorkerNodeIP  string   `json:"worker-node-ip"`
	TunnelType    string   `json:"tunnel-type"`
	Dedicated     bool     `json:"dedicated"`
	Index         int      `json:"index"`
}

type Route struct {
	Dst string
	GW  string
	Dev string
}

type driver struct {
	newWorkerNodeTunneler func() Tunneler
	newPodNodeTunneler    func() Tunneler
}

var drivers = make(map[string]*driver)

func Register(tunnelType string, newWorkerNodeTunneler, newPodNodeTunneler func() Tunneler) {
	drivers[tunnelType] = &driver{
		newWorkerNodeTunneler: newWorkerNodeTunneler,
		newPodNodeTunneler:    newPodNodeTunneler,
	}
}

func getDriver(tunnelType string) (*driver, error) {

	driver, ok := drivers[tunnelType]
	if !ok {
		return nil, fmt.Errorf("unknown tunnel type: %q", tunnelType)
	}
	return driver, nil
}

func WorkerNodeTunneler(tunnelType string) (Tunneler, error) {

	driver, err := getDriver(tunnelType)
	if err != nil {
		return nil, err
	}
	return driver.newWorkerNodeTunneler(), nil
}

func PodNodeTunneler(tunnelType string) (Tunneler, error) {

	driver, err := getDriver(tunnelType)
	if err != nil {
		return nil, err
	}
	return driver.newPodNodeTunneler(), nil
}
