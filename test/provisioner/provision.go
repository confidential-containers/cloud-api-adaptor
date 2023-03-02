// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"github.com/BurntSushi/toml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"time"
)

// CloudProvision defines operations to provision the environment on cloud providers.
type CloudProvision interface {
	CreateCluster(ctx context.Context, cfg *envconf.Config) error
	CreateVPC(ctx context.Context, cfg *envconf.Config) error
	DeleteCluster(ctx context.Context, cfg *envconf.Config) error
	DeleteVPC(ctx context.Context, cfg *envconf.Config) error
	UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error
}

type CloudAPIAdaptor struct {
	caaDaemonSet         *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	ccDaemonSet          *appsv1.DaemonSet    // Represents the CoCo installer daemonset
	cloudProvider        string               // Cloud provider
	controllerDeployment *appsv1.Deployment   // Represents the controller manager deployment
	namespace            string               // The CoCo namespace
	kustomizeHelper      *KustomizeHelper     // Pointer to the kustomize helper
	runtimeClass         *nodev1.RuntimeClass // The Kata Containers runtimeclass
}

func NewCloudAPIAdaptor(provider string) (p *CloudAPIAdaptor) {
	namespace := "confidential-containers-system"
	overlayDir := path.Join("../../install/overlays", provider)

	return &CloudAPIAdaptor{
		caaDaemonSet:         &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cloud-api-adaptor-daemonset-" + provider, Namespace: namespace}},
		ccDaemonSet:          &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-daemon-install", Namespace: namespace}},
		cloudProvider:        provider,
		controllerDeployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-controller-manager", Namespace: namespace}},
		namespace:            namespace,
		kustomizeHelper:      &KustomizeHelper{configDir: overlayDir},
		runtimeClass:         &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata", Namespace: ""}},
	}
}

// GetCloudProvisioner returns a CloudProvision implementation
func GetCloudProvisioner(provider string, propertiesFile string) (CloudProvision, error) {
	var (
		err         error
		properties  map[string]string
		provisioner CloudProvision
	)

	properties = make(map[string]string)
	if propertiesFile != "" {
		f, err := os.ReadFile(propertiesFile)
		if err != nil {
			return nil, err
		}
		if err = toml.Unmarshal(f, &properties); err != nil {
			return nil, err
		}
	}

	switch provider {
	case "azure":
		provisioner, err = NewAzureCloudProvisioner("default", "default")
	case "libvirt":
		provisioner, err = NewLibvirtProvisioner(properties)
	case "ibmcloud":
		provisioner, err = NewIBMCloudProvisioner("default", "default")
	default:
		return nil, fmt.Errorf("Not implemented provisioner for %s\n", provider)
	}

	return provisioner, err
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

	fmt.Println("Uninstall CoCo and cloud-api-adaptor")
	if err = p.kustomizeHelper.Delete(ctx, cfg); err != nil {
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

	fmt.Println("Uninstall the controller manager")
	err = decoder.DecodeEachFile(ctx, os.DirFS("../../install/yamls"), "deploy.yaml", decoder.DeleteHandler(resources))
	if err != nil {
		return err
	}

	fmt.Printf("Wait for the %s deployment be deleted\n", p.controllerDeployment.GetName())
	if err = wait.For(conditions.New(resources).ResourcesDeleted(deployments),
		wait.WithTimeout(time.Minute*1)); err != nil {
		return err
	}

	return nil
}

// Deploy installs Peer Pods on the cluster.
func (p *CloudAPIAdaptor) Deploy(ctx context.Context, cfg *envconf.Config) error {
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}
	resources := client.Resources(p.namespace)

	fmt.Println("Install the controller manager")
	err = decoder.DecodeEachFile(ctx, os.DirFS("../../install/yamls"), "deploy.yaml", decoder.CreateIgnoreAlreadyExists(resources))
	if err != nil {
		return err
	}

	fmt.Printf("Wait for the %s deployment be available\n", p.controllerDeployment.GetName())
	if err = wait.For(conditions.New(resources).DeploymentConditionMatch(p.controllerDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithTimeout(time.Minute*1)); err != nil {
		return err
	}

	fmt.Println("Install CoCo and cloud-api-adaptor")
	if err := p.kustomizeHelper.Apply(ctx, cfg); err != nil {
		return err
	}

	// Wait for the CoCo installer and CAA pods be ready
	daemonSetList := map[*appsv1.DaemonSet]time.Duration{
		p.ccDaemonSet:  time.Minute * 10,
		p.caaDaemonSet: time.Minute * 2,
	}

	for ds, timeout := range daemonSetList {
		// Wait for the daemonset to have at least one pod running then wait for each pod
		// be ready.

		if err = wait.For(conditions.New(resources).ResourceMatch(ds, func(object k8s.Object) bool {
			ds = object.(*appsv1.DaemonSet)

			return ds.Status.CurrentNumberScheduled > 0
		}), wait.WithTimeout(time.Second*20)); err != nil {
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
		wait.WithTimeout(time.Second*20)); err != nil {
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
