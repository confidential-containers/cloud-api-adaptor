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

var podvmNamePrefix = "podvm"

func CreateInstanceName(podName, sandboxID string, podvmNameMax int) string {

	podName = sanitize(podName)
	sandboxID = sanitize(sandboxID)

	podNameLen := len(podName)
	if podvmNameMax > 0 && len(podvmNamePrefix)+podNameLen+10 > podvmNameMax {
		podNameLen = podvmNameMax - len(podvmNamePrefix) - 10
		if podNameLen < 0 {
			panic(fmt.Errorf("podvmNameMax is too small: %d", podvmNameMax))
		}
		fmt.Printf("podNameLen: %d", podNameLen)
	}

	vmName := fmt.Sprintf("%s-%.*s-%.8s", podvmNamePrefix, podNameLen, podName, sandboxID)

	return vmName
}
