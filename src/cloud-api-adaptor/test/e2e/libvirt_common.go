//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strings"
	"testing"
	"time"

	"libvirt.org/go/libvirt"
)

// LibvirtAssert implements the CloudAssert interface for Libvirt.
type LibvirtAssert struct {
	// TODO: create the connection once on the initializer.
	//conn libvirt.Connect
}

func (c LibvirtAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (l LibvirtAssert) HasPodVM(t *testing.T, id string) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		t.Fatal(err)
	}

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	for _, dom := range domains {
		name, _ := dom.GetName()
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			return
		}
	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
}

func (l LibvirtAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	return "", nil
}

// Get the PodVM's CPU and memory resources
func (l LibvirtAssert) GetPodVMResource(t *testing.T, id string) (uint, uint, error) {
	// Get the resources of PodVM
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		t.Fatal(err)
	}

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	for _, dom := range domains {
		name, _ := dom.GetName()
		// TODO: PodVM name is podvm-POD_NAME-SANDBOX_ID, where SANDBOX_ID is truncated
		// in the 8th word. Ideally we should match the exact name, not just podvm-POD_NAME.
		if strings.HasPrefix(name, strings.Join([]string{"podvm", id, ""}, "-")) {
			info, _ := dom.GetInfo()
			mem := (uint)(info.MaxMem / 1024)
			mem = mem / 1024
			return info.NrVirtCpu, mem, nil
		}

	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
	return 0, 0, nil
}
