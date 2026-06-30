// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/k8sops"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const OldVMDeletionTimeout = time.Second * 30

func DoTestCaaDaemonsetRollingUpdate(t *testing.T, testEnv env.Environment, assert RollingUpdateAssert) {
	runtimeClassName := "kata-remote"
	deploymentName := "webserver-deployment"
	containerName := "webserver"
	imageName := "python:3"
	serviceName := "webserver-service"
	portName := "port80"
	rc := int32(2)
	labelsMap := map[string]string{
		"app": "webserver-app",
	}
	verifyPodName := "verify-pod"
	verifyContainerName := "verify-container"
	verifyImageName := "quay.io/curl/curl:latest"

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: E2eNamespace,
			Labels:    labelsMap,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rc,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsMap,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsMap,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            containerName,
							Image:           imageName,
							ImagePullPolicy: v1.PullAlways,
							Command: []string{
								"python",
								"-m",
								"http.server",
							},
						},
					},
					RuntimeClassName: &runtimeClassName,
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"webserver-app"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
		},
	}

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: E2eNamespace,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name:       portName,
					Port:       int32(80),
					TargetPort: intstr.FromInt(8000),
					Protocol:   v1.ProtocolTCP,
				},
			},
			Selector: labelsMap,
		},
	}

	verifyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      verifyPodName,
			Namespace: E2eNamespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:  verifyContainerName,
					Image: verifyImageName,
					Command: []string{
						"/bin/sh",
						"-c",
						// Not complete command; will append later
					},
				},
			},
		},
	}

	upgradeFeature := features.New("CAA DaemonSet upgrade test").
		WithSetup("Create webserver deployment and service", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			t.Log("Creating webserver deployment...")
			if err = client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			waitForDeploymentAvailable(t, client, deployment, rc)
			t.Log("webserver deployment is available now")

			// Cache Pod VM instance IDs before upgrade
			assert.CachePodVMIDs(t, deploymentName)

			t.Log("Creating webserver Service")
			if err = client.Resources().Create(ctx, svc); err != nil {
				t.Fatal(err)
			}
			clusterIP := WaitForClusterIP(t, client, svc)
			t.Logf("webserver service is available on cluster IP: %s", clusterIP)

			// Update verify command
			verifyPod.Spec.Containers[0].Command = append(
				verifyPod.Spec.Containers[0].Command,
				fmt.Sprintf(`
						while true; do
						if ! curl -m 5 -IsSf %s:80 > /dev/null; then
							echo "disconnected: $(date)"
							exit 1
						else
							echo "connected: $(date)"
							sleep 1
						fi
						done
				`, clusterIP))
			if err = client.Resources().Create(ctx, verifyPod); err != nil {
				t.Fatal(err)
			}
			if err = wait.For(conditions.New(client.Resources()).PodRunning(verifyPod), wait.WithTimeout(WaitPodRunningTimeout)); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("Access for upgrade test", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			caaDaemonSetName := "cloud-api-adaptor-daemonset"
			caaNamespace := pv.GetCAANamespace()

			ds := &appsv1.DaemonSet{}
			if err = client.Resources().Get(ctx, caaDaemonSetName, caaNamespace, ds); err != nil {
				t.Fatal(err)
			}
			t.Log("Force to update CAA pods by increasing StartupProbe.FailureThreshold")
			ds.Spec.Template.Spec.Containers[0].StartupProbe.FailureThreshold += 1
			if err = client.Resources().Update(ctx, ds); err != nil {
				t.Fatal(err)
			}

			waitForCaaDaemonSetUpdated(t, client, ds, rc)

			// Wait for webserver deployment available again
			waitForDeploymentAvailable(t, client, deployment, rc)

			if err = client.Resources().Get(ctx, verifyPodName, E2eNamespace, verifyPod); err != nil {
				t.Fatal(err)
			}
			t.Logf("verify pod status: %s", verifyPod.Status.Phase)
			if verifyPod.Status.Phase != v1.PodRunning {
				clientset, err := kubernetes.NewForConfig(client.RESTConfig())
				if err != nil {
					t.Logf("Failed to new client set: %v", err)
				} else {
					req := clientset.CoreV1().Pods(E2eNamespace).GetLogs(verifyPodName, &v1.PodLogOptions{})
					podLogs, err := req.Stream(ctx)
					if err != nil {
						t.Logf("Failed to get pod logs: %v", err)
					} else {
						defer podLogs.Close()
						buf := new(bytes.Buffer)
						_, err = io.Copy(buf, podLogs)
						if err != nil {
							t.Logf("Failed to copy pod logs: %v", err)
						} else {
							podLogString := strings.TrimSpace(buf.String())
							t.Logf("verify pod logs: \n%s", podLogString)
						}
					}
				}
				t.Fatal(fmt.Errorf("verify pod is not running"))
			}

			time.Sleep(OldVMDeletionTimeout)
			t.Log("Verify old VM instances have been deleted:")
			assert.VerifyOldVMDeleted(t)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			t.Log("Deleting verify pod...")
			if err = client.Resources().Delete(ctx, verifyPod); err != nil {
				t.Fatal(err)
			}

			t.Log("Deleting webserver service...")
			if err = client.Resources().Delete(ctx, svc); err != nil {
				t.Fatal(err)
			}

			t.Log("Deleting webserver deployment...")
			if err = client.Resources().Delete(ctx, deployment); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()

	testEnv.Test(t, upgradeFeature)
}

func waitForCaaDaemonSetUpdated(t *testing.T, client klient.Client, ds *appsv1.DaemonSet, rc int32) {
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(ds, func(object k8s.Object) bool {
		dsObj, ok := object.(*appsv1.DaemonSet)
		if !ok {
			t.Logf("Not a DaemonSet object: %v", object)
			return false
		}

		t.Logf("Current CAA DaemonSet UpdatedNumberScheduled: %d", dsObj.Status.UpdatedNumberScheduled)
		t.Logf("Current CAA DaemonSet NumberAvailable: %d", dsObj.Status.NumberAvailable)
		return dsObj.Status.UpdatedNumberScheduled == rc && dsObj.Status.NumberAvailable == rc
	}), wait.WithTimeout(WaitDeploymentAvailableTimeout)); err != nil {
		t.Fatal(err)
	}
}

// DoTestExtendedResourcesAfterRollout verifies that node extended resources
// (kata.peerpods.io/vm) survive a zero-workload CAA DaemonSet rollout restart.
//
// The race this exercises: the new CAA pod advertises the resource and becomes
// Ready before the old pod finishes terminating. Without the owner-annotation
// fix the old pod's Shutdown() removes the resource the new pod already set,
// leaving the node with no capacity and preventing peer pod scheduling.
func DoTestExtendedResourcesAfterRollout(t *testing.T, testEnv env.Environment) {
	const (
		caaDaemonSetName = "cloud-api-adaptor-daemonset"
		extResourceName  = "kata.peerpods.io/vm"
	)
	caaNamespace := pv.GetCAANamespace()

	feature := features.New("Extended resources survive CAA rollout restart").
		Assess("Rollout CAA DaemonSet and verify extended resources remain on all nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			clientset, err := kubernetes.NewForConfig(client.RESTConfig())
			if err != nil {
				t.Fatal(err)
			}

			// Get the DaemonSet and record the expected pod count.
			ds := &appsv1.DaemonSet{}
			if err = client.Resources().Get(ctx, caaDaemonSetName, caaNamespace, ds); err != nil {
				t.Fatal(err)
			}
			rc := ds.Status.DesiredNumberScheduled
			if rc == 0 {
				t.Skip("No CAA pods scheduled; skipping test")
			}

			// List all current CAA pods to snapshot node names and pre-rollout pod UIDs.
			caaPodList, err := clientset.CoreV1().Pods(caaNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=cloud-api-adaptor",
			})
			if err != nil {
				t.Fatalf("Failed to list CAA pods: %v", err)
			}
			if len(caaPodList.Items) == 0 {
				t.Fatal("Expected at least one CAA pod but found none")
			}

			type nodeSnapshot struct {
				nodeName  string
				oldPodUID string
			}
			snapshots := make([]nodeSnapshot, 0, len(caaPodList.Items))
			for _, pod := range caaPodList.Items {
				snapshots = append(snapshots, nodeSnapshot{
					nodeName:  pod.Spec.NodeName,
					oldPodUID: string(pod.UID),
				})
				t.Logf("Pre-rollout: node=%s pod-uid=%s", pod.Spec.NodeName, pod.UID)
			}

			// Confirm extended resources are present before the rollout.
			// If any node lacks the resource, extended resources are disabled
			// (PeerPodsLimitPerNode < 0) and this test does not apply.
			for _, snap := range snapshots {
				node, err := clientset.CoreV1().Nodes().Get(ctx, snap.nodeName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get node %s: %v", snap.nodeName, err)
				}
				qty, ok := node.Status.Capacity[v1.ResourceName(extResourceName)]
				if !ok || qty.IsZero() {
					t.Skipf("Node %s has no %s capacity; extended resources disabled — skipping", snap.nodeName, extResourceName)
				}
				t.Logf("Pre-rollout: node=%s %s=%s owner=%q",
					snap.nodeName, extResourceName, qty.String(), node.Annotations[k8sops.OwnerAnnotation])
			}

			// Trigger rollout via the same annotation kubectl rollout restart writes.
			// This is a pure template hash bump with no functional side-effect.
			t.Log("Triggering CAA DaemonSet rollout...")
			if ds.Spec.Template.Annotations == nil {
				ds.Spec.Template.Annotations = make(map[string]string)
			}
			ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
			if err = client.Resources().Update(ctx, ds); err != nil {
				t.Fatal(err)
			}

			waitForCaaDaemonSetUpdated(t, client, ds, rc)
			t.Log("CAA DaemonSet rollout complete")

			// Assert extended resources are still present on every node.
			for _, snap := range snapshots {
				node, err := clientset.CoreV1().Nodes().Get(ctx, snap.nodeName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Post-rollout: failed to get node %s: %v", snap.nodeName, err)
				}

				qty, ok := node.Status.Capacity[v1.ResourceName(extResourceName)]
				if !ok || qty.IsZero() {
					t.Errorf("Post-rollout: node %s lost %s — old CAA shutdown incorrectly removed the resource",
						snap.nodeName, extResourceName)
				} else {
					t.Logf("Post-rollout: node=%s %s=%s (preserved)", snap.nodeName, extResourceName, qty.String())
				}

				// Verify the owner annotation was updated to the new pod's UID,
				// confirming the new instance took ownership before the old one shut down.
				currentOwner := node.Annotations[k8sops.OwnerAnnotation]
				switch currentOwner {
				case "":
					t.Errorf("Post-rollout: node %s is missing the %s annotation", snap.nodeName, k8sops.OwnerAnnotation)
				case snap.oldPodUID:
					t.Errorf("Post-rollout: node %s owner annotation still points to the old pod %s — ownership handoff failed",
						snap.nodeName, snap.oldPodUID)
				default:
					t.Logf("Post-rollout: node=%s owner updated from %s to %s (ownership handed off)",
						snap.nodeName, snap.oldPodUID, currentOwner)
				}
			}

			return ctx
		}).
		Assess("Create a peer pod to verify CAA is functional after rollout", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			podOrErr := NewBusyboxPodWithName(E2eNamespace, "caa-rollout-verify-pod")
			if podOrErr.err != nil {
				t.Fatalf("Failed to build peer pod spec: %v", podOrErr.err)
			}
			pod := podOrErr.pod

			t.Logf("Creating peer pod %s in namespace %s...", pod.Name, E2eNamespace)
			if err = client.Resources().Create(ctx, pod); err != nil {
				t.Fatalf("Failed to create peer pod: %v", err)
			}
			defer func() {
				if err = client.Resources().Delete(ctx, pod); err != nil {
					t.Logf("Failed to delete peer pod: %v", err)
				}
			}()

			if err = wait.For(conditions.New(client.Resources()).PodPhaseMatch(pod, v1.PodRunning),
				wait.WithTimeout(WaitPodRunningTimeout)); err != nil {
				t.Fatalf("Peer pod did not reach Running state: %v", err)
			}
			if err = wait.For(conditions.New(client.Resources()).ContainersReady(pod),
				wait.WithTimeout(WaitPodRunningTimeout)); err != nil {
				t.Fatalf("Peer pod containers not ready: %v", err)
			}
			t.Logf("Peer pod %s is Running — CAA is functional after rollout", pod.Name)

			return ctx
		}).
		Feature()

	testEnv.Test(t, feature)
}

func waitForDeploymentAvailable(t *testing.T, client klient.Client, deployment *appsv1.Deployment, rc int32) {
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(deployment, func(object k8s.Object) bool {
		deployObj, ok := object.(*appsv1.Deployment)
		if !ok {
			t.Logf("Not a Deployment object: %v", object)
			return false
		}

		t.Logf("Current deployment available replicas: %d", deployObj.Status.AvailableReplicas)
		return deployObj.Status.AvailableReplicas == rc
	}), wait.WithTimeout(WaitDeploymentAvailableTimeout)); err != nil {
		t.Fatal(err)
	}
}
