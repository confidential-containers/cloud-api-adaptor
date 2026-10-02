// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package podvm

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/containerd/ttrpc"
	agent "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testForwarderServerName = "podvm-test"
	testContainerImage      = "quay.io/prometheus/busybox@sha256:f07fff1d7544464a1041fc2b6be79873a335a3af8f0b747b82fa630287968d4d"
)

func TestDebugPodVM(t *testing.T) {
	image := os.Getenv("TEST_PODVM_IMAGE")
	if image == "" {
		t.Skip("TEST_PODVM_IMAGE is not set")
	}

	image, err := filepath.Abs(image)
	require.NoError(t, err)
	require.FileExists(t, image)
	requireDevice(t, "/dev/kvm")
	requireDevice(t, "/dev/vhost-vsock")

	qemuBinary := os.Getenv("TEST_PODVM_QEMU_BINARY")
	if qemuBinary == "" {
		qemuBinary = "qemu-system-x86_64"
	}
	qemuBinary, err = exec.LookPath(qemuBinary)
	require.NoError(t, err, "find QEMU")
	_, err = exec.LookPath("qemu-img")
	require.NoError(t, err, "find qemu-img")

	ovmfCode, ovmfVars, err := findOVMF()
	require.NoError(t, err)
	cid, err := guestCID()
	require.NoError(t, err)
	signer := newSSHSigner(t)
	forwarderConfig, forwarderClientTLS := newForwarderFixture(t)
	forwarderJSON, err := json.Marshal(forwarderConfig)
	require.NoError(t, err)

	vm, err := startQEMU(qemuConfig{
		image:         image,
		qemuBinary:    qemuBinary,
		ovmfCode:      ovmfCode,
		ovmfVars:      ovmfVars,
		guestCID:      cid,
		authorizedKey: ssh.MarshalAuthorizedKey(signer.PublicKey()),
		userData: &cloudinit.CloudConfig{
			WriteFiles: []cloudinit.WriteFile{
				{
					Path:    forwarder.DefaultConfigPath,
					Content: string(forwarderJSON),
				},
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(vm.close)

	bootTimeout := 2 * time.Minute
	if value := os.Getenv("TEST_PODVM_BOOT_TIMEOUT"); value != "" {
		bootTimeout, err = time.ParseDuration(value)
		require.NoError(t, err, "parse TEST_PODVM_BOOT_TIMEOUT")
	}

	bootCtx, cancelBoot := context.WithTimeout(context.Background(), bootTimeout)
	defer cancelBoot()
	client, err := vm.waitForSSH(bootCtx, signer)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	t.Run("ExecuteCommandOverVSock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := vm.runSSHCommand(ctx, signer, "printf podvm-ready")
		require.NoError(t, err)
		require.Equal(t, "podvm-ready", string(output))
	})

	t.Run("ConfigDriveMounted", func(t *testing.T) {
		var output []byte
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := retryUntil(ctx, "mounted config drive", func(ctx context.Context) error {
			output, err = vm.runSSHCommand(ctx, signer,
				"findmnt --evaluate --mountpoint /media/cidata --source LABEL=cidata --types iso9660 --noheadings --output TARGET")
			return err
		})
		require.NoError(t, err, "findmnt output: %s", output)
		require.Equal(t, "/media/cidata\n", string(output))
	})

	t.Run("ProvisionUserData", func(t *testing.T) {
		var output []byte
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		err := retryUntil(ctx, "provisioned user data", func(ctx context.Context) error {
			output, err = vm.runSSHCommand(ctx, signer, "cat "+forwarder.DefaultConfigPath)
			return err
		})
		require.NoError(t, err)
		require.JSONEq(t, string(forwarderJSON), string(output))
	})

	t.Run("SystemdServices", func(t *testing.T) {
		services := []struct {
			name string
			unit string
		}{
			{name: "ProcessUserData", unit: "process-user-data.service"},
			{name: "AttestationAgent", unit: "attestation-agent.service"},
			{name: "ConfidentialDataHub", unit: "confidential-data-hub.service"},
			{name: "APIServerREST", unit: "api-server-rest.service"},
			{name: "KataAgent", unit: "kata-agent.service"},
		}

		for _, service := range services {
			t.Run(service.name, func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				require.NoError(t, waitForSystemdService(ctx, vm, signer, service.unit))
			})
		}
	})

	t.Run("ForwarderMutualTLS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, waitForForwarderTLS(ctx, vm.forwarderAddr, forwarderClientTLS))
	})

	t.Run("KataAgentHealth", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, waitForKataAgent(ctx, vm.forwarderAddr, forwarderClientTLS))
	})

	t.Run("SandboxLifecycle", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, err := dialForwarderTLS(ctx, vm.forwarderAddr, forwarderClientTLS)
		require.NoError(t, err)
		ttrpcClient := ttrpc.NewClient(conn)
		defer ttrpcClient.Close()

		agentClient := agent.NewAgentServiceClient(ttrpcClient)
		_, err = agentClient.CreateSandbox(ctx, &agent.CreateSandboxRequest{
			Hostname:  "podvm-test",
			SandboxId: "podvm-test",
		})
		require.NoError(t, err)

		defer func() {
			destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer destroyCancel()
			_, err := agentClient.DestroySandbox(destroyCtx, &agent.DestroySandboxRequest{})
			// TODO: there is race in the ttrpc client that will discard responses after
			// a connection is closed.
			if err != nil {
				expectedClose := status.Code(err) == codes.Unknown &&
					status.Convert(err).Message() == ttrpc.ErrClosed.Error()
				require.True(t, expectedClose, "DestroySandbox failed: %v", err)
			}
		}()

		t.Run("LaunchContainers", func(t *testing.T) {
			image := os.Getenv("TEST_PODVM_CONTAINER_IMAGE")
			if image == "" {
				image = testContainerImage
			}

			t.Run("Message", func(t *testing.T) {
				output := runGuestContainer(
					t,
					agentClient,
					vm,
					signer,
					image,
					"podvm-test-message",
					"printf podvm-container-ready > /tmp/result",
					"/tmp/result",
				)
				require.Equal(t, "podvm-container-ready", string(output))
			})

			t.Run("GetEvidence", func(t *testing.T) {
				evidence := runGuestContainer(
					t,
					agentClient,
					vm,
					signer,
					image,
					"podvm-test-evidence",
					"wget -qO /tmp/evidence 'http://127.0.0.1:8006/aa/evidence?runtime_data=podvm-test'",
					"/tmp/evidence",
				)
				require.JSONEq(t, `{"svn":"1","report_data":"cG9kdm0tdGVzdA=="}`, string(evidence))
			})
		})
	})
}

func runGuestContainer(
	t *testing.T,
	agentClient agent.AgentServiceService,
	vm *qemuVM,
	signer ssh.Signer,
	image string,
	containerID string,
	command string,
	outputPath string,
) []byte {
	t.Helper()

	const sandboxID = "podvm-test"
	rootfs := "/run/kata-containers/" + containerID + "/rootfs"
	bundle := "/run/kata-containers/" + containerID
	annotations := map[string]string{
		"io.katacontainers.pkg.oci.bundle_path":    bundle,
		"io.katacontainers.pkg.oci.container_type": "pod_container",
		"io.kubernetes.cri.container-type":         "container",
		"io.kubernetes.cri.image-name":             image,
		"io.kubernetes.cri.sandbox-id":             sandboxID,
	}
	driverOptions, err := json.Marshal(map[string]any{"metadata": annotations})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err = agentClient.CreateContainer(ctx, &agent.CreateContainerRequest{
		ContainerId: containerID,
		ExecId:      containerID,
		Storages: []*agent.Storage{
			{
				Driver:        "image_guest_pull",
				DriverOptions: []string{"image_guest_pull=" + string(driverOptions)},
				Source:        image,
				Fstype:        "overlay",
				MountPoint:    rootfs,
			},
		},
		OCI: &agent.Spec{
			Version: "1.1.0",
			Process: &agent.Process{
				User: &agent.User{
					AdditionalGids: []uint32{0},
				},
				Args: []string{
					"/bin/sh",
					"-c",
					command,
				},
				Env:          []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
				Cwd:          "/",
				Capabilities: &agent.LinuxCapabilities{},
			},
			Root: &agent.Root{Path: rootfs},
			Mounts: []*agent.Mount{
				{
					Destination: "/proc",
					Source:      "proc",
					Type:        "proc",
					Options:     []string{"nosuid", "noexec", "nodev"},
				},
				{
					Destination: "/dev",
					Source:      "tmpfs",
					Type:        "tmpfs",
					Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
				},
				{
					Destination: "/sys",
					Source:      "sysfs",
					Type:        "sysfs",
					Options:     []string{"nosuid", "noexec", "nodev", "ro"},
				},
			},
			Annotations: annotations,
			Linux: &agent.Linux{
				Namespaces: []*agent.LinuxNamespace{
					{Type: "ipc"},
					{Type: "uts"},
					{Type: "mount"},
				},
			},
		},
	})
	require.NoError(t, err)

	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer removeCancel()
		_, err := agentClient.RemoveContainer(removeCtx, &agent.RemoveContainerRequest{
			ContainerId: containerID,
			Timeout:     10,
		})
		require.NoError(t, err)
	}()

	_, err = agentClient.StartContainer(ctx, &agent.StartContainerRequest{ContainerId: containerID})
	require.NoError(t, err)

	response, err := agentClient.WaitProcess(ctx, &agent.WaitProcessRequest{
		ContainerId: containerID,
		ExecId:      containerID,
	})
	require.NoError(t, err)
	require.Zero(t, response.Status)

	outputCtx, outputCancel := context.WithTimeout(ctx, 10*time.Second)
	defer outputCancel()
	output, err := vm.runSSHCommand(outputCtx, signer, "cat "+rootfs+outputPath)
	require.NoError(t, err)
	return output
}

func newSSHSigner(t *testing.T) ssh.Signer {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	return signer
}

func requireDevice(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeDevice, "%s is not a device", path)

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err, "%s is not accessible", path)
	require.NoError(t, file.Close())
}

func newForwarderFixture(t *testing.T) (*forwarder.Config, *tls.Config) {
	t.Helper()

	var config forwarder.Config
	require.NoError(t, json.Unmarshal([]byte(testForwarderConfig), &config))

	serverCA, err := tlsutil.NewCAService("podvm-test-forwarder")
	require.NoError(t, err)
	serverCert, serverKey, err := serverCA.Issue(testForwarderServerName)
	require.NoError(t, err)
	clientCert, clientKey, err := tlsutil.NewClientCertificate("podvm-test-client")
	require.NoError(t, err)

	config.TLSServerCert = string(serverCert)
	config.TLSServerKey = string(serverKey)
	config.TLSClientCA = string(clientCert)

	clientTLS, err := tlsutil.GetTLSConfigFor(&tlsutil.TLSConfig{
		CAData:   serverCA.RootCertificate(),
		CertData: clientCert,
		KeyData:  clientKey,
	})
	require.NoError(t, err)
	clientTLS.ServerName = testForwarderServerName

	return &config, clientTLS
}

func waitForForwarderTLS(ctx context.Context, address string, config *tls.Config) error {
	return retryUntil(ctx, "agent protocol forwarder TLS", func(ctx context.Context) error {
		conn, err := dialForwarderTLS(ctx, address, config)
		if err != nil {
			return err
		}
		return conn.Close()
	})
}

func waitForKataAgent(ctx context.Context, address string, config *tls.Config) error {
	return retryUntil(ctx, "kata agent health", func(ctx context.Context) error {
		conn, err := dialForwarderTLS(ctx, address, config)
		if err != nil {
			return err
		}

		ttrpcClient := ttrpc.NewClient(conn)
		defer ttrpcClient.Close()
		_, err = agent.NewHealthClient(ttrpcClient).Check(ctx, &agent.CheckRequest{})
		return err
	})
}

func dialForwarderTLS(ctx context.Context, address string, config *tls.Config) (*tls.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, config.Clone())
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}
