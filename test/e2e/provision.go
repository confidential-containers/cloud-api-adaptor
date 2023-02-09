package e2e

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// CloudProvision defines operations to provision the environment on cloud providers.
type CloudProvision interface {
	CreateCluster(ctx context.Context, cfg *envconf.Config) error
	CreateVPC(ctx context.Context, cfg *envconf.Config) error
	DeleteCluster(ctx context.Context, cfg *envconf.Config) error
	DeleteVPC(ctx context.Context, cfg *envconf.Config) error
	UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error
}

type PeerPods struct {
	caaDaemonSet         *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	ccDaemonSet          *appsv1.DaemonSet    // Represents the CoCo installer daemonset
	cloudProvider        string               // Cloud provider
	controllerDeployment *appsv1.Deployment   // Represents the controller manager deployment
	namespace            string               // The CoCo namespace
	runtimeClass         *nodev1.RuntimeClass // The Kata Containers runtimeclass
}

func NewPeerPods(provider string) (p *PeerPods) {
	namespace := "confidential-containers-system"
	return &PeerPods{
		caaDaemonSet:         &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cloud-api-adaptor-daemonset-" + provider, Namespace: namespace}},
		ccDaemonSet:          &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-daemon-install", Namespace: namespace}},
		cloudProvider:        provider,
		controllerDeployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "cc-operator-controller-manager", Namespace: namespace}},
		namespace:            namespace,
		runtimeClass:         &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "kata", Namespace: ""}},
	}
}

func (p *PeerPods) Delete(ctx context.Context, cfg *envconf.Config) error {
	// TODO: implement me.
	return nil
}

// Deploy installs Peer Pods on the cluster.
func (p *PeerPods) Deploy(ctx context.Context, cfg *envconf.Config) error {
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
	overlayDir := path.Join("../../install/overlays", p.cloudProvider)
	kustomizeHelper := &KustomizeHelper{configDir: overlayDir}
	if err := kustomizeHelper.Apply(ctx, cfg); err != nil {
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
			return ds.Status.NumberAvailable > 0
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

	fmt.Printf("Check the %s runtimeclass exists\n", p.runtimeClass.GetName())
	if err = resources.Get(context.TODO(), p.runtimeClass.GetName(), p.runtimeClass.GetNamespace(), p.runtimeClass); err != nil {
		return err
	}

	return nil
}

func (p *PeerPods) DoKustomize(ctx context.Context, cfg *envconf.Config) {
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