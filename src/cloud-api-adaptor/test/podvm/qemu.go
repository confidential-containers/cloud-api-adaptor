// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package podvm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/mdlayher/vsock"
	"golang.org/x/crypto/ssh"
)

const (
	defaultGuestCID = 3
	sshPort         = 22
)

type qemuConfig struct {
	image         string
	qemuBinary    string
	ovmfCode      string
	ovmfVars      string
	guestCID      uint32
	authorizedKey []byte
	userData      cloudinit.CloudConfigGenerator
}

type qemuVM struct {
	cmd           *exec.Cmd
	done          chan struct{}
	waitErr       error
	guestCID      uint32
	qemuLog       string
	serialLog     string
	workDir       string
	hostKey       ssh.PublicKey
	forwarderAddr string
}

type imageInfo struct {
	Format string `json:"format"`
}

func startQEMU(config qemuConfig) (_ *qemuVM, err error) {
	format, err := getImageFormat(config.image)
	if err != nil {
		return nil, err
	}

	workDir, err := os.MkdirTemp("", "podvm-test-")
	if err != nil {
		return nil, fmt.Errorf("create temporary directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(workDir)
		}
	}()

	ovmfVars := filepath.Join(workDir, "OVMF_VARS.fd")
	if err := copyFile(config.ovmfVars, ovmfVars); err != nil {
		return nil, fmt.Errorf("copy OVMF variables: %w", err)
	}

	serialLog := filepath.Join(workDir, "serial.log")
	qemuLog := filepath.Join(workDir, "qemu.log")
	configDrive := filepath.Join(workDir, "config-drive.iso")
	if err := createConfigDrive(configDrive, config.userData); err != nil {
		return nil, err
	}
	logFile, err := os.Create(qemuLog)
	if err != nil {
		return nil, fmt.Errorf("create QEMU log: %w", err)
	}
	defer logFile.Close()

	forwarderListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("reserve agent protocol forwarder port: %w", err)
	}
	defer forwarderListener.Close()
	forwarderAddr := forwarderListener.Addr().String()
	_, forwarderPort, err := net.SplitHostPort(forwarderAddr)
	if err != nil {
		return nil, fmt.Errorf("parse agent protocol forwarder address: %w", err)
	}

	args := []string{
		"-machine", "q35,accel=kvm",
		"-cpu", "host",
		"-m", "2048",
		"-smp", "2",
		"-drive", fmt.Sprintf("if=pflash,format=raw,readonly=on,file=%s", config.ovmfCode),
		"-drive", fmt.Sprintf("if=pflash,format=raw,file=%s", ovmfVars),
		"-drive", fmt.Sprintf("if=virtio,format=%s,file=%s,snapshot=on", format, config.image),
		"-drive", fmt.Sprintf("file=%s,media=cdrom,readonly=on", configDrive),
		"-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", config.guestCID),
		"-device", "virtio-rng-pci",
		"-netdev", fmt.Sprintf(
			"user,id=net0,hostfwd=tcp:127.0.0.1:%s-:%s",
			forwarderPort,
			forwarder.DefaultListenPort,
		),
		"-device", "virtio-net-pci,netdev=net0",
		"-display", "none",
		"-monitor", "none",
		"-serial", "file:" + serialLog,
		"-smbios", "type=11,value=io.systemd.credential.binary:ssh.ephemeral-authorized_keys-all=" +
			base64.StdEncoding.EncodeToString(config.authorizedKey),
	}

	cmd := exec.Command(config.qemuBinary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := forwarderListener.Close(); err != nil {
		return nil, fmt.Errorf("release agent protocol forwarder port: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start QEMU: %w", err)
	}

	vm := &qemuVM{
		cmd:           cmd,
		done:          make(chan struct{}),
		guestCID:      config.guestCID,
		qemuLog:       qemuLog,
		serialLog:     serialLog,
		workDir:       workDir,
		forwarderAddr: forwarderAddr,
	}
	go func() {
		vm.waitErr = cmd.Wait()
		close(vm.done)
	}()

	return vm, nil
}

func (vm *qemuVM) close() {
	if vm.cmd.Process != nil {
		_ = vm.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-vm.done:
		case <-time.After(5 * time.Second):
			_ = vm.cmd.Process.Kill()
			<-vm.done
		}
	}
	_ = os.RemoveAll(vm.workDir)
}

func (vm *qemuVM) waitForSSH(ctx context.Context, signer ssh.Signer) (*ssh.Client, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		client, err := vm.sshClient(signer)
		if err == nil {
			return client, nil
		}
		lastErr = err

		select {
		case <-vm.done:
			return nil, fmt.Errorf("QEMU exited before SSH became ready: %v\n%s", vm.waitErr, vm.logs())
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for SSH over vsock: %w (last error: %v)\n%s", ctx.Err(), lastErr, vm.logs())
		case <-ticker.C:
		}
	}
}

func (vm *qemuVM) sshClient(signer ssh.Signer) (*ssh.Client, error) {
	conn, err := dialVSock(vm.guestCID, sshPort)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(5 * time.Second)
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set vsock deadline: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: vm.verifyHostKey,
	}
	clientConn, channels, requests, err := ssh.NewClientConn(conn, fmt.Sprintf("vsock:%d", vm.guestCID), config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH handshake: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		clientConn.Close()
		return nil, fmt.Errorf("clear vsock deadline: %w", err)
	}

	return ssh.NewClient(clientConn, channels, requests), nil
}

func (vm *qemuVM) verifyHostKey(_ string, _ net.Addr, key ssh.PublicKey) error {
	if vm.hostKey == nil {
		// The dedicated AF_VSOCK CID identifies the VM for the first connection.
		// Pin its ephemeral key so retries cannot silently switch SSH endpoints.
		vm.hostKey = key
		return nil
	}
	if !bytes.Equal(vm.hostKey.Marshal(), key.Marshal()) {
		return fmt.Errorf(
			"SSH host key changed: expected %s, got %s",
			ssh.FingerprintSHA256(vm.hostKey),
			ssh.FingerprintSHA256(key),
		)
	}
	return nil
}

func (vm *qemuVM) logs() string {
	const maxLogSize = 64 * 1024

	var output string
	for _, logFile := range []string{vm.qemuLog, vm.serialLog} {
		data, err := os.ReadFile(logFile)
		if err != nil {
			output += fmt.Sprintf("read %s: %v\n", logFile, err)
			continue
		}
		if len(data) > maxLogSize {
			data = data[len(data)-maxLogSize:]
		}
		output += fmt.Sprintf("==> %s <==\n%s\n", logFile, data)
	}
	return output
}

func getImageFormat(image string) (string, error) {
	output, err := exec.Command("qemu-img", "info", "--output=json", image).Output()
	if err != nil {
		return "", fmt.Errorf("inspect podvm image: %w", err)
	}

	var info imageInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return "", fmt.Errorf("decode qemu-img output: %w", err)
	}
	switch info.Format {
	case "raw", "qcow2":
		return info.Format, nil
	default:
		return "", fmt.Errorf("unsupported podvm image format %q", info.Format)
	}
}

func findOVMF() (code, vars string, err error) {
	code = os.Getenv("TEST_PODVM_OVMF_CODE")
	vars = os.Getenv("TEST_PODVM_OVMF_VARS")
	if code != "" || vars != "" {
		if code == "" || vars == "" {
			return "", "", errors.New("TEST_PODVM_OVMF_CODE and TEST_PODVM_OVMF_VARS must be set together")
		}
		return code, vars, nil
	}

	candidates := [][2]string{
		{"/usr/share/OVMF/OVMF_CODE_4M.fd", "/usr/share/OVMF/OVMF_VARS_4M.fd"},
		{"/usr/share/OVMF/OVMF_CODE.fd", "/usr/share/OVMF/OVMF_VARS.fd"},
	}
	for _, candidate := range candidates {
		if fileExists(candidate[0]) && fileExists(candidate[1]) {
			return candidate[0], candidate[1], nil
		}
	}
	return "", "", errors.New("OVMF firmware not found; set TEST_PODVM_OVMF_CODE and TEST_PODVM_OVMF_VARS")
}

func guestCID() (uint32, error) {
	value := os.Getenv("TEST_PODVM_GUEST_CID")
	if value == "" {
		return defaultGuestCID, nil
	}
	cid, err := strconv.ParseUint(value, 10, 32)
	if err != nil || cid < 3 {
		return 0, fmt.Errorf("invalid TEST_PODVM_GUEST_CID %q: must be an integer greater than or equal to 3", value)
	}
	return uint32(cid), nil
}

func dialVSock(cid, port uint32) (*vsock.Conn, error) {
	conn, err := vsock.Dial(cid, port, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to vsock %d:%d: %w", cid, port, err)
	}
	return conn, nil
}

func copyFile(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
