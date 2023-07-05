// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type patchLabel struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Adds the worker label to all workers nodes in a given cluster
func AddNodeRoleWorkerLabel(ctx context.Context, clusterName string, cfg *envconf.Config) error {
	fmt.Printf("Adding worker label to nodes belonging to: %s\n", clusterName)
	client, err := cfg.NewClient()
	if err != nil {
		return err
	}

	nodelist := &corev1.NodeList{}
	if err := client.Resources().List(ctx, nodelist); err != nil {
		return err
	}
	// Use full path to avoid overwriting other labels (see RFC 6902)
	payload := []patchLabel{{
		Op: "add",
		// "/" must be written as ~1 (see RFC 6901)
		Path:  "/metadata/labels/node.kubernetes.io~1worker",
		Value: "",
	}}
	payloadBytes, _ := json.Marshal(payload)
	workerStr := clusterName + "-worker"
	for _, node := range nodelist.Items {
		if strings.Contains(node.Name, workerStr) {
			if err := client.Resources().Patch(ctx, &node, k8s.Patch{PatchType: types.JSONPatchType, Data: payloadBytes}); err != nil {
				return err
			}
		}

	}
	return nil
}
