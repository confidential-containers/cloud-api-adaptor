//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
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

func (l LibvirtAssert) VerifyPodvmConsole(t *testing.T, podvmName, expectedString string) {

	var dom *libvirt.Domain
	var err error
	for range 10 {
		dom, err = l.conn.LookupDomainByName(podvmName)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if dom == nil {
		t.Error("PodVM was not created")
	}

	state, _, err := dom.GetState()
	if err != nil {
		t.Error("Failed to get domain state")
	}

	if state == libvirt.DOMAIN_SHUTOFF {
		log.Info("starting podvm")
		err = dom.Create()
		if err != nil {
			t.Error("Failed to start domain")
		}

	}

	stream, err := l.conn.NewStream(0)
	if err != nil {
		t.Errorf("Failed to create stream : %v", err)
	}

	defer stream.Free()

	err = dom.OpenConsole("", stream, libvirt.DOMAIN_CONSOLE_FORCE)
	if err != nil {
		t.Errorf("Failed to open console : %v", err)
	}

	buf := make([]byte, 4096)
	var output strings.Builder
	var LibvirtLog = ""
	for range [10]int{} {
		n, err := stream.Recv(buf)
		if n > 0 {
			output.Write(buf[:n])
			if len(output.String()) > len(LibvirtLog) {
				LibvirtLog = output.String()
			}
			if strings.Contains(LibvirtLog, expectedString) {
				t.Logf("Found expected String :%s in \n console :%s", expectedString, LibvirtLog)
				return
			}
		}
		if err != nil && LibvirtLog != "" {
			t.Logf("Warning: Did not find expected String :%s in \n console :%s", expectedString, LibvirtLog)
			return
		} else if err != nil {
			t.Logf("Warning: Did not receive any data from console yet, err: %v", err)
			time.Sleep(6 * time.Second)
		}
	}
}
