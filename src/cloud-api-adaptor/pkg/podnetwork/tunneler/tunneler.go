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
	PodIP               netip.Prefix `json:"podip"`
	PodHwAddr           string       `json:"pod-hw-addr"`
	InterfaceName       string       `json:"interface"`
	WorkerNodeIP        netip.Prefix `json:"worker-node-ip"`
	TunnelType          string       `json:"tunnel-type"`
	Routes              []*Route     `json:"routes"`
	Neighbors           []*Neighbor  `json:"neighbors"`
	MTU                 int          `json:"mtu"`
	Index               int          `json:"index"`
	VXLANPort           int          `json:"vxlan-port,omitempty"`
	VXLANID             int          `json:"vxlan-id,omitempty"`
	Dedicated           bool         `json:"dedicated"`
	ExternalNetViaPodVM bool         `json:"external-net-via-pod-vm"`
}

type Route struct {
	Dst      netip.Prefix         `json:"dst,omitempty"`
	GW       netip.Addr           `json:"gw,omitempty"`
	Dev      string               `json:"dev,omitempty"`
	Protocol netops.RouteProtocol `json:"protocol,omitempty"`
	Scope    netops.RouteScope    `json:"scope,omitempty"`
}

type Neighbor struct {
	IP           netip.Addr           `json:"ip,omitempty"`
	HardwareAddr string               `json:"hw-addr,omitempty"`
	Dev          string               `json:"dev,omitempty"`
	State        netops.NeighborState `json:"state,omitempty"`
}

type driver struct {
	newWorkerNodeTunneler func() (Tunneler, error)
	newPodNodeTunneler    func() (Tunneler, error)
}

var drivers = make(map[string]*driver)

func Register(tunnelType string, newWorkerNodeTunneler, newPodNodeTunneler func() (Tunneler, error)) {
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
	return driver.newWorkerNodeTunneler()
}

func PodNodeTunneler(tunnelType string) (Tunneler, error) {

	driver, err := getDriver(tunnelType)
	if err != nil {
		return nil, err
	}
	return driver.newPodNodeTunneler()
}
