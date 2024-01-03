// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/BurntSushi/toml"
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
	UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error
}

type NewProvisionerFunc func(properties map[string]string) (CloudProvisioner, error)

var NewProvisionerFunctions = make(map[string]NewProvisionerFunc)

type CloudAPIAdaptor struct {
	caaDaemonSet         *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	ccDaemonSet          *appsv1.DaemonSet    // Represents the CoCo installer daemonset
	cloudProvider        string               // Cloud provider
	controllerDeployment *appsv1.Deployment   // Represents the controller manager deployment
	namespace            string               // The CoCo namespace
	installOverlay       InstallOverlay       // Pointer to the kustomize overlay
	runtimeClass         *nodev1.RuntimeClass // The Kata Containers runtimeclass
}

type NewInstallOverlayFunc func(installDir, provider string) (InstallOverlay, error)

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

func NewCloudAPIAdaptor(provider string, installDir string) (*CloudAPIAdaptor, error) {
	namespace := "confidential-containers-system"

	overlay, err := GetInstallOverlay(provider, installDir)
	if err != nil {
		return nil, err
	}

	return &CloudAPIAdaptor{
		caaDaemonSet:         &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cloud-api-adaptor-daemonset", Namespace: namespace}},
		ccDaemonSet:          &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-daemon-install", Namespace: namespace}},
		cloudProvider:        provider,
		controllerDeployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-controller-manager", Namespace: namespace}},
		namespace:            namespace,
		installOverlay:       overlay,
		runtimeClass:         &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata-remote", Namespace: ""}},
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

// Deletes the peer pods installation including the controller manager.
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
	cmd := exec.Command("kubectl", "delete", "-k", "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods?ref=v0.8.0")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
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
	cmd = exec.Command("kubectl", "delete", "-k", "github.com/confidential-containers/operator/config/default?ref=v0.8.0")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
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

	return nil
}

// Deploy installs Peer Pods on the cluster.
func (p *CloudAPIAdaptor) Deploy(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	resources := client.Resources(p.namespace)

	log.Info("Install the controller manager")
	// TODO - find go idiomatic way to apply/delete remote kustomize and apply to this file
	cmd := exec.Command("kubectl", "apply", "-k", "github.com/confidential-containers/operator/config/default?ref=v0.8.0")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
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

	cmd = exec.Command("kubectl", "apply", "-k", "github.com/confidential-containers/operator/config/samples/ccruntime/peer-pods?ref=v0.8.0")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
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

	fmt.Printf("Wait for the %s runtimeclass be created\n", p.runtimeClass.GetName())
	if err = wait.For(conditions.New(resources).ResourcesFound(&nodev1.RuntimeClassList{Items: []nodev1.RuntimeClass{*p.runtimeClass}}),
		wait.WithTimeout(time.Second*60)); err != nil {
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
		if err := wait.For(conditions.New(resources).PodReady(obj), wait.WithTimeout(time.Second*6)); err != nil {
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
