package k8sops

import (
	"context"
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

	patch := append([]jsonPatch{}, newJsonPatch("add", "/status/capacity", "kata.peerpods.io~1vm",
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

	patch := append([]jsonPatch{}, newJsonPatch("remove", "/status/capacity", "kata.peerpods.io~1vm", ""))

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

// newJsonPatch returns a new jsonPatch object
func newJsonPatch(verb string, jsonpath string, key string, value string) jsonPatch {
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
	_, err := os.Stat("/var/run/secrets/kubernetes.io")
	return !os.IsNotExist(err)
}
