// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// CloudProvisioner defines operations to provision the environment on cloud providers.
type CloudProvisioner interface {
	CreateCluster(ctx context.Context, cfg *envconf.Config) error
	CreateVPC(ctx context.Context, cfg *envconf.Config) error
	DeleteCluster(ctx context.Context, cfg *envconf.Config) error
	DeleteVPC(ctx context.Context, cfg *envconf.Config) error
	GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string
	UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error
}

// PodVMInstanceHandler defines optional VM instance creation capability
type PodVMInstanceHandler interface {
	CreatePodVMInstance(ctx context.Context, cfg *envconf.Config) error
	DeletePodVMInstance(ctx context.Context, cfg *envconf.Config) error
}

type NewProvisionerFunc func(properties map[string]string) (CloudProvisioner, error)

// KbsInstallOverlay implements the InstallOverlay interface
type KbsInstallOverlay struct {
	overlay *KustomizeOverlay
}

var NewProvisionerFunctions = make(map[string]NewProvisionerFunc)

type CloudAPIAdaptor struct {
	caaDaemonSet  *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	cloudProvider string               // Cloud provider
	namespace     string               // The CoCo namespace
	installDir    string               // The install directory path
	runtimeClass  *nodev1.RuntimeClass // The Kata Containers runtimeclass
	rootSrcDir    string               // The root src directory of cloud-api-adaptor
}

type KeyBrokerService struct {
	installOverlay InstallOverlay // Pointer to the kustomize overlay
	endpoint       string         // KBS Service endpoint, such as: http://NodeIP:Port
}

// InstallOverlay defines common operations to an install overlay (install/overlays/*)
type InstallOverlay interface {
	// Apply applies the overlay. Equivalent to the `kubectl apply -k` command
	Apply(ctx context.Context, cfg *envconf.Config) error
	// Delete deletes the overlay. Equivalent to the `kubectl delete -k` command
	Delete(ctx context.Context, cfg *envconf.Config) error
	// Edit changes overlay files
	Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error
}

// InstallChart defines common operations to an install chart (install/charts/*)
type InstallChart interface {
	// Install installs the chart. Equivalent to the `helm install` command
	Install(ctx context.Context, cfg *envconf.Config) error
	// Uninstall uninstalls the chart. Equivalent to the `helm uninstall` command
	Uninstall(ctx context.Context, cfg *envconf.Config) error
	// Configure changes chart values
	Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error
}

type NewInstallChartFunc func(installDir, provider string) (InstallChart, error)

var NewInstallChartFunctions = make(map[string]NewInstallChartFunc)

// Waiting timeout for bringing up the pod
const PodWaitTimeout = time.Second * 30

func NewCloudAPIAdaptor(provider string, installDir string) (*CloudAPIAdaptor, error) {
	namespace := GetCAANamespace()

	return &CloudAPIAdaptor{
		caaDaemonSet:  &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cloud-api-adaptor-daemonset", Namespace: namespace}},
		cloudProvider: provider,
		namespace:     namespace,
		installDir:    installDir,
		runtimeClass:  &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata-remote", Namespace: ""}},
		rootSrcDir:    filepath.Dir(installDir),
	}, nil
}

// GetCloudProvisioner returns a CloudProvisioner implementation
func GetCloudProvisioner(provider string, propertiesFile string) (CloudProvisioner, error) {
	properties := make(map[string]string)
	if propertiesFile != "" {
		f, err := os.ReadFile(propertiesFile)
		if err != nil {
			return nil, err
		}
		if err = toml.Unmarshal(f, &properties); err != nil {
			return nil, err
		}
	}

	newProvisioner, ok := NewProvisionerFunctions[provider]
	if !ok {
		log.Info("Supported providers are:")
		for provisioner := range NewProvisionerFunctions {
			log.Info(provisioner)
		}
		return nil, fmt.Errorf("Not implemented provisioner for %s\n", provider)
	}

	return newProvisioner(properties)
}

// GetInstallChart returns the InstallChart implementation for the provider
func GetInstallChart(provider string, installDir string) (InstallChart, error) {
	chartFunc, ok := NewInstallChartFunctions[provider]
	if !ok {
		return nil, fmt.Errorf("Not implemented install chart for %s\n", provider)
	}

	return chartFunc(installDir, provider)
}

// Deletes the peer pods installation including the controller manager.
func (p *CloudAPIAdaptor) Delete(ctx context.Context, cfg *envconf.Config) error {
	log.Info("Uninstall the cloud-api-adaptor using helm")
	chart, err := GetInstallChart(p.cloudProvider, p.installDir)
	if err != nil {
		return err
	}
	if err = chart.Uninstall(ctx, cfg); err != nil {
		return err
	}
	return nil

}

// Deploy installs Peer Pods on the cluster.
func (p *CloudAPIAdaptor) Deploy(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	// Install cert-manager (required for webhook)
	if err := p.installCertManager(ctx, cfg); err != nil {
		return err
	}
	log.Info("Install the cloud-api-adaptor using helm")
	chart, err := GetInstallChart(p.cloudProvider, p.installDir)
	if err != nil {
		return err
	}
	if err := chart.Configure(ctx, cfg, props); err != nil {
		return err
	}
	if err := chart.Install(ctx, cfg); err != nil {
		return err
	}

	// Wait for webhook and peerpod-ctrl deployments to be available.
	// Use label-based lookup to find deployments regardless of namespace or namePrefix overrides.
	if err := findAndWaitForDeployment(ctx, cfg, "peerpods-webhook", time.Minute*5); err != nil {
		return fmt.Errorf("webhook deployment wait failed: %w", err)
	}

	if err := findAndWaitForDeployment(ctx, cfg, "peerpodctrl", time.Minute*5); err != nil {
		return fmt.Errorf("peerpod-ctrl deployment wait failed: %w", err)
	}

	return nil

}

// TODO: convert this into a klient/wait/conditions
func AllPodsRunning(ctx context.Context, cfg *envconf.Config, namespace string) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	resources := client.Resources(namespace)
	objList := &corev1.PodList{}
	err = resources.List(context.TODO(), objList)
	if err != nil {
		return err
	}
	metaList, _ := meta.ExtractList(objList)
	for _, o := range metaList {
		obj, _ := o.(k8s.Object)
		fmt.Printf("Wait pod '%s' status for Ready\n", obj.GetName())
		if err := wait.For(conditions.New(resources).PodReady(obj), wait.WithTimeout(PodWaitTimeout)); err != nil {
			return err
		}
		fmt.Printf("pod '%s' is Ready\n", obj.GetName())
	}
	return nil
}

// Returns a list of running Pods owned by a DaemonSet
func GetDaemonSetOwnedPods(ctx context.Context, cfg *envconf.Config, daemonset *appsv1.DaemonSet) (*corev1.PodList, error) {
	client, err := cfg.NewClient()
	if err != nil {
		return nil, err
	}

	resources := client.Resources(daemonset.GetNamespace())
	pods, retPods := &corev1.PodList{}, &corev1.PodList{}

	_ = resources.List(context.TODO(), pods)
	for _, pod := range pods.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "DaemonSet" && owner.Name == daemonset.Name {
				retPods.Items = append(retPods.Items, pod)
			}
		}
	}

	return retPods, nil
}

func GetCAANamespace() string {
	namespace := os.Getenv("TEST_CAA_NAMESPACE")
	if namespace == "" {
		namespace = "confidential-containers-system"
	}
	return namespace
}

// installCertManager installs cert-manager which is required for the webhook
func (p *CloudAPIAdaptor) installCertManager(ctx context.Context, cfg *envconf.Config) error {
	log.Info("Installing cert-manager")
	cmd := exec.Command("make", "-C", "../webhook", "deploy-cert-manager")
	// Run the deployment from the root src dir
	cmd.Dir = p.rootSrcDir
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		log.Infof("Error in install cert-manager: %s: %s", err, stdoutStderr)
		return err
	}
	return nil
}

// findAndWaitForDeployment finds a deployment by labels and waits for it to be available.
// This is used for helm installations where namespace and namePrefix can be overridden.
func findAndWaitForDeployment(ctx context.Context, cfg *envconf.Config, partOfLabel string, timeout time.Duration) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	// List all deployments and filter by labels
	deploymentList := &appsv1.DeploymentList{}
	if err = client.Resources().List(ctx, deploymentList); err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	// Find deployment matching labels
	var deployment *appsv1.Deployment
	for i := range deploymentList.Items {
		labels := deploymentList.Items[i].GetLabels()
		if labels["app.kubernetes.io/part-of"] == partOfLabel && labels["control-plane"] == "controller-manager" {
			deployment = &deploymentList.Items[i]
			break
		}
	}

	if deployment == nil {
		return fmt.Errorf("deployment not found with label app.kubernetes.io/part-of=%s", partOfLabel)
	}

	resources := client.Resources(deployment.Namespace)

	fmt.Printf("Wait for deployment %s in namespace %s to be available\n", deployment.Name, deployment.Namespace)
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(timeout)); err != nil {
		return err
	}

	return nil
}

func CreateAndWaitForNamespace(ctx context.Context, client klient.Client, namespaceName string) error {
	log.Infof("Creating namespace '%s'...", namespaceName)
	nsObj := corev1.Namespace{}
	nsObj.Name = namespaceName
	if err := client.Resources().Create(ctx, &nsObj); err != nil {
		return err
	}

	if err := waitForNamespaceToBeUseable(ctx, client, namespaceName); err != nil {
		return err
	}
	return nil
}

const WaitNamespaceAvailableTimeout = time.Second * 120

func waitForNamespaceToBeUseable(ctx context.Context, client klient.Client, namespaceName string) error {
	log.Infof("Wait for namespace '%s' be ready...", namespaceName)
	nsObj := corev1.Namespace{}
	nsObj.Name = namespaceName
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&nsObj, func(object k8s.Object) bool {
		ns, ok := object.(*corev1.Namespace)
		if !ok {
			log.Printf("Not a namespace object: %v", object)
			return false
		}
		return ns.Status.Phase == corev1.NamespaceActive
	}), wait.WithTimeout(WaitNamespaceAvailableTimeout)); err != nil {
		return err
	}

	// SH: There is a race condition where the default service account isn't ready when we
	// try and use it #1657, so we want to ensure that it is available before continuing.
	// As the serviceAccount doesn't have a status I can't seem to use the wait condition to
	// detect if it is ready, so do things the old-fashioned way
	log.Infof("Wait for default serviceaccount in namespace '%s'...", namespaceName)
	var saList corev1.ServiceAccountList
	for start := time.Now(); time.Since(start) < WaitNamespaceAvailableTimeout; {
		if err := client.Resources(namespaceName).List(ctx, &saList); err != nil {
			return err
		}
		for _, sa := range saList.Items {
			if sa.Name == "default" {

				log.Infof("default serviceAccount exists, namespace '%s' is ready for use", namespaceName)
				return nil
			}
		}
		log.Tracef("default serviceAccount not found after %.0f seconds", time.Since(start).Seconds())
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("default service account not found in namespace '%s' after %.0f seconds wait", namespaceName, WaitNamespaceAvailableTimeout.Seconds())
}
