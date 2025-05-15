//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strconv"
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
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		t.Fatal(err)
	}

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		t.Fatal(err)
	}
	// Get the CPU and Memory information of the matching PodVM
	for _, dom := range domains {
		name, _ := dom.GetName()
		// PodVM name is podvm-POD_NAME-SANDBOX_ID
		if strings.HasPrefix(name, strings.Join([]string{"podvm", podName, ""}, "-")) {
			info, _ := dom.GetInfo()
			mem := (uint)(info.MaxMem / 1024)
			cpuStr := strconv.FormatUint(uint64(info.NrVirtCpu), 10)
			memStr := strconv.FormatUint(uint64(mem), 10)
			return cpuStr + "x" + memStr, nil
		}
	}

	// It didn't find the PodVM if it reached here.
	t.Error("PodVM was not created")
	return "", nil
}

func CreateInstanceProfileFromCPUMemory(cpu uint, memory uint) string {
	cpuStr := strconv.FormatUint(uint64(cpu), 10)
	memStr := strconv.FormatUint(uint64(memory), 10)
	return cpuStr + "x" + memStr
}
