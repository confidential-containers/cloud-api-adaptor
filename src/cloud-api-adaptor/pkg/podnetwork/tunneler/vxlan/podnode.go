// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

const (
	hostVxlanInterface = "vxlan0"
	maxMTU             = 1450
)

type podNodeTunneler struct {
}

func NewPodNodeTunneler() (tunneler.Tunneler, error) {
	return &podNodeTunneler{}, nil
}

func (t *podNodeTunneler) Setup(nsPath string, podNodeIPs []netip.Addr, config *tunneler.Config) error {

	podVxlanInterface := config.InterfaceName
	if podVxlanInterface == "" {
		return errors.New("InterfaceName is not specified")
	}

	nodeAddr := config.WorkerNodeIP

	if !nodeAddr.IsValid() {
		return fmt.Errorf("WorkerNodeIP is not specified: %#v", config.WorkerNodeIP)
	}

	podAddr := config.PodIP
	if !podAddr.IsValid() {
		return fmt.Errorf("PodIP is not specified: %#v", config.PodIP)
	}

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get host network namespace: %w", err)
	}
	defer hostNS.Close()

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a pod network namespace: %s: %w", nsPath, err)
	}
	defer podNS.Close()

	if err := iptablesSetup(hostNS, nodeAddr.Addr(), config.VXLANPort, config.VXLANID); err != nil {
		return err
	}

	vxlanDevice := &netops.VXLAN{
		Group: nodeAddr.Addr(),
		ID:    config.VXLANID,
		Port:  config.VXLANPort,
	}
	logger.Printf("Creating VXLAN interface %s on %s with group %s, id %d, port %d", hostVxlanInterface, hostNS.Path(), vxlanDevice.Group, vxlanDevice.ID, vxlanDevice.Port)

	vxlan, err := hostNS.LinkAdd(hostVxlanInterface, vxlanDevice)
	if err != nil {
		return fmt.Errorf("failed to add vxlan interface %s: %w", hostVxlanInterface, err)
	}

	if err := vxlan.SetNamespace(podNS); err != nil {
		return fmt.Errorf("failed to move vxlan interface %s to netns %s: %w", hostVxlanInterface, podNS.Path(), err)
	}

	if err := vxlan.SetName(podVxlanInterface); err != nil {
		return fmt.Errorf("failed to rename vxlan interface %s on netns %s: %w", hostVxlanInterface, podNS.Path(), err)
	}

	if err := vxlan.SetHardwareAddr(config.PodHwAddr); err != nil {
		return fmt.Errorf("failed to set pod HW address %s on %s: %w", config.PodHwAddr, podVxlanInterface, err)
	}

	mtu := int(config.MTU)
	if mtu > maxMTU {
		mtu = maxMTU
	}
	if err := vxlan.SetMTU(mtu); err != nil {
		return fmt.Errorf("failed to set MTU of %s to %d on %s: %w", podVxlanInterface, mtu, nsPath, err)
	}

	if err := vxlan.AddAddr(podAddr); err != nil {
		return fmt.Errorf("failed to add pod IP %s to %s on %s: %w", podAddr, podVxlanInterface, nsPath, err)
	}

	if err := vxlan.SetUp(); err != nil {
		return err
	}

	return nil
}

func (t *podNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {

	ifName := config.InterfaceName

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get host network namespace: %w", err)
	}
	defer hostNS.Close()

	podNS, err := netops.OpenNamespace(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a pod network namespace: %s: %w", nsPath, err)
	}
	defer podNS.Close()

	vxlan, err := podNS.LinkFind(ifName)
	if err != nil {
		return fmt.Errorf("failed to find vxlan interface %q on netns %s: %w", ifName, podNS.Path(), err)
	}

	device, err := vxlan.GetDevice()
	if err != nil {
		return fmt.Errorf("failed to get device info of %s: %w", ifName, err)
	}

	vxlanDevice, ok := device.(*netops.VXLAN)
	if !ok {
		return fmt.Errorf("not a VXLAN interface: %s", ifName)
	}

	dstAddr := vxlanDevice.Group
	dstPort := vxlanDevice.Port
	vxlanID := vxlanDevice.ID

	if err := vxlan.Delete(); err != nil {
		return fmt.Errorf("failed to delete vxlan interface %s at %s: %w", secondPodInterface, podNS.Path(), err)
	}

	if err := iptablesTeardown(hostNS, dstAddr, dstPort, vxlanID); err != nil {
		return err
	}

	return nil
}
