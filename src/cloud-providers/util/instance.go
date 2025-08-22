package util

import (
	"fmt"
	"strings"
)

const (
	podvmNamePrefix = "podvm"
)

func sanitize(input string) string {

	var output string

	for _, c := range strings.ToLower(input) {
		if (c < 'a' || 'z' < c) && (c < '0' || '9' < c) && c != '-' {
			c = '-'
		}
		output += string(c)
	}

	return output
}

func GenerateInstanceName(podName, sandboxID string, podvmNameMax int) string {

	podName = sanitize(podName)
	sandboxID = sanitize(sandboxID)

	prefixLen := len(podvmNamePrefix)
	podNameLen := len(podName)
	if podvmNameMax > 0 && prefixLen+podNameLen+10 > podvmNameMax {
		podNameLen = podvmNameMax - prefixLen - 10
		if podNameLen < 0 {
			panic(fmt.Errorf("podvmNameMax is too small: %d", podvmNameMax))
		}
		fmt.Printf("podNameLen: %d", podNameLen)
	}

	instanceName := fmt.Sprintf("%s-%.*s-%.8s", podvmNamePrefix, podNameLen, podName, sandboxID)

	return instanceName
}
