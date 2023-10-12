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
	"strings"
	"path/filepath"
	"io/ioutil"

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
	rootSrcDir           string               // The root src directory of cloud-api-adaptor
}

type NewInstallOverlayFunc func(installDir, provider string) (InstallOverlay, error)

type KeyBrokerService struct {
        cloudProvider        string               // Cloud provider
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

func NewKeyBrokerService(provider string) (*KeyBrokerService, error) {
	// Clone kbs repo
	repoURL := "https://github.com/confidential-containers/kbs"
	cmd := exec.Command("git", "clone", repoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running git clone: %v\n", err)
		return nil, err
	}

	// Create secret
	content := []byte("This is my super secret")
	filePath := "kbs/config/kubernetes/overlays/key.bin"
	// Create the file.
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return nil, err
	}
	defer file.Close()

	// Write the content to the file.
	_, err = file.Write(content)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return nil, err
	}

	return &KeyBrokerService{
		cloudProvider:        provider,
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

// TODO: Use kustomize overlay to update this file
func UpdateKbsKustomizationFile(imagePath string, imageTag string) error {
	// Read the content of the existing kustomization.yaml file.
	filePath := "base/kustomization.yaml"
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading kustomization file: %v\n", err)
		return err
	}

	// Convert the content to a string.
	kustomizationContent := string(content)

	// Define the values to update.
	kustomizationContent = strings.Replace(kustomizationContent, "newName: ghcr.io/confidential-containers/key-broker-service", "newName: "+imagePath, -1)
	kustomizationContent = strings.Replace(kustomizationContent, "newTag: built-in-as-v0.7.0", "newTag: "+imageTag, -1)

	// Write the updated content back to the same file.
	err = ioutil.WriteFile(filePath, []byte(kustomizationContent), 0644)
	if err != nil {
		fmt.Printf("Error writing to kustomization file: %v\n", err)
		return err
	}

	fmt.Println("Kustomization file updated successfully.")
	return nil

}

func (p *KeyBrokerService) Deploy(ctx context.Context, imagePath string, imageTag string) error {
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting the current working directory: %v\n", err)
		return err
	}

	// jump to kbs kubernetes config directory
	newDirectory := "kbs/config/kubernetes/"
	err = os.Chdir(newDirectory)
	if err != nil {
		fmt.Printf("Error changing the working directory: %v\n", err)
		return err
	}

	// Note: Use kustomize overlay to update this
	err = UpdateKbsKustomizationFile(imagePath, imageTag)
	if err != nil {
		fmt.Printf("Error updating kustomization file: %v\n", err)
		return err
	}

	// Deploy kbs
	k8sCnfDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting the current working directory: %v\n", err)
		return err
	}
	fmt.Println(k8sCnfDir)

	keyFile := filepath.Join(k8sCnfDir, "overlays/key.bin")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		fmt.Println("key.bin file does not exist")
		//return err
	}

	kbsCert := filepath.Join(k8sCnfDir, "base/kbs.pem")
	if _, err := os.Stat(kbsCert); os.IsNotExist(err) {
		kbsKey := filepath.Join(k8sCnfDir, "base/kbs.key")
		keyOutputFile, err := os.Create(kbsKey)
		if err != nil {
			fmt.Printf("Error creating key file: %v\n", err)
			os.Exit(1)
		}
		defer keyOutputFile.Close()

		opensslGenPKeyCmd := exec.Command("openssl", "genpkey", "-algorithm", "ed25519")
		opensslGenPKeyCmd.Stdout = keyOutputFile
		opensslGenPKeyCmd.Stderr = os.Stderr
		fmt.Printf("Running command: %v\n", opensslGenPKeyCmd.Args)
		if err := opensslGenPKeyCmd.Run(); err != nil {
			fmt.Printf("Error generating key: %v\n", err)
			return err
		}

		opensslPKeyCmd := exec.Command("openssl", "pkey", "-in", kbsKey, "-pubout", "-out", kbsCert)
		opensslPKeyCmd.Stdout = os.Stdout
		opensslPKeyCmd.Stderr = os.Stderr
		if err := opensslPKeyCmd.Run(); err != nil {
			fmt.Printf("Error creating kbs.pem: %v\n", err)
			return err
		}
	}

	kubectlApplyCmd := exec.Command("kubectl", "apply", "-k", k8sCnfDir+"/overlays")
	kubectlApplyCmd.Stdout = os.Stdout
	kubectlApplyCmd.Stderr = os.Stderr
	if err := kubectlApplyCmd.Run(); err != nil {
		fmt.Printf("Error running 'kubectl apply': %v\n", err)
		return err
	}

	// Return to the original working directory.
	err = os.Chdir(originalDir)
	if err != nil {
		fmt.Printf("Error changing back to the original working directory: %v\n", err)
		return err
	}

	// remove kbs repo
	directoryPath := "kbs"

	err = os.RemoveAll(directoryPath)
	if err != nil {
		fmt.Printf("Error deleting directory: %v\n", err)
		return err
	}

	return nil
}

func (p *KeyBrokerService) Delete(ctx context.Context) error {
	// Remove kbs deployment
	k8sCnfDir := "kbs/config/kubernetes"
	kubectlDeleteCmd := exec.Command("kubectl", "delete", "-k", k8sCnfDir+"/overlays")
	kubectlDeleteCmd.Stdout = os.Stdout
	kubectlDeleteCmd.Stderr = os.Stderr

	err := kubectlDeleteCmd.Run()
	if err != nil {
		fmt.Printf("Error running 'kubectl delete': %v\n", err)
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
