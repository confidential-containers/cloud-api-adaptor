// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const WAIT_POD_RUNNING_TIMEOUT = time.Second * 300

// doTestCreateSimplePod tests a simple peer-pod can be created.
func doTestCreateSimplePod(t *testing.T, assert CloudAssert) {
	// TODO: generate me.
	namespace := "default"
	name := "simple-peer-pod"
	pod := newPod(namespace, name, "nginx", "kata-remote")

	simplePodFeature := features.New("Simple Peer Pod").
		WithSetup("Create pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, pod); err != nil {
				t.Fatal(err)
			}
			if err = wait.For(conditions.New(client.Resources()).PodRunning(pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("PodVM is created", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			assert.HasPodVM(t, name)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Delete(ctx, pod); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()
	testEnv.Test(t, simplePodFeature)
}

func doTestCreatePodWithConfigMap(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	name := "configmap-pod"
	configmapname := "nginx-config"
	configmapData := map[string]string{"example.txt": "Hello, world"}
	containerName := "nginx"
	pod := newPodWithConfigMap(namespace, name, containerName, "kata-remote", configmapname)
	configmap := newConfigMap(namespace, configmapname, configmapData)
	nginxPodFeature := features.New("Configmap Pod").
		WithSetup("Create pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			if err = client.Resources().Create(ctx, configmap); err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, pod); err != nil {
				t.Fatal(err)
			}
			if err = wait.For(conditions.New(client.Resources()).PodRunning(pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("Configmap is created and contains data", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var podlist v1.PodList
			var stdout, stderr bytes.Buffer
			if err := cfg.Client().Resources(namespace).List(context.TODO(), &podlist); err != nil {
				t.Fatal(err)
			}
			for _, i := range podlist.Items {
				if i.ObjectMeta.Name == name {
					if err := cfg.Client().Resources(namespace).ExecInPod(ctx, namespace, name, containerName, []string{"cat", "/etc/config/example.txt"}, &stdout, &stderr); err != nil {
						t.Log(stderr.String())
						t.Fatal(err)
					}
				}
			}
			if stdout.String() == "Hello, world" {
				log.Infof("Data Inside Configmap: %s", stdout.String())
			} else {
				t.Errorf("Configmap with invalid Data: %s", stdout.String())
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Delete(ctx, pod); err != nil {
				t.Fatal(err)
			} else {
				log.Infof("Deleting pod... %s", name)
			}
			if err = client.Resources().Delete(ctx, configmap); err != nil {
				t.Fatal(err)
			} else {
				log.Infof("Deleting Configmap... %s", configmapname)
			}

			return ctx
		}).Feature()
	testEnv.Test(t, nginxPodFeature)
}
func doTestCreatePodWithSecret(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	name := "secret-pod"
	secretname := "nginx-secret"
	containerName := "nginx"
	secretData := map[string][]byte{"password": []byte("123456"), "username": []byte("admin")}
	pod := newPodWithSecret(namespace, name, containerName, "kata-remote", secretname)
	secret := newSecret(namespace, secretname, secretData)
	nginxPodFeature := features.New("Secret Pod").
		WithSetup("Create pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, secret); err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Create(ctx, pod); err != nil {
				t.Fatal(err)
			}

			if err = wait.For(conditions.New(client.Resources()).PodRunning(pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).
		Assess("Secret is created and contains data", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var podlist v1.PodList
			var usernameStdOut, passwordStdOut, usernameStdErr, passwordStdErr bytes.Buffer
			if err := cfg.Client().Resources(namespace).List(context.TODO(), &podlist); err != nil {
				t.Fatal(err)
			}
			for _, i := range podlist.Items {
				if i.ObjectMeta.Name == name {
					if err := cfg.Client().Resources(namespace).ExecInPod(ctx, namespace, name, containerName, []string{"cat", "/etc/secret/username"}, &usernameStdOut, &usernameStdErr); err != nil {
						t.Log(usernameStdErr.String())
						t.Fatal(err)
					}
					if err := cfg.Client().Resources(namespace).ExecInPod(ctx, namespace, name, containerName, []string{"cat", "/etc/secret/password"}, &passwordStdOut, &passwordStdErr); err != nil {
						t.Log(passwordStdErr.String())
						t.Fatal(err)
					}
				}
			}
			if usernameStdOut.String() == "admin" && passwordStdOut.String() == "123456" {
				log.Infof("Username inside volume: %s", usernameStdOut.String())
				log.Infof("Password inside volume: %s", passwordStdOut.String())
			} else {
				t.Errorf("Secret with Invalid user: %s and password: %s", usernameStdOut.String(), passwordStdOut.String())
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Delete(ctx, pod); err != nil {
				t.Fatal(err)
			} else {
				log.Infof("Deleting pod... %s", name)
			}
			if err = client.Resources().Delete(ctx, secret); err != nil {
				t.Fatal(err)
			} else {
				log.Infof("Deleting Secret... %s", secretname)
			}

			return ctx
		}).Feature()
	testEnv.Test(t, nginxPodFeature)
}
func doTestCreatePeerPodContainerWithExternalIPAccess(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podname := "busy-box-pod"
	pod := newBusyboxPod(namespace, podname, "busybox", "kata-remote")
	PublicpodFeature := features.New("Peer Pod Container").
		WithSetup("Create pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources(namespace).Create(ctx, pod); err != nil {
				t.Fatal(err)
			}
			if err = wait.For(conditions.New(client.Resources()).PodRunning(pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Peer Pod Container Connected to External IP", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var podlist v1.PodList
			var pingstdout, pingstderr bytes.Buffer
			if err := cfg.Client().Resources(namespace).List(ctx, &podlist); err != nil {
				t.Fatal(err)
			}
			for _, i := range podlist.Items {
				if i.ObjectMeta.Name == podname {
					if err := cfg.Client().Resources(namespace).ExecInPod(ctx, namespace, podname, "busybox", []string{"ping", "-c", "1", "www.google.com"}, &pingstdout, &pingstderr); err != nil {
						log.Println(pingstderr.String())
						t.Fatal(err)
					} else {
						log.Println("Pinging www.google.com ...")
					}
				}
			}
			if pingstdout.String() != "" {
				log.Printf("Output of ping command in busybox : %s", pingstdout.String())

			} else {
				t.Errorf("No output from ping command")
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Resources().Delete(ctx, pod); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()
	testEnv.Test(t, PublicpodFeature)
}
