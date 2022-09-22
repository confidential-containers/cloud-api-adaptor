// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package vxlan

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
	"github.com/vishvananda/netlink"
)

var logger = log.New(log.Writer(), "[tunneler/vxlan] ", log.LstdFlags|log.Lmsgprefix)

const (
	DefaultVXLANPort         = 4789
	DefaultVXLANMinID        = 555000
	hostVxlanInterfacePrefix = "ppvxlan"
	secondPodInterface       = "vxlan1"
)

type workerNodeTunneler struct {
}

func NewWorkerNodeTunneler() tunneler.Tunneler {
	return &workerNodeTunneler{}
}

func (t *workerNodeTunneler) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {

	var dstAddr net.IP

	if config.Dedicated {
		dstAddr = podNodeIPs[1]
	} else {
		dstAddr = podNodeIPs[0]
	}

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	index := 1

	links, err := hostNS.LinkList()
	if err != nil {
		return fmt.Errorf("failed to get interfaces on host: %w", err)
	}

	var hostVxlanInterface string

	for {
		hostVxlanInterface = fmt.Sprintf("%s%d", hostVxlanInterfacePrefix, index)
		var found bool
		for _, link := range links {
			if link.Attrs().Name == hostVxlanInterface {
				found = true
				break
			}
		}

		if !found {

			vxlanLink := &netlink.Vxlan{
				Group:   dstAddr,
				VxlanId: config.VXLANID,
				Port:    config.VXLANPort,
			}
			logger.Printf("vxlan %s (remote %s:%d, id: %d) created at %s", hostVxlanInterface, dstAddr.String(), config.VXLANPort, config.VXLANID, hostNS.Path)
			err := hostNS.LinkAdd(hostVxlanInterface, vxlanLink)
			if err == nil {
				logger.Printf("vxlan %s created at %s", hostVxlanInterface, hostNS.Path)
				break
			}
			logger.Printf("vxlan %s created at %s: %v", hostVxlanInterface, hostNS.Path, err)
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("failed to add vxlan interface %s: %w", hostVxlanInterface, err)
			}
		}
		index++
		if index > 5 {
			return fmt.Errorf("failed to create vxlan interface %s: too many", hostVxlanInterface)
		}
	}

	if err := hostNS.LinkSetNS(hostVxlanInterface, podNS); err != nil {
		return fmt.Errorf("failed to move vxlan interface %s to netns %s: %w", hostVxlanInterface, podNS.Path, err)
	}
	logger.Printf("vxlan %s is moved to %s", hostVxlanInterface, podNS.Path)

	if err := podNS.LinkSetName(hostVxlanInterface, secondPodInterface); err != nil {
		return fmt.Errorf("failed to change vxlan interface name %s on netns %s to %s: %w", hostVxlanInterface, podNS.Path, secondPodInterface, err)
	}

	if err := podNS.LinkSetUp(secondPodInterface); err != nil {
		return err
	}

	podInterface := config.InterfaceName

	logger.Printf("Add tc redirect filters between %s and %s on pod network namespace %s", podInterface, secondPodInterface, nsPath)

	if err := podNS.RedirectAdd(podInterface, secondPodInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", podInterface, secondPodInterface, err)
	}

	if err := podNS.RedirectAdd(secondPodInterface, podInterface); err != nil {
		return fmt.Errorf("failed to add a tc redirect filter from %s to %s: %w", secondPodInterface, podInterface, err)
	}

	return nil
}

func (t *workerNodeTunneler) Teardown(nsPath, hostInterface string, config *tunneler.Config) error {

	hostNS, err := netops.GetNS()
	if err != nil {
		return fmt.Errorf("failed to get current network namespace: %w", err)
	}
	defer func() {
		if e := hostNS.Close(); e != nil {
			err = fmt.Errorf("failed to close the original network namespace: %w (previous error: %v)", e, err)
		}
	}()

	podNS, err := netops.NewNSFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("failed to get a network namespace: %s: %w", nsPath, err)
	}
	defer func() {
		if e := podNS.Close(); e != nil {
			err = fmt.Errorf("failed close the pod network namespace: %w (previous error: %v)", e, err)
		}
	}()

	logger.Printf("Delete tc redirect filters on %s and %s in the network namespace %s", config.InterfaceName, hostInterface, nsPath)

	if err := podNS.RedirectDel(config.InterfaceName); err != nil {
		return fmt.Errorf("failed to delete a tc redirect filter from %s to %s: %w", config.InterfaceName, secondPodInterface, err)
	}

	if err := podNS.RedirectDel(secondPodInterface); err != nil {
		return fmt.Errorf("failed to delete a tc redirect filter from %s to %s: %w", secondPodInterface, config.InterfaceName, err)
	}

	logger.Printf("Delete vxlan interface %s in the network namespace %s", secondPodInterface, nsPath)

	if err := podNS.LinkDel(secondPodInterface); err != nil {
		return fmt.Errorf("failed to delete vxlan interface %s at %s: %w", secondPodInterface, podNS.Name, err)
	}
	return nil
}
