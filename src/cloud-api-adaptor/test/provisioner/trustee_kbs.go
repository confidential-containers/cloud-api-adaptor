// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	kbsNamespace       = "coco-trustee"
	kbsReleaseName     = "trustee"
	kbsServiceName     = "trustee-kbs"
	kbsNodePortSvcName = "trustee-kbs-nodeport"
	kbsDeploymentName  = "trustee-kbs"
	kbsBootstrapSecret = "trustee-bootstrap-user-keys"
)

// cloneTrusteeRepo clones the trustee repo at the pinned SHA into a temp
// directory and returns its path. The caller is responsible for removing it.
func cloneTrusteeRepo() (string, error) {
	versions, err := utils.GetVersions()
	if err != nil {
		return "", fmt.Errorf("reading versions.yaml: %w", err)
	}
	kbs, ok := versions.Git["kbs"]
	if !ok {
		return "", fmt.Errorf("git.kbs not found in versions.yaml")
	}

	dir, err := os.MkdirTemp("", "trustee-")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	for _, args := range [][]string{
		{"git", "clone", "--depth", "1", kbs.URL, dir},
		{"git", "-C", dir, "fetch", "--depth=1", "origin", kbs.Ref},
		{"git", "-C", dir, "checkout", "FETCH_HEAD"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("running %v: %w\n%s", args, err, out)
		}
	}
	return dir, nil
}

// buildTrusteeValues writes a temporary Helm values override file and returns
// its path. The caller is responsible for removing it.
func buildTrusteeValues(ibmseCredsDir, workerNodeName string) (string, error) {
	versions, err := utils.GetVersions()
	if err != nil {
		return "", fmt.Errorf("reading versions.yaml: %w", err)
	}
	kbs := versions.Git["kbs"]
	tag := kbs.Ref
	kbsRepo := kbs.Images["kbs"]
	asRepo := kbs.Images["as"]
	rvpsRepo := kbs.Images["rvps"]

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `log_level: debug
kbs:
  image:
    repository: %q
    tag: %q
  service:
    exposeLoadBalancer: false
  resources:
    requests:
      cpu: 50m
      memory: 128Mi
    limits:
      cpu: "1"
      memory: 1Gi
as:
  image:
    repository: %q
    tag: %q
  resources:
    requests:
      cpu: 50m
      memory: 256Mi
    limits:
      cpu: "2"
      memory: 2Gi
rvps:
  image:
    repository: %q
    tag: %q
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: "1"
      memory: 512Mi
nodePort:
  enabled: true
`, kbsRepo, tag, asRepo, tag, rvpsRepo, tag)

	if ibmseCredsDir != "" {
		fmt.Fprintf(&buf, `as:
  verifier:
    se:
      credsDir: %q
      nodeName: %q
  podSecurityContext:
    fsGroup: 1000
`, ibmseCredsDir, workerNodeName)
	}

	f, err := os.CreateTemp("", "trustee-values-*.yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(buf.Bytes()); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// helmInstallTrustee builds chart dependencies and runs helm upgrade --install.
func helmInstallTrustee(chartDir, valuesFile string, cfg *envconf.Config) error {
	depCmd := exec.Command("helm", "dependency", "build", chartDir)
	depCmd.Env = os.Environ()
	if out, err := depCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helm dependency build: %w\n%s", err, out)
	}

	args := []string{
		"upgrade", "--install", kbsReleaseName, chartDir,
		"--namespace", kbsNamespace,
		"--create-namespace",
		"--kubeconfig", cfg.KubeconfigFile(),
		"-f", valuesFile,
		"--wait", "--timeout", "5m",
	}
	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	log.Info("helm install output:\n", string(out))
	if err != nil {
		return fmt.Errorf("helm upgrade --install: %w\n%s", err, out)
	}
	return nil
}

// helmUninstallTrustee runs helm uninstall for the Trustee release.
func helmUninstallTrustee(cfg *envconf.Config) error {
	args := []string{
		"uninstall", kbsReleaseName,
		"--namespace", kbsNamespace,
		"--kubeconfig", cfg.KubeconfigFile(),
		"--wait",
	}
	cmd := exec.Command("helm", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	log.Info("helm uninstall output:\n", string(out))
	if err != nil {
		return fmt.Errorf("helm uninstall: %w\n%s", err, out)
	}
	return nil
}

// extractAdminToken reads KBS_ADMIN_TOKEN from the Helm bootstrap Secret.
func extractAdminToken(cfg *envconf.Config) (string, error) {
	args := []string{
		"get", "secret", kbsBootstrapSecret,
		"-n", kbsNamespace,
		"--kubeconfig", cfg.KubeconfigFile(),
		"-o", "jsonpath={.data.KBS_ADMIN_TOKEN}",
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl get secret %s: %w", kbsBootstrapSecret, err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("base64 decode admin token: %w", err)
	}
	return string(decoded), nil
}

func NewKeyBrokerService(clusterName string, cfg *envconf.Config) (*KeyBrokerService, error) {
	log.Infof("NewKeyBrokerService: cluster=%s", clusterName)

	ibmseCredsDir := os.Getenv("IBM_SE_CREDS_DIR")
	var workerNodeName string

	if ibmseCredsDir != "" {
		log.Info("IBM_SE_CREDS_DIR set, will configure IBM SE verifier")
		workerNodeIP, name, err := getFirstWorkerNodeIPAndName(cfg)
		if err != nil {
			return nil, err
		}
		workerNodeName = name
		log.Infof("Copying IBM SE creds to worker node %s (%s)", name, workerNodeIP)
		if err := copyGivenFilesToWorkerNode(ibmseCredsDir, workerNodeIP); err != nil {
			return nil, err
		}
	}

	log.Info("Cloning trustee repo")
	repoDir, err := cloneTrusteeRepo()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(repoDir)

	valuesFile, err := buildTrusteeValues(ibmseCredsDir, workerNodeName)
	if err != nil {
		return nil, err
	}
	defer os.Remove(valuesFile)

	chartDir := filepath.Join(repoDir, "deployment/helm-chart")
	log.Info("Installing Trustee via Helm")
	if err := helmInstallTrustee(chartDir, valuesFile, cfg); err != nil {
		return nil, err
	}

	log.Info("Extracting admin token from bootstrap Secret")
	adminToken, err := extractAdminToken(cfg)
	if err != nil {
		return nil, err
	}

	return &KeyBrokerService{adminToken: adminToken}, nil
}

func getHardwarePlatform() (string, error) {
	out, err := exec.Command("uname", "-m").Output()
	return strings.TrimSuffix(string(out), "\n"), err
}

func getFirstWorkerNodeIPAndName(cfg *envconf.Config) (string, string, error) {
	client, err := cfg.NewClient()
	if err != nil {
		return "", "", err
	}
	nodeList := &corev1.NodeList{}
	if err := client.Resources("").List(context.TODO(), nodeList); err != nil {
		return "", "", err
	}
	for _, node := range nodeList.Items {
		if isWorkerNode(&node) {
			return node.Status.Addresses[0].Address, node.Name, nil
		}
	}
	return "", "", fmt.Errorf("no worker nodes found")
}

func isWorkerNode(node *corev1.Node) bool {
	_, isMaster := node.Labels["node-role.kubernetes.io/master"]
	_, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]
	return !isMaster && !isControlPlane
}

func copyGivenFilesToWorkerNode(sourceDir, targetNodeIP string) error {
	tarFilePath, err := compressDirectory(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to compress directory: %v", err)
	}
	defer os.Remove(tarFilePath)

	targetFilePath := "/tmp/" + filepath.Base(tarFilePath)
	if err = transferFile(tarFilePath, targetNodeIP, targetFilePath); err != nil {
		return fmt.Errorf("failed to transfer file: %v", err)
	}

	if err = decompressFileOnTargetNode(targetNodeIP, targetFilePath, "/root"); err != nil {
		return fmt.Errorf("failed to decompress file on target node: %v", err)
	}
	return nil
}

func compressDirectory(sourceDir string) (string, error) {
	tarFilePath := sourceDir + ".tar.gz"
	cmd := exec.Command("tar", "-czf", tarFilePath, "-C", filepath.Dir(sourceDir), filepath.Base(sourceDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return tarFilePath, cmd.Run()
}

func transferFile(localFilePath, targetNodeIP, remoteFilePath string) error {
	cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		localFilePath, fmt.Sprintf("root@%s:%s", targetNodeIP, remoteFilePath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func decompressFileOnTargetNode(targetNodeIP, remoteFilePath, targetDir string) error {
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("root@%s", targetNodeIP), fmt.Sprintf("tar -xzf %s -C %s", remoteFilePath, targetDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getNodeIPForSvc(deploymentName string, service corev1.Service, cfg *envconf.Config) (string, error) {
	client, err := cfg.NewClient()
	if err != nil {
		return "", err
	}
	podList := &corev1.PodList{}
	if err := client.Resources(service.Namespace).List(context.TODO(), podList); err != nil {
		return "", err
	}

	nodeList := &corev1.NodeList{}
	if err := client.Resources("").List(context.TODO(), nodeList); err != nil {
		return "", err
	}

	var matchingPod *corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Labels["app"] == deploymentName {
			matchingPod = pod
			break
		}
	}
	if matchingPod == nil {
		return "", fmt.Errorf("no pod with app=%s found", deploymentName)
	}

	for _, node := range nodeList.Items {
		if node.Name == matchingPod.Spec.NodeName {
			return node.Status.Addresses[0].Address, nil
		}
	}
	return "", fmt.Errorf("node IP not found for service %s", service.Name)
}

func (p *KeyBrokerService) GetCachedKbsEndpoint() (string, error) {
	if p.endpoint != "" {
		return p.endpoint, nil
	}
	return "", fmt.Errorf("KeyBrokerService endpoint not set")
}

func (p *KeyBrokerService) GetKbsEndpoint(ctx context.Context, cfg *envconf.Config) (string, error) {
	client, err := cfg.NewClient()
	if err != nil {
		return "", err
	}

	resources := client.Resources(kbsNamespace)
	kbsDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: kbsDeploymentName, Namespace: kbsNamespace},
	}
	fmt.Printf("Wait for the %s deployment to be available\n", kbsDeploymentName)
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(
		kbsDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(time.Minute*5)); err != nil {
		return "", err
	}

	services := &corev1.ServiceList{}
	if err := resources.List(ctx, services); err != nil {
		return "", err
	}

	for _, svc := range services.Items {
		if svc.Name == kbsNodePortSvcName {
			if svc.Spec.Type != corev1.ServiceTypeNodePort {
				return "", fmt.Errorf("service %s is not of type NodePort", kbsNodePortSvcName)
			}
			if len(svc.Spec.Ports) == 0 {
				return "", fmt.Errorf("service %s has no ports", kbsNodePortSvcName)
			}
			nodePort := svc.Spec.Ports[0].NodePort
			nodeIP, err := getNodeIPForSvc(kbsDeploymentName, svc, cfg)
			if err != nil {
				return "", err
			}
			p.endpoint = fmt.Sprintf("http://%s:%d", nodeIP, nodePort)
			return p.endpoint, nil
		}
	}
	return "", fmt.Errorf("service %s not found in namespace %s", kbsNodePortSvcName, kbsNamespace)
}

// runKbsClientAdmin writes the stored admin token to a temp file and runs
// kbs-client with the supplied config subcommand arguments.
func (p *KeyBrokerService) runKbsClientAdmin(args ...string) error {
	f, err := os.CreateTemp("", "kbs-admin-token-*")
	if err != nil {
		return fmt.Errorf("creating admin token file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(p.adminToken); err != nil {
		f.Close()
		return fmt.Errorf("writing admin token: %w", err)
	}
	f.Close()

	cmdArgs := append([]string{"--url", p.endpoint, "config", "--admin-token-file", f.Name()}, args...)
	cmd := exec.Command("kbs-client", cmdArgs...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, out)
	return err
}

func (p *KeyBrokerService) EnableKbsCustomizedResourcePolicy(customizedOpaFile string) error {
	log.Info("EnableKbsCustomizedResourcePolicy: ", customizedOpaFile)
	return p.runKbsClientAdmin("set-resource-policy", "--policy-file", customizedOpaFile)
}

func (p *KeyBrokerService) EnableKbsCustomizedAttestationPolicy(customizedOpaFile string) error {
	log.Info("EnableKbsCustomizedAttestationPolicy: ", customizedOpaFile)
	return p.runKbsClientAdmin("set-attestation-policy", "--policy-file", customizedOpaFile)
}

func (p *KeyBrokerService) setSecretKey(resource string, path string) error {
	log.Info("set key resource: ", resource)
	return p.runKbsClientAdmin("set-resource", "--path", resource, "--resource-file", path)
}

func (p *KeyBrokerService) SetSecret(resourcePath string, secret []byte) error {
	tempDir, err := os.MkdirTemp("", "kbs_resource_files")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	secretFilePath := filepath.Join(tempDir, filepath.Base(resourcePath))
	if err := os.WriteFile(secretFilePath, secret, 0o644); err != nil {
		return err
	}
	return p.setSecretKey(resourcePath, secretFilePath)
}

func (p *KeyBrokerService) SetImageDecryptionKey(keyID string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("image decryption key must be 32 bytes")
	}
	f, err := os.CreateTemp("", "image-decryption-*.key")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(key); err != nil {
		return err
	}
	f.Close()
	return p.setSecretKey(keyID, f.Name())
}

func (p *KeyBrokerService) Deploy(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	// Helm install was performed by NewKeyBrokerService; nothing more to do here.
	return nil
}

func (p *KeyBrokerService) Delete(ctx context.Context, cfg *envconf.Config) error {
	log.Info("Uninstalling Trustee via Helm")
	if err := helmUninstallTrustee(cfg); err != nil {
		log.Warnf("helm uninstall failed (may already be gone): %v", err)
	}

	log.Info("Deleting namespace ", kbsNamespace)
	cmd := exec.Command("kubectl", "delete", "ns", kbsNamespace,
		"--ignore-not-found",
		"--kubeconfig", cfg.KubeconfigFile())
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, out)
	return err
}
