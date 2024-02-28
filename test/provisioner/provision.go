// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
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

// KbsInstallOverlay implements the InstallOverlay interface
type KbsInstallOverlay struct {
	overlay *KustomizeOverlay
}

var NewProvisionerFunctions = make(map[string]NewProvisionerFunc)

type CloudAPIAdaptor struct {
	caaDaemonSet         *appsv1.DaemonSet    // Represents the cloud-api-adaptor daemonset
	ccDaemonSet          *appsv1.DaemonSet    // Represents the CoCo installer daemonset
	cloudProvider        string               // Cloud provider
	controllerDeployment *appsv1.Deployment   // Represents the controller manager deployment
	namespace            string               // The CoCo namespace
	installOverlay       InstallOverlay       // Pointer to the kustomize overlay
	runtimeClass         *nodev1.RuntimeClass // The Kata Containers runtimeclass
	rootSrcDir           string               // The root src directory of cloud-api-adaptor
}

type NewInstallOverlayFunc func(installDir, provider string) (InstallOverlay, error)

type KeyBrokerService struct {
	installOverlay InstallOverlay // Pointer to the kustomize overlay
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

func saveToFile(filename string, content []byte) error {
	// Save contents to file
	err := os.WriteFile(filename, content, 0644)
	if err != nil {
		return fmt.Errorf("writing contents to file: %w", err)
	}
	return nil
}

func NewKeyBrokerService(clusterName string) (*KeyBrokerService, error) {
	log.Info("creating key.bin")

	// Create secret
	content := []byte("This is my cluster name: " + clusterName)
	filePath := "kbs/kbs/config/kubernetes/overlays/key.bin"
	// Create the file.
	file, err := os.Create(filePath)
	if err != nil {
		err = fmt.Errorf("creating file: %w\n", err)
		log.Errorf("%v", err)
		return nil, err
	}
	defer file.Close()

	// Write the content to the file.
	err = saveToFile(filePath, content)
	if err != nil {
		err = fmt.Errorf("writing to the file: %w\n", err)
		log.Errorf("%v", err)
		return nil, err
	}

	k8sCnfDir, err := os.Getwd()
	if err != nil {
		err = fmt.Errorf("getting the current working directory: %w\n", err)
		log.Errorf("%v", err)
		return nil, err
	}
	fmt.Println(k8sCnfDir)

	kbsCert := filepath.Join(k8sCnfDir, "kbs/kbs/config/kubernetes/base/kbs.pem")
	if _, err := os.Stat(kbsCert); os.IsNotExist(err) {
		kbsKey := filepath.Join(k8sCnfDir, "kbs/kbs/config/kubernetes/base/kbs.key")
		keyOutputFile, err := os.Create(kbsKey)
		if err != nil {
			err = fmt.Errorf("creating key file: %w\n", err)
			log.Errorf("%v", err)
			return nil, err
		}
		defer keyOutputFile.Close()

		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			err = fmt.Errorf("generating Ed25519 key pair: %w\n", err)
			log.Errorf("%v", err)
			return nil, err
		}

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKey,
		})

		// Save private key to file
		err = saveToFile(kbsKey, privateKeyPEM)
		if err != nil {
			err = fmt.Errorf("saving private key to file: %w\n", err)
			log.Errorf("%v", err)
			return nil, err
		}

		publicKey := privateKey.Public().(ed25519.PublicKey)
		publicKeyX509, err := x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			err = fmt.Errorf("generating Ed25519 public key: %w\n", err)
			log.Errorf("%v", err)
			return nil, err
		}

		publicKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyX509,
		})

		// Save public key to file
		err = saveToFile(kbsCert, publicKeyPEM)
		if err != nil {
			err = fmt.Errorf("saving public key to file: %w\n", err)
			log.Errorf("%v", err)
			return nil, err
		}

	}

	overlay, err := NewBaseKbsInstallOverlay("kbs")
	if err != nil {
		return nil, err
	}

	return &KeyBrokerService{
		installOverlay: overlay,
	}, nil
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

func NewBaseKbsInstallOverlay(installDir string) (InstallOverlay, error) {
	log.Info("Creating kbs install overlay")
	overlay, err := NewKustomizeOverlay(filepath.Join(installDir, "kbs/config/kubernetes/base"))
	if err != nil {
		return nil, err
	}

	return &KbsInstallOverlay{
		overlay: overlay,
	}, nil
}

func NewKbsInstallOverlay(installDir string) (InstallOverlay, error) {
	log.Info("Creating kbs install overlay")
	overlay, err := NewKustomizeOverlay(filepath.Join(installDir, "kbs/config/kubernetes/overlays"))
	if err != nil {
		return nil, err
	}

	return &KbsInstallOverlay{
		overlay: overlay,
	}, nil
}

func (lio *KbsInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Apply(ctx, cfg)
}

func (lio *KbsInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Delete(ctx, cfg)
}

func (lio *KbsInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	var err error
	log.Infof("Updating kbs image with %q", props["KBS_IMAGE"])
	if err = lio.overlay.SetKustomizeImage("kbs-container-image", "newName", props["KBS_IMAGE"]); err != nil {
		return err
	}

	log.Infof("Updating kbs image tag with %q", props["KBS_IMAGE_TAG"])
	if err = lio.overlay.SetKustomizeImage("kbs-container-image", "newTag", props["KBS_IMAGE_TAG"]); err != nil {
		return err
	}

	return nil
}

func (p *KeyBrokerService) GetKbsPodIP(ctx context.Context, cfg *envconf.Config) (string, error) {
	client, err := cfg.NewClient()
	if err != nil {
		return "", err
	}

	namespace := "coco-tenant"
	deploymentName := "kbs"

	err = AllPodsRunning(ctx, cfg, namespace)
	if err != nil {
		err = fmt.Errorf("All pods are not running: %w\n", err)
		log.Errorf("%v", err)
		return "", err
	}

	resources := client.Resources(namespace)
	podList := &corev1.PodList{}
	err = resources.List(context.TODO(), podList)
	if err != nil {
		err = fmt.Errorf("Error listing pods: %w\n", err)
		log.Errorf("%v", err)
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
		return "", fmt.Errorf("No pod with label selector found")
	}

	fmt.Printf("Pod IP: %s\n", matchingPod.Status.PodIP)
	return matchingPod.Status.PodIP, nil
}

func (p *KeyBrokerService) Deploy(ctx context.Context, cfg *envconf.Config, props map[string]string) error {
	log.Info("Customize the overlay yaml file")
	if err := p.installOverlay.Edit(ctx, cfg, props); err != nil {
		return err
	}

	// Create kustomize pointer for overlay directory with updated changes
	tmpoverlay, err := NewKbsInstallOverlay("kbs")
	if err != nil {
		return err
	}

	log.Info("Install Kbs")
	if err := tmpoverlay.Apply(ctx, cfg); err != nil {
		return err
	}

	return nil
}

func (p *KeyBrokerService) Delete(ctx context.Context, cfg *envconf.Config) error {
	// Create kustomize pointer for overlay directory with updated changes
	tmpoverlay, err := NewKbsInstallOverlay("kbs")
	if err != nil {
		return err
	}

	log.Info("Uninstall the cloud-api-adaptor")
	if err = tmpoverlay.Delete(ctx, cfg); err != nil {
		return err
	}
	return nil
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
	cmd = exec.Command("kubectl", "delete", "-k", "github.com/confidential-containers/operator/config/release?ref=v0.8.0")
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

	log.Info("Delete the peerpod-ctrl deployment")
	cmd = exec.Command("make", "-C", "peerpod-ctrl", "undeploy")
	// Run the command from the root src dir
	cmd.Dir = p.rootSrcDir
	// Set the KUBECONFIG env var
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
	stdoutStderr, err = cmd.CombinedOutput()
	log.Tracef("%v, output: %s", cmd, stdoutStderr)
	if err != nil {
		return err
	}

	log.Info("Wait for the peerpod-ctrl deployment to be deleted")
	if err = wait.For(conditions.New(resources).ResourcesDeleted(
		&appsv1.DeploymentList{Items: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{Name: "peerpod-ctrl-controller-manager", Namespace: p.namespace}}}}),
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
	cmd := exec.Command("kubectl", "apply", "-k", "github.com/confidential-containers/operator/config/release?ref=v0.8.0")
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

	log.Info("Installing peerpod-ctrl")
	cmd = exec.Command("make", "-C", "peerpod-ctrl", "deploy")
	// Run the deployment from the root src dir
	cmd.Dir = p.rootSrcDir
	// Set the KUBECONFIG env var
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG="+cfg.KubeconfigFile()))
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
