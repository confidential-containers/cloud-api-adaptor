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
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
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
	GetProvisionValues() map[string]interface{}
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
	caaDaemonSet         *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	ccDaemonSet          *appsv1.DaemonSet    // Represents the CoCo installer daemonset
	ccOpGitRepo          string               // CoCo operator's repository URL
	ccOpConfig           string               // CoCo operator's config to use: default or release
	ccOpGitRef           string               // CoCo operator's repository reference
	cloudProvider        string               // Cloud provider
	controllerDeployment *appsv1.Deployment   // Represents the controller manager deployment
	namespace            string               // The CoCo namespace
	installOverlay       InstallOverlay       // Pointer to the kustomize overlay
	installDir           string               // The install directory path
	runtimeClass         *nodev1.RuntimeClass // The Kata Containers runtimeclass
	rootSrcDir           string               // The root src directory of cloud-api-adaptor
}

type NewInstallOverlayFunc func(installDir, provider string) (InstallOverlay, error)

type KeyBrokerService struct {
	installOverlay InstallOverlay // Pointer to the kustomize overlay
	endpoint       string         // KBS Service endpoint, such as: http://NodeIP:Port
}

var NewInstallOverlayFunctions = make(map[string]NewInstallOverlayFunc)

// InstallOverlay defines common operations to an install overlay (install/overlays/*)
type InstallOverlay interface {
	// Apply applies the overlay. Equivalent to the `kubectl apply -k` command
	Apply(ctx context.Context, cfg *envconf.Config) error
	// Delete deletes the overlay. Equivalent to the `kubectl delete -k` command
	Delete(ctx context.Context, cfg *envconf.Config) error
	// Edit changes overlay files
	Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error
}

// Waiting timeout for bringing up the pod
const PodWaitTimeout = time.Second * 30

func NewCloudAPIAdaptor(provider string, installDir string) (*CloudAPIAdaptor, error) {
	namespace := GetCAANamespace()

	overlay, err := GetInstallOverlay(provider, installDir)
	if err != nil {
		return nil, err
	}

	versions, err := utils.GetVersions()
	if err != nil {
		return nil, err
	}
	ccOperator := versions.Git["coco-operator"]

	return &CloudAPIAdaptor{
		caaDaemonSet:         &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cloud-api-adaptor-daemonset", Namespace: namespace}},
		ccDaemonSet:          &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-daemon-install", Namespace: namespace}},
		ccOpGitRepo:          ccOperator.Url,
		ccOpConfig:           ccOperator.Config,
		ccOpGitRef:           ccOperator.Ref,
		cloudProvider:        provider,
		controllerDeployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-controller-manager", Namespace: namespace}},
		namespace:            namespace,
		installOverlay:       overlay,
		installDir:           installDir,
		runtimeClass:         &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata-remote", Namespace: ""}},
		rootSrcDir:           filepath.Dir(installDir),
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

// GetInstallOverlay returns the InstallOverlay implementation for the provider
func GetInstallOverlay(provider string, installDir string) (InstallOverlay, error) {
	overlayFunc, ok := NewInstallOverlayFunctions[provider]
	if !ok {
		return nil, fmt.Errorf("Not implemented install overlay for %s\n", provider)
	}

	return overlayFunc(installDir, provider)
}

// Delete uninstalls the peer pods installation using kustomize.
// For helm-based uninstallation, use DeleteWithHelm() instead.
func (p *CloudAPIAdaptor) Delete(ctx context.Context, cfg *envconf.Config) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	resources := client.Resources(p.namespace)

	ccPods, err := GetDaemonSetOwnedPods(ctx, cfg, p.ccDaemonSet)
	if err != nil {
		return err
	}
	caaPods, err := GetDaemonSetOwnedPods(ctx, cfg, p.caaDaemonSet)
	if err != nil {
		return err
	}

	log.Info("Uninstall the cloud-api-adaptor")
	if err = p.installOverlay.Delete(ctx, cfg); err != nil {
		return err
	}

	log.Info("Uninstall CCRuntime CRD")
	cmd := exec.Command("kubectl", "delete", "-k", p.ccOpGitRepo+"/config/samples/ccruntime/peer-pods?ref="+p.ccOpGitRef)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	for _, pods := range []*corev1.PodList{ccPods, caaPods} {
		if err != nil {
			return err
		}
		if err = wait.For(conditions.New(resources).ResourcesDeleted(pods), wait.WithTimeout(time.Minute*5)); err != nil {
			return err
		}
	}

	deployments := &appsv1.DeploymentList{Items: []appsv1.Deployment{*p.controllerDeployment}}

	log.Info("Uninstall the controller manager")
	cmd = exec.Command("kubectl", "delete", "-k", p.ccOpGitRepo+"/config/"+p.ccOpConfig+"?ref="+p.ccOpGitRef)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	log.Infof("Wait for the %s deployment be deleted\n", p.controllerDeployment.GetName())
	if err = wait.For(conditions.New(resources).ResourcesDeleted(deployments),
		wait.WithTimeout(time.Minute*1)); err != nil {
		return err
	}

	log.Info("Delete the peerpod-ctrl deployment")
	cmd = exec.Command("make", "ignore-not-found=true", "-C", "../peerpod-ctrl", "undeploy")
	// Run the command from the root src dir
	cmd.Dir = p.rootSrcDir
	// Set the KUBECONFIG env var
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	log.Info("Wait for the peerpod-ctrl deployment to be deleted")
	if err = wait.For(conditions.New(resources).ResourcesDeleted(
		&appsv1.DeploymentList{Items: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{Name: "peerpod-ctrl-controller-manager", Namespace: p.namespace}},
		}}),
		wait.WithTimeout(time.Minute*1)); err != nil {
		return err
	}

	return nil
}

// DeployWithHelm installs Peer Pods using Helm with provided values files.
func (p *CloudAPIAdaptor) DeployWithHelm(ctx context.Context, cfg *envconf.Config, valuesFiles []string) error {
	dryRun := os.Getenv("HELM_DRY_RUN") == "true"

	// Install cert-manager (required for webhook) - skip in dry-run mode
	if !dryRun {
		if err := p.installCertManager(ctx, cfg); err != nil {
			return err
		}
	}

	log.Info("Install the cloud-api-adaptor using helm")
	chartPath := filepath.Join(p.installDir, "charts", "peerpods")
	namespace := GetCAANamespace()

	helm, err := NewHelm(chartPath, namespace, "peerpods", false)
	if err != nil {
		return err
	}

	// Load all values files (provider base + static from workflow + dynamic from provisioner)
	for _, vf := range valuesFiles {
		if err := helm.LoadFromFile(vf); err != nil {
			return err
		}
	}

	if err := helm.Install(ctx, cfg); err != nil {
		if err == ErrDryRun {
			log.Info("Dry-run completed, skipping deployment waits")
			return ErrDryRun
		}
		return err
	}

	// Wait for webhook and peerpod-ctrl deployments to be available.
	if err := findAndWaitForDeployment(ctx, cfg, "peerpods-webhook", time.Minute*5); err != nil {
		return fmt.Errorf("webhook deployment wait failed: %w", err)
	}

	if err := findAndWaitForDeployment(ctx, cfg, "peerpodctrl", time.Minute*5); err != nil {
		return fmt.Errorf("peerpod-ctrl deployment wait failed: %w", err)
	}

	return nil
}

// DeleteWithHelm uninstalls Peer Pods using Helm.
func (p *CloudAPIAdaptor) DeleteWithHelm(ctx context.Context, cfg *envconf.Config) error {
	log.Info("Uninstall the cloud-api-adaptor using helm")
	chartPath := filepath.Join(p.installDir, "charts", "peerpods")
	namespace := GetCAANamespace()

	helm, err := NewHelm(chartPath, namespace, "peerpods", false)
	if err != nil {
		return err
	}

	return helm.Uninstall(ctx, cfg)
}

// Deploy installs Peer Pods on the cluster using kustomize.
// For helm-based installation, use DeployWithHelm() instead.
func (p *CloudAPIAdaptor) Deploy(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	resources := client.Resources(p.namespace)

	log.Info("Install the controller manager")
	// TODO - find go idiomatic way to apply/delete remote kustomize and apply to this file
	cmd := exec.Command("kubectl", "apply", "-k", p.ccOpGitRepo+"/config/"+p.ccOpConfig+"?ref="+p.ccOpGitRef)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err := cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	fmt.Printf("Wait for the %s deployment be available\n", p.controllerDeployment.GetName())
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(p.controllerDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(time.Minute*10)); err != nil {
		return err
	}

	log.Info("Customize the overlay yaml file")
	if err := p.installOverlay.Edit(ctx, cfg, props); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "apply", "-k", p.ccOpGitRepo+"/config/samples/ccruntime/peer-pods?ref="+p.ccOpGitRef)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	log.Info("Install the cloud-api-adaptor")
	if err := p.installOverlay.Apply(ctx, cfg); err != nil {
		return err
	}

	// Wait for the CoCo installer and CAA pods be ready
	daemonSetList := map[*appsv1.DaemonSet]time.Duration{
		p.ccDaemonSet:  time.Minute * 15,
		p.caaDaemonSet: time.Minute * 10,
	}

	for ds, timeout := range daemonSetList {
		// Wait for the daemonset to have at least one pod running then wait for each pod
		// be ready.

		fmt.Printf("Wait for the %s DaemonSet be available\n", ds.GetName())
		if err = wait.For(conditions.New(resources).ResourceMatch(ds, func(object k8s.Object) bool {
			ds = object.(*appsv1.DaemonSet)
			return ds.Status.CurrentNumberScheduled > 0
		}), wait.WithTimeout(time.Minute*5)); err != nil {
			return err
		}
		pods, err := GetDaemonSetOwnedPods(ctx, cfg, ds)
		if err != nil {
			return err
		}
		for _, pod := range pods.Items {
			fmt.Printf("Wait for the pod %s be ready\n", pod.GetName())
			if err = wait.For(conditions.New(resources).PodReady(&pod), wait.WithTimeout(timeout)); err != nil {
				return err
			}
		}
	}

	cmd = exec.Command("kubectl", "get", "cm", "peer-pods-cm", "-n", GetCAANamespace(), "-o", "yaml")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	log.Infof("Wait for the %s runtimeclass be created\n", p.runtimeClass.GetName())
	if err = wait.For(conditions.New(resources).ResourcesFound(&nodev1.RuntimeClassList{Items: []nodev1.RuntimeClass{*p.runtimeClass}}),
		wait.WithTimeout(time.Second*60)); err != nil {
		return err
	}

	log.Info("Installing peerpod-ctrl")
	cmd = exec.Command("make", "-C", "../peerpod-ctrl", "deploy")
	// Run the deployment from the root src dir
	cmd.Dir = p.rootSrcDir
	// Set the KUBECONFIG env var
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	// Wait for the peerpod-ctrl deployment to be ready
	log.Info("Wait for the peerpod-ctrl deployment to be available")
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "peerpod-ctrl-controller-manager", Namespace: p.namespace}},
		appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(time.Minute*5)); err != nil {
		return err
	}

	if err := p.installCertManager(ctx, cfg); err != nil {
		return err
	}

	log.Info("Installing webhook")
	cmd = exec.Command("make", "-C", "../webhook", "deploy")
	// Run the deployment from the root src dir
	cmd.Dir = p.rootSrcDir
	// Set the KUBECONFIG env var
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.KubeconfigFile())
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	// Wait for the webhook deployment to be ready
	log.Info("Wait for the webhook deployment to be available")
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "peer-pods-webhook-controller-manager", Namespace: "peer-pods-webhook-system"}},
		appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(time.Minute*10)); err != nil {
		return err
	}

	return nil
}

func (p *CloudAPIAdaptor) DoKustomize(ctx context.Context, cfg *envconf.Config) {
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
