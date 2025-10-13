// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	FEATURE_SETUP_FAILED          contextValueString = "WithSetupFailed"
	WAIT_NGINX_DEPLOYMENT_TIMEOUT                    = time.Second * 900
)

type (
	contextValueString string
	deploymentOption   func(*appsv1.Deployment)
)

func WithReplicaCount(replicas int32) deploymentOption {
	return func(deployment *appsv1.Deployment) {
		deployment.Spec.Replicas = &replicas
	}
}

func NewDeployment(namespace, deploymentName, containerName, imageName string, options ...deploymentOption) *appsv1.Deployment {
	runtimeClassName := "kata-remote"
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": containerName},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": containerName},
				},
				Spec: v1.PodSpec{
					RuntimeClassName: &runtimeClassName,
					Containers: []v1.Container{
						{
							Name:  containerName,
							Image: imageName,
							ReadinessProbe: &v1.Probe{
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								TimeoutSeconds:      1,
								FailureThreshold:    3,
							},
							LivenessProbe: &v1.Probe{
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 3,
								PeriodSeconds:       30,
								TimeoutSeconds:      1,
								FailureThreshold:    3,
							},
						},
					},
				},
			},
		},
	}

	for _, option := range options {
		option(deployment)
	}
	return deployment
}

func DoTestNginxDeployment(t *testing.T, testEnv env.Environment, assert CloudAssert) {
	deploymentName := "nginx-deployment"
	containerName := "nginx"
	imageName, err := utils.GetImage("unprivileged-nginx")
	if err != nil {
		t.Fatal(err)
	}
	replicas := int32(2)
	deployment := NewDeployment(E2eNamespace, deploymentName, containerName, imageName, WithReplicaCount(replicas))

	nginxImageFeature := features.New("Nginx image deployment test").
		WithSetup("Create nginx deployment", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx = context.WithValue(ctx, FEATURE_SETUP_FAILED, false)
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Creating nginx deployment...")
			if err = client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			waitForNginxDeploymentAvailable(ctx, t, client, deployment, replicas)
			if !t.Failed() {
				t.Log("nginx deployment is available now")
			} else {
				ctx = context.WithValue(ctx, FEATURE_SETUP_FAILED, true)
			}
			return ctx
		}).
		Assess("Access for nginx deployment test", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if ctx.Value(FEATURE_SETUP_FAILED).(bool) {
				// Test setup failed, so skip this assess
				t.Skip()
				return ctx
			}
			client, err := cfg.NewClient()
			if err != nil {
				t.Error(err)
				return ctx
			}
			var podlist v1.PodList
			if err := client.Resources(deployment.ObjectMeta.Namespace).List(ctx, &podlist); err != nil {
				t.Error(err)
				return ctx
			}
			for _, pod := range podlist.Items {
				if pod.ObjectMeta.Labels["app"] == "nginx" {
					podvmName, err := getPodvmName(ctx, client, &pod)
					if err != nil {
						t.Errorf("GetPodvmName failed: %v", err)
					}
					t.Logf("Checking PodVM for pod %s with PodVM name %s", pod.ObjectMeta.Name, podvmName)
					assert.HasPodVM(t, podvmName)
				}
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			t.Log("Deleting webserver deployment...")
			duration := 2 * time.Minute
			if err = client.Resources().Delete(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			t.Logf("Deleting deployment %s...", deploymentName)
			if err = wait.For(conditions.New(
				client.Resources()).ResourceDeleted(deployment),
				wait.WithInterval(5*time.Second),
				wait.WithTimeout(duration)); err != nil {
				t.Fatal(err)
			}
			t.Logf("Deployment %s has been successfully deleted within %.0fs", deploymentName, duration.Seconds())

			return ctx
		}).Feature()

	testEnv.Test(t, nginxImageFeature)
}

func waitForNginxDeploymentAvailable(ctx context.Context, t *testing.T, client klient.Client, deployment *appsv1.Deployment, rc int32) {
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(deployment, func(object k8s.Object) bool {
		deployObj, ok := object.(*appsv1.Deployment)
		if !ok {
			t.Logf("Not a Deployment object: %v", object)
			return false
		}
		t.Logf("Current deployment available replicas: %d", deployObj.Status.AvailableReplicas)
		return deployObj.Status.AvailableReplicas == rc
	}), wait.WithTimeout(WAIT_NGINX_DEPLOYMENT_TIMEOUT), wait.WithInterval(10*time.Second)); err != nil {
		var podlist v1.PodList
		t.Errorf("%v", err)
		if err := client.Resources(deployment.ObjectMeta.Namespace).List(ctx, &podlist); err != nil {
			t.Errorf("%v", err)
			return
		}
		for _, pod := range podlist.Items {
			if pod.ObjectMeta.Labels["app"] == "nginx" {
				// Added logs for debugging nightly tests
				t.Log("===================")
				t.Logf("Debug info for pod: %v", pod.ObjectMeta.Name)
				yamlData, err := yaml.Marshal(pod.Status)
				if err != nil {
					t.Logf("Error marshaling pod.Status to YAML: %v", err.Error())
				} else {
					t.Logf("Current Pod State: %v", string(yamlData))
				}
				if pod.Status.Phase == v1.PodRunning {
					t.Logf("Log of the pod %v", pod.Name)
					t.Logf("===================")
					podLogString, _ := GetPodLog(ctx, client, &pod)
					if podLogString != "" {
						t.Log(podLogString)
					} else {
						t.Log("No logs found")
					}
					t.Log("===================")
				}
			}
		}
	}
}
