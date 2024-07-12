// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package tunneler

import (
	"fmt"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

type Tunneler interface {
	Setup(nsPath string, podNodeIPs []netip.Addr, config *Config) error
	Teardown(nsPath, hostInterface string, config *Config) error
}

type Config struct {
	PodIP         netip.Prefix `json:"podip"`
	PodHwAddr     string       `json:"pod-hw-addr"`
	InterfaceName string       `json:"interface"`
	WorkerNodeIP  netip.Prefix `json:"worker-node-ip"`
	TunnelType    string       `json:"tunnel-type"`
	Routes        []*Route     `json:"routes"`
	MTU           int          `json:"mtu"`
	Index         int          `json:"index"`
	VXLANPort     int          `json:"vxlan-port,omitempty"`
	VXLANID       int          `json:"vxlan-id,omitempty"`
	Dedicated     bool         `json:"dedicated"`
}

type Route struct {
	Dst      netip.Prefix
	GW       netip.Addr
	Dev      string
	Protocol netops.RouteProtocol
	Scope    netops.RouteScope
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
