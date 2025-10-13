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
	conn libvirt.Connect
}

func NewLibvirtAssert() (*LibvirtAssert, error) {
	return NewLibvirtAssertWithURI("qemu:///system")
}

func NewLibvirtAssertWithURI(uri string) (*LibvirtAssert, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, err
	}
	return &LibvirtAssert{conn: *conn}, nil
}

func (c LibvirtAssert) DefaultTimeout() time.Duration {
	return 1 * time.Minute
}

func (l LibvirtAssert) HasPodVM(t *testing.T, podvmName string) {

	// Wait for the PodVM to be created for 30 seconds
	dom, err := l.conn.LookupDomainByName(podvmName)
	for range 10 {
		if err == nil {
			return
		}
		dom, err = l.conn.LookupDomainByName(podvmName)
		time.Sleep(3 * time.Second)
	}

	if dom == nil {
		t.Error("PodVM was not created")
	}
}

func (l LibvirtAssert) GetInstanceType(t *testing.T, podName string) (string, error) {
	// Get Instance Type of PodVM
	domains, err := l.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
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
