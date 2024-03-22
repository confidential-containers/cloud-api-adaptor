// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package podnetwork

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/netops"
)

type PodNode interface {
	Setup() error
	Teardown() error
}

type podNode struct {
	config        *tunneler.Config
	nsPath        string
	hostInterface string
}

func NewPodNode(nsPath string, hostInterface string, config *tunneler.Config) PodNode {

	podNode := &podNode{
		nsPath:        nsPath,
		hostInterface: hostInterface,
		config:        config,
	}

	return podNode
}

func (n *podNode) Setup() error {

	tun, err := tunneler.PodNodeTunneler(n.config.TunnelType)
	if err != nil {
		return fmt.Errorf("failed to get tunneler: %w", err)
	}

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	hostPrimaryInterface, err := detectPrimaryInterface(hostNS, 3*time.Minute)
	if err != nil {
		return err
	}

	primaryPodNodeIP, err := detectIP(hostNS, hostPrimaryInterface, 3*time.Minute)
	if err != nil {
		return err
	}

	podNodeIPs := []netip.Addr{primaryPodNodeIP}

	hostInterface := n.hostInterface
	if hostInterface == "" {
		hostInterface = hostPrimaryInterface
	}

	if n.config.Dedicated {
		if hostInterface == hostPrimaryInterface {
			return fmt.Errorf("%s is not a dedicated interface", hostInterface)
		}

		dedicatedPodNodeIP, err := detectIP(hostNS, hostInterface, 3*time.Minute)
		if err != nil {
			return err
		}

		podNodeIPs = append(podNodeIPs, dedicatedPodNodeIP)
	}

	podNS, err := netops.OpenNamespace(n.nsPath)
	if err != nil {
		return fmt.Errorf("failed to open network namespace %q: %w", n.nsPath, err)
	}
	defer func() {
		if err := podNS.Close(); err != nil {
			logger.Printf("failed to close a network namespace: %q", podNS.Path())
		}
	}()

	if err := tun.Setup(n.nsPath, podNodeIPs, n.config); err != nil {
		return fmt.Errorf("failed to set up tunnel %q: %w", n.config.TunnelType, err)
	}

	return nil
}

func (n *podNode) Teardown() error {

	tun, err := tunneler.PodNodeTunneler(n.config.TunnelType)
	if err != nil {
		return fmt.Errorf("failed to get tunneler: %w", err)
	}

	hostNS, err := netops.OpenCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to open the host network namespace: %w", err)
	}
	defer func() {
		if err := hostNS.Close(); err != nil {
			logger.Printf("failed to close the host network namespace: %v", err)
		}
	}()

	hostInterface := n.hostInterface
	if hostInterface == "" {
		hostPrimaryInterface, err := findPrimaryInterface(hostNS)
		if err != nil {
			return fmt.Errorf("failed to identify the host primary interface: %w", err)
		}
		hostInterface = hostPrimaryInterface
	}

	if err := tun.Teardown(n.nsPath, hostInterface, n.config); err != nil {
		return fmt.Errorf("failed to tear down tunnel %q: %w", n.config.TunnelType, err)
	}

	return nil
}

func detectPrimaryInterface(hostNS netops.Namespace, timeout time.Duration) (string, error) {

	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {

		hostPrimaryInterface, err := findPrimaryInterface(hostNS)
		if err == nil {
			return hostPrimaryInterface, nil
		}

		select {
		case <-timeoutCh:
			return "", fmt.Errorf("failed to identify primary interface on netns %s", hostNS.Path())
		case <-ticker.C:
		}

		logger.Printf("failed to identify the host primary interface: %v (retrying...)", err)
	}
}

func detectIP(hostNS netops.Namespace, hostInterface string, timeout time.Duration) (netip.Addr, error) {

	// An IP address of the second network interface of an IBM Cloud VPC instance is assigned by DHCP
	// several seconds after the first interface gets an IP address.

	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {

		hostLink, err := hostNS.LinkFind(hostInterface)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("failed to find host interface %q on netns %s: %w", hostInterface, hostNS.Path(), err)
		}

		prefixes, err := hostLink.GetAddr()
		if err != nil {
			return netip.Addr{}, fmt.Errorf("failed to get addresses assigned %s on netns %s: %w", hostLink.Name(), hostLink.Namespace().Path(), err)
		}
		if len(prefixes) > 1 {
			return netip.Addr{}, fmt.Errorf("more than one IP address assigned on %s (netns: %s)", hostLink.Name(), hostLink.Namespace().Path())
		}
		if len(prefixes) == 1 {
			return prefixes[0].Addr(), nil
		}

		select {
		case <-timeoutCh:
			return netip.Addr{}, fmt.Errorf("failed to identify IP address assigned to host interface %s on netns %s", hostLink.Name(), hostLink.Namespace().Path())
		case <-ticker.C:
		}
	}
}
