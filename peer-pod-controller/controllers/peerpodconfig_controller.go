/*
Copyright Confidential Containers Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"os"
	"path"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"

	ccv1alpha1 "github.com/confidential-containers/cloud-api-adaptor/peer-pod-controller/api/v1alpha1"
)

// PeerPodConfigReconciler reconciles a PeerPodConfig object
type PeerPodConfigReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	peerPodConfig *ccv1alpha1.PeerPodConfig
}

var ppCloudProviderName string
var validCloudProviderNames = []string{"aws", "libvirt", "ibmcloud", "vsphere", "azure"}

//Adding sideEffects=none as a workaround for https://github.com/kubernetes-sigs/kubebuilder/issues/1917
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpodconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpodconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpodconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes/status,verbs=patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=create;get;update;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;update;list;watch
//+kubebuilder:rbac:groups="";machineconfiguration.openshift.io,resources=nodes;machineconfigs;machineconfigpools;containerruntimeconfigs;pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PeerPodConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *PeerPodConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	_ = r.Log.WithValues("peerpod-controller", req.NamespacedName)
	r.Log.Info("Reconciling PeerPodConfig in Kubernetes Cluster")

	// Fetch the CcRuntime instance
	r.peerPodConfig = &ccv1alpha1.PeerPodConfig{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, r.peerPodConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if err := r.peerpodCloudProviderIsValid(); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.advertiseExtendedResources(); err != nil {
		return ctrl.Result{}, err
	}

	// Create the cloud-api-adapter DaemonSet
	ds := r.createCcaDaemonset(ppCloudProviderName)
	if err := controllerutil.SetControllerReference(r.peerPodConfig, ds, r.Scheme); err != nil {
		r.Log.Error(err, "Failed setting ControllerReference for cloud-api-adaptor DS")
		return ctrl.Result{}, err
	}
	foundDs := &appsv1.DaemonSet{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, foundDs)
	if err != nil && k8serrors.IsNotFound(err) {
		r.Log.Info("Creating cloud-api-adapter daemonset", "ds.Namespace", ds.Namespace, "ds.Name", ds.Name)
		err = r.Client.Create(context.TODO(), ds)
		if err != nil {
			r.Log.Error(err, "failed to create cloud-api-adaptor")
			return ctrl.Result{}, err
		}
	}

	r.Log.Info("Reconciling PeerPodConfig")

	return ctrl.Result{}, nil
}

// Check if "peer-pods-secret" exists and has valid cloud provider name set in CLOUD_PROVIDER
func (r *PeerPodConfigReconciler) peerpodCloudProviderIsValid() error {
	peerpodscm := corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "peer-pods-cm", Namespace: os.Getenv("PEERPODS_NAMESPACE")}, &peerpodscm)
	if err != nil && k8serrors.IsNotFound(err) {
		return err
	} else if err != nil {
		// Error reading the object - requeue the request.
		return err
	} else {
		ppCloudProviderName = peerpodscm.Data["CLOUD_PROVIDER"]
		if ppCloudProviderName == "" || !contains(validCloudProviderNames, ppCloudProviderName) {
			r.Log.Info("CLOUD_PROVIDER=", "ppCloudProviderName", ppCloudProviderName)
			return errors.New("set CLOUD_PROVIDER name to one supported in cloud-api-provider, found CLOUD_PROVIDER=" + ppCloudProviderName)
		}
	}
	return nil
}

func MountProgagationRef(mode corev1.MountPropagationMode) *corev1.MountPropagationMode {
	return &mode
}

/*
  cloudProviderName needs to be verified against validCloudProviders by caller
*/
func (r *PeerPodConfigReconciler) createCcaDaemonset(cloudProviderName string) *appsv1.DaemonSet {
	var (
		runPrivileged           = true
		runAsUser         int64 = 0
		defaultMode       int32 = 0600
		sshSecretOptional       = true
	)

	dsName := "peer-pod-controller-cca-daemon"
	dsLabelSelectors := map[string]string{
		"name": dsName,
	}
	nodeSelector := map[string]string{
		"node-role.kubernetes.io/worker": "",
	}
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: os.Getenv("PEERPODS_NAMESPACE"),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: dsLabelSelectors,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dsLabelSelectors,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					NodeSelector:       nodeSelector,
					HostNetwork:        true,
					Containers: []corev1.Container{
						{
							Name: "cc-runtime-install-pod",
							// TODO make configurable via env var
							Image:           "quay.io/confidential-containers/cloud-api-adaptor-" + cloudProviderName + ":latest",
							ImagePullPolicy: "Always",
							SecurityContext: &corev1.SecurityContext{
								// TODO - do we really need to run as root?
								Privileged: &runPrivileged,
								RunAsUser:  &runAsUser,
							},
							Command: []string{"/usr/local/bin/entrypoint.sh"},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: r.peerPodConfig.Spec.CloudSecretName,
										},
									},
								},
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: r.peerPodConfig.Spec.ConfigMapName,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ssh",
									MountPath: "/root/.ssh",
									ReadOnly:  true,
								},
								{
									MountPath: "/run/peerpod",
									Name:      "pods-dir",
								},
								{
									MountPath:        "/run/netns",
									MountPropagation: MountProgagationRef(corev1.MountPropagationHostToContainer),
									Name:             "netns",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "ssh",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "ssh-key-secret",
									DefaultMode: &defaultMode,
									Optional:    &sshSecretOptional,
								},
							},
						},
						{
							Name: "pods-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/peerpod",
								},
							},
						},
						{
							Name: "netns",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/netns",
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerPodConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ccv1alpha1.PeerPodConfig{}).
		Complete(r)
}

func (r *PeerPodConfigReconciler) getNodesWithLabels(nodeLabels map[string]string) (error, *corev1.NodeList) {
	nodes := &corev1.NodeList{}
	labelSelector := labels.SelectorFromSet(nodeLabels)
	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: labelSelector},
	}

	if err := r.Client.List(context.TODO(), nodes, listOpts...); err != nil {
		r.Log.Error(err, "Getting list of nodes having specified labels failed")
		return err, &corev1.NodeList{}
	}
	return nil, nodes
}

func (r *PeerPodConfigReconciler) advertiseExtendedResources() error {

	r.Log.Info("set up extended resources")
	err, nodesList := r.getNodesWithLabels(map[string]string{"node-role.kubernetes.io/worker": ""})
	if err != nil {
		r.Log.Info("getting node list failed when trying to update nodes with extended resources")
		return err
	}

	// FIXME distribute remainder among nodes
	var limitInt int64
	limitInt, err = strconv.ParseInt(r.peerPodConfig.Spec.Limit, 0, 64)
	if err != nil {
		r.Log.Error(err, "spec.Limit in PeerPodConfig must be an integer")
	}

	limitPerNode := limitInt / int64(len(nodesList.Items))

	for _, node := range nodesList.Items {
		patches := append([]JsonPatch{}, NewJsonPatch("add", "/status/capacity", "kata.peerpods.io~1vm",
			strconv.Itoa(int(limitPerNode))))
		cli, err := r.GetClient()
		if err != nil {
			r.Log.Error(err, "failed to get k8s client")
		}
		err = r.PatchNodeStatus(cli, node.Name, patches)
		if err != nil {
			r.Log.Error(err, "Failed to set extended resource for node", "node name", node.Name)
		}
	}
	return nil
}

func (r *PeerPodConfigReconciler) PatchNodeStatus(c *k8sclient.Clientset, nodeName string, patches []JsonPatch) error {
	if len(patches) > 0 {
		data, err := json.Marshal(patches)
		if err == nil {
			_, err = c.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, data, metav1.PatchOptions{}, "status")
		}
		return err
	}
	r.Log.Info("empty patch for node, no change")
	return nil
}

// JsonPatch is a json marshaling helper used for patching API objects
type JsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

// NewJsonPatch returns a new JsonPatch object
func NewJsonPatch(verb string, jsonpath string, key string, value string) JsonPatch {
	return JsonPatch{verb, path.Join(jsonpath, strings.ReplaceAll(key, "/", "~1")), value}
}

// GetClient creates and returns a new clientset from given config
func (r PeerPodConfigReconciler) GetClient() (*k8sclient.Clientset, error) {
	Kubeconfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := k8sclient.NewForConfig(Kubeconfig)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
