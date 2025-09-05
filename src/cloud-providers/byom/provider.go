// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"time"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/byom] ", log.LstdFlags|log.Lmsgprefix)

const (
	sshPort      = "22"
	userDataFile = "/media/cidata/user-data" // User-data file
	rebootFile   = "/media/cidata/reboot"    // Reboot trigger file
)

// byomProvider implements the Provider interface for BYOM
type byomProvider struct {
	serviceConfig *Config
	globalPoolMgr GlobalVMPoolManager
}

// NewProvider creates a new BYOM provider instance
func NewProvider(config *Config) (provider.Provider, error) {
	logger.Printf("BYOM config: %+v", config.Redact())

	// Initialize SSH keys for authentication
	sshConfig := &util.SSHConfig{
		PublicKey:      config.SSHPubKey,
		PrivateKey:     config.SSHPrivKey,
		PublicKeyPath:  config.SSHPubKeyPath,
		PrivateKeyPath: config.SSHPrivKeyPath,
		Username:       config.SSHUserName,
		EnableSFTP:     true, // Always enabled for BYOM
	}

	if err := util.InitializeSSHKeys(sshConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize SSH keys: %w", err)
	}

	// Update config with initialized keys
	config.SSHPubKey = sshConfig.PublicKey
	config.SSHPrivKey = sshConfig.PrivateKey

	// Initialize Kubernetes client for in-cluster usage
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Determine namespace for ConfigMap storage
	poolNamespace := config.PoolNamespace
	if poolNamespace == "" {
		// Auto-detect namespace from running pod
		poolNamespace = getCurrentNamespaceWithDefault()
	}

	// Determine ConfigMap name
	configMapName := config.PoolConfigMapName
	if configMapName == "" {
		configMapName = "byom-ip-pool-state"
	}

	// Create global pool configuration
	poolConfig := &GlobalVMPoolConfig{
		Namespace:        poolNamespace,
		ConfigMapName:    configMapName,
		PoolIPs:          config.VMPoolIPs,
		MaxRetries:       5,
		RetryInterval:    100 * time.Millisecond,
		OperationTimeout: 30 * time.Second,
	}

	logger.Printf("Pool configuration: namespace=%s, configMap=%s, IPs=%d",
		poolNamespace, configMapName, len(config.VMPoolIPs))

	// Create ConfigMap-based pool manager
	globalPoolMgr, err := NewConfigMapVMPoolManager(kubeClient, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create global pool manager: %w", err)
	}

	return NewProviderWithComponents(config, globalPoolMgr)
}

// NewProviderWithComponents creates a BYOM provider with custom components (for testing)
func NewProviderWithComponents(config *Config, globalPoolMgr GlobalVMPoolManager) (provider.Provider, error) {
	if globalPoolMgr == nil {
		return nil, fmt.Errorf("globalPoolMgr must be provided - old VMPool is no longer supported")
	}

	p := &byomProvider{
		serviceConfig: config,
		globalPoolMgr: globalPoolMgr,
	}

	// Initialize state recovery
	ctx := context.Background()
	if err := p.globalPoolMgr.RecoverState(ctx); err != nil {
		logger.Printf("Warning: failed to recover state: %v", err)
	}

	// Log pool status
	total, available, inUse, err := p.globalPoolMgr.GetPoolStatus(ctx)
	if err != nil {
		logger.Printf("Warning: failed to get pool status: %v", err)
	} else {
		logger.Printf("Initialized BYOM provider with %d VMs (%d available, %d in use)", total, available, inUse)
	}

	return p, nil
}

// CreateInstance allocates a VM from the pool and configures it
func (p *byomProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {
	// Generate allocation ID
	allocationID := fmt.Sprintf("%s-%s", podName, sandboxID)

	// Parse pod name to extract namespace (format: namespace/podname or just podname)
	podNamespace := "default" // default namespace
	actualPodName := podName
	if parts := strings.Split(podName, "/"); len(parts) == 2 {
		podNamespace = parts[0]
		actualPodName = parts[1]
	}

	// Allocate IP from global pool
	ip, err := p.globalPoolMgr.AllocateIP(ctx, allocationID, actualPodName, podNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP from pool: %w", err)
	}

	// Generate cloud config data
	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		// Rollback allocation on error
		if rollbackErr := p.globalPoolMgr.DeallocateIP(ctx, allocationID); rollbackErr != nil {
			logger.Printf("Warning: failed to rollback IP allocation: %v", rollbackErr)
		}
		return nil, fmt.Errorf("failed to generate cloud config: %w", err)
	}

	// Send config to the VM via SFTP
	if err := p.sendConfigFile(cloudConfigData, ip); err != nil {
		// Rollback allocation on error
		if rollbackErr := p.globalPoolMgr.DeallocateIP(ctx, allocationID); rollbackErr != nil {
			logger.Printf("Warning: failed to rollback IP allocation: %v", rollbackErr)
		}
		return nil, fmt.Errorf("failed to send config to VM %s: %w", ip.String(), err)
	}

	// Note: No need to update PeerPod CR - it will automatically contain the IP in spec.instanceID
	// when the instance is created by the hypervisor service

	// Create instance object
	instance := &provider.Instance{
		ID:   ip.String(), // Use IP as instance ID for BYOM
		Name: fmt.Sprintf("byom-%s", ip.String()),
		IPs:  []netip.Addr{ip},
	}

	// Log current pool status
	total, available, inUse, err := p.globalPoolMgr.GetPoolStatus(ctx)
	if err != nil {
		logger.Printf("Warning: failed to get pool status: %v", err)
	} else {
		logger.Printf("Created instance %s: total=%d, available=%d, inUse=%d", instance.ID, total, available, inUse)
	}

	return instance, nil
}

// DeleteInstance returns a VM back to the pool
func (p *byomProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	// Parse instance ID (which is the IP address)
	ip, err := netip.ParseAddr(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID %s: %w", instanceID, err)
	}

	// Send reboot trigger file to VM before deallocating
	if err := p.sendRebootFile(ip); err != nil {
		logger.Printf("Warning: failed to send reboot file to VM %s: %v", ip.String(), err)
		// Continue with deallocation even if reboot file sending fails
	}

	// Return IP to global pool
	if err := p.globalPoolMgr.DeallocateByIP(ctx, ip); err != nil {
		return fmt.Errorf("failed to deallocate IP %s: %w", ip.String(), err)
	}

	logger.Printf("Returned VM to pool (not deleted): IP=%s", ip.String())

	// Log current pool status
	total, available, inUse, err := p.globalPoolMgr.GetPoolStatus(ctx)
	if err != nil {
		logger.Printf("Warning: failed to get pool status: %v", err)
	} else {
		logger.Printf("Pool status after deallocation: total=%d, available=%d, inUse=%d", total, available, inUse)
	}

	return nil
}

// Teardown cleans up resources
func (p *byomProvider) Teardown() error {
	logger.Printf("BYOM provider teardown completed")
	return nil
}

// ConfigVerifier validates the provider configuration
func (p *byomProvider) ConfigVerifier() error {
	if len(p.serviceConfig.VMPoolIPs) == 0 {
		return fmt.Errorf("vm-pool-ips is required and cannot be empty")
	}

	if p.serviceConfig.SSHUserName == "" {
		return fmt.Errorf("ssh-username is required")
	}

	if p.serviceConfig.SSHPrivKey == "" {
		return fmt.Errorf("SSH private key is required")
	}

	// Test SSH connectivity to all VMs using common utility
	logger.Printf("Verifying SSH connectivity to %d VMs...", len(p.serviceConfig.VMPoolIPs))

	sshClientConfig := &util.SSHClientConfig{
		Username:              p.serviceConfig.SSHUserName,
		PrivateKey:            p.serviceConfig.SSHPrivKey,
		Timeout:               time.Duration(p.serviceConfig.SSHTimeout) * time.Second,
		InsecureIgnoreHostKey: p.serviceConfig.SSHInsecureIgnoreHostKey,
	}

	sshConfig, err := util.CreateSSHClientConfig(sshClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	if err := util.ValidateSSHConnectivityToIPs(p.serviceConfig.VMPoolIPs, sshPort, sshConfig); err != nil {
		return err
	}

	logger.Printf("SSH connectivity verified for all %d VMs", len(p.serviceConfig.VMPoolIPs))
	return nil
}

// sendConfigFile sends cloud-init user-data to a VM via SFTP using common utility
func (p *byomProvider) sendConfigFile(userData string, ip netip.Addr) error {
	sshClientConfig := &util.SSHClientConfig{
		Username:              p.serviceConfig.SSHUserName,
		PrivateKey:            p.serviceConfig.SSHPrivKey,
		Timeout:               time.Duration(p.serviceConfig.SSHTimeout) * time.Second,
		InsecureIgnoreHostKey: p.serviceConfig.SSHInsecureIgnoreHostKey,
	}

	sshConfig, err := util.CreateSSHClientConfig(sshClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	address := net.JoinHostPort(ip.String(), sshPort)
	if err := p.sendFileViaSFTPWithChroot(address, sshConfig, userDataFile, []byte(userData)); err != nil {
		return fmt.Errorf("failed to send user-data to VM %s: %w", ip.String(), err)
	}

	logger.Printf("Successfully sent user-data to VM %s", ip.String())
	return nil
}

// sendRebootFile sends a reboot trigger file to a VM via SFTP
func (p *byomProvider) sendRebootFile(ip netip.Addr) error {
	sshClientConfig := &util.SSHClientConfig{
		Username:              p.serviceConfig.SSHUserName,
		PrivateKey:            p.serviceConfig.SSHPrivKey,
		Timeout:               time.Duration(p.serviceConfig.SSHTimeout) * time.Second,
		InsecureIgnoreHostKey: p.serviceConfig.SSHInsecureIgnoreHostKey,
	}

	sshConfig, err := util.CreateSSHClientConfig(sshClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	address := net.JoinHostPort(ip.String(), sshPort)
	rebootData := []byte("reboot")
	if err := p.sendFileViaSFTPWithChroot(address, sshConfig, rebootFile, rebootData); err != nil {
		return fmt.Errorf("failed to send reboot file to VM %s: %w", ip.String(), err)
	}

	logger.Printf("Successfully sent reboot trigger to VM %s", ip.String())
	return nil
}

// sendFileViaSFTPWithChroot sends a file via SFTP, stripping the /media prefix for chrooted SFTP
func (p *byomProvider) sendFileViaSFTPWithChroot(address string, sshConfig *ssh.ClientConfig, remotePath string, content []byte) error {
	// Strip /media prefix for chrooted SFTP (SFTP server chroots to /media)
	adjustedPath := remotePath
	if after, found := strings.CutPrefix(remotePath, "/media/"); found {
		adjustedPath = after
		logger.Printf("Adjusted SFTP path from %s to %s for chrooted environment", remotePath, adjustedPath)
	}

	return util.SendFileViaSFTP(address, sshConfig, adjustedPath, content)
}
