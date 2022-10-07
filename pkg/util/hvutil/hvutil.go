package hvutil

import (
	"fmt"
	"strings"

	cri "github.com/containerd/containerd/pkg/cri/annotations"
)

func GetPodName(annotations map[string]string) string {

	sandboxName := annotations[cri.SandboxName]

	// cri-o stores the sandbox name in the form of k8s_<pod name>_<namespace>_<uid>_0
	// Extract the pod name from it.
	if tmp := strings.Split(sandboxName, "_"); len(tmp) > 1 && tmp[0] == "k8s" {
		return tmp[1]
	}

	return sandboxName
}

func GetPodNamespace(annotations map[string]string) string {

	return annotations[cri.SandboxNamespace]
}

func sanitize(input string) string {

	var output string

	for _, c := range strings.ToLower(input) {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '-') {
			c = '-'
		}
		output += string(c)
	}

	return output
}

func CreateInstanceName(nodeName, podNamespace, podName, sandboxID string) string {

	nodeName = sanitize(nodeName)
	podNamespace = sanitize(podNamespace)
	podName = sanitize(podName)
	sandboxID = sanitize(sandboxID)

	vmName := fmt.Sprintf("podvm-%s-%s-%s-%.8s", nodeName, podNamespace, podName, sandboxID)

	return vmName
}
