package k8sops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// AdvertiseExtendedResources sets up extended resources for the node
func AdvertiseExtendedResources(peerPodsLimitPerNode int) error {

	logger.Printf("set up extended resources")

	if peerPodsLimitPerNode < 0 {
		logger.Printf("No extended resource limit to set")
		return nil
	}

	nodeName := os.Getenv("NODE_NAME")

	patch := append([]jsonPatch{}, newJSONPatch("add", "/status/capacity", "kata.peerpods.io~1vm",
		strconv.Itoa(peerPodsLimitPerNode)))

	config, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %v", err)
	}

	cli, err := getClient(config)
	if err != nil {
		return fmt.Errorf("failed to get k8s client: %v", err)
	}

	err = patchNodeStatus(cli, nodeName, patch)
	if err != nil {
		logger.Printf("Failed to set extended resource for node %s", nodeName)
		return err
	}

	logger.Printf("Successfully set extended resource for node %s", nodeName)

	return nil
}

// Patch the status of a node to remove extended resources
func RemoveExtendedResources() error {

	logger.Printf("remove extended resources")

	nodeName := os.Getenv("NODE_NAME")

	patch := append([]jsonPatch{}, newJSONPatch("remove", "/status/capacity", "kata.peerpods.io~1vm", ""))

	config, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %v", err)
	}

	cli, err := getClient(config)
	if err != nil {
		return fmt.Errorf("failed to get k8s client: %v", err)
	}

	err = patchNodeStatus(cli, nodeName, patch)
	if err != nil {
		logger.Printf("Failed to remove extended resource for node %s", nodeName)
		return err
	}

	logger.Printf("Successfully removed extended resource for node %s", nodeName)

	return nil
}

// Auths contains Registries with credentials
type Auths struct {
	Registries Registries `json:"auths"`
}

// Registries contains credentials for hosts
type Registries map[string]Auth

// Auth contains credentials for a given host
type Auth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth"`
}

// GetImagePullSecrets gets image pull secrets for the specified pod
func GetImagePullSecrets(podName string, namespace string) ([]byte, error) {

	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %v", err)
	}

	cli, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s client: %v", err)
	}

	pod, err := cli.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	accountName := pod.Spec.ServiceAccountName
	if accountName == "" {
		accountName = "default"
	}
	serviceaAccount, err := cli.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), accountName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	auths := Auths{}
	auths.Registries = make(map[string]Auth)
	for _, secret := range serviceaAccount.ImagePullSecrets {
		err := addAuths(cli, namespace, secret.Name, &auths)
		if err != nil {
			return nil, err
		}
	}
	for _, secret := range pod.Spec.ImagePullSecrets {
		err := addAuths(cli, namespace, secret.Name, &auths)
		if err != nil {
			return nil, err
		}
	}

	if len(auths.Registries) > 0 {
		authJSON, err := json.Marshal(auths)
		if err != nil {
			return nil, err
		}
		return authJSON, nil
	}
	return nil, nil
}

// getAuths get auth credentials from specified docker secret
func addAuths(cli *k8sclient.Clientset, namespace string, secretName string, auths *Auths) error {
	secret, err := cli.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		// Ignore errors getting secrets to match K8S behavior
		return nil
	}
	registries := Registries{}
	if secretData, ok := secret.Data[".dockerconfigjson"]; ok {
		auths := Auths{}
		err := json.Unmarshal(secretData, &auths)
		if err != nil {
			return err
		}
		registries = auths.Registries
	} else if secretData, ok := secret.Data[".dockercfg"]; ok {
		err = json.Unmarshal(secretData, &registries)
		if err != nil {
			return err
		}
	}
	for registry, creds := range registries {
		if creds.Auth == "" {
			if creds.Username != "" && creds.Password != "" {
				creds.Auth = base64.StdEncoding.EncodeToString([]byte(creds.Username + ":" + creds.Password))
			} else {
				continue
			}
		}
		creds.Username = ""
		creds.Password = ""
		auths.Registries[registry] = creds
	}
	return nil
}

// patchNodeStatus patches the status of a node
func patchNodeStatus(c *k8sclient.Clientset, nodeName string, patches []jsonPatch) error {
	if len(patches) > 0 {
		data, err := json.Marshal(patches)
		if err == nil {
			_, err = c.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, data, metav1.PatchOptions{}, "status")
		}
		return err
	}
	logger.Printf("empty patch for node, no change")
	return nil
}

// jsonPatch is a json marshaling helper used for patching API objects
type jsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

// newJSONPatch returns a new jsonPatch object
func newJSONPatch(verb string, jsonpath string, key string, value string) jsonPatch {
	return jsonPatch{verb, path.Join(jsonpath, strings.ReplaceAll(key, "/", "~1")), value}
}

// Get kubeconfig from environment
func getKubeConfig() (*restclient.Config, error) {
	kubeConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubeConfig, nil
}

// getClient creates and returns a new clientset from given config
func getClient(kubeConfig *restclient.Config) (*k8sclient.Clientset, error) {
	clientSet, err := k8sclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

// isKubernetesEnvironment checks if the environment is Kubernetes
func IsKubernetesEnvironment() bool {
	nodeName := os.Getenv("NODE_NAME")
	return nodeName != ""
}
