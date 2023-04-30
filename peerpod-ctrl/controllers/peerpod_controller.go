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
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	confidentialcontainersorgv1alpha1 "github.com/confidential-containers/cloud-api-adaptor/peerpod-ctrl/api/v1alpha1"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud/cloudmgr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// PeerPodReconciler reconciles a PeerPod object
type PeerPodReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Provider cloud.Provider
}

const (
	ppFinalizer = "peer.pod/finalizer"
	ppConfigMap = "peer-pods-cm"
	ppSecret    = "peer-pods-secret"
)

//+kubebuilder:rbac:groups="",resourceNames=peer-pods-cm;peer-pods-secret,resources=configmaps;secrets,verbs=get

//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods/finalizers,verbs=update

func (r *PeerPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	pp := confidentialcontainersorgv1alpha1.PeerPod{}

	// cloud provider was not set, try to fetch cloud provider and its configs dynamically from ConfigMap or Secret
	// make sure the matching RBAC rules are set
	if r.Provider == nil {
		if err := r.cloudConfigsGetter(); err != nil {
			// don't requeue, if cloud configs are missing it will requeue later
			logger.Info("connot fetch cloud configs at the moment", "error", err)
		}

		var pErr error
		r.Provider, pErr = SetProvider()
		if pErr != nil {
			return ctrl.Result{}, pErr
		}
	}

	if err := r.Get(ctx, req.NamespacedName, &pp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if pp.ObjectMeta.GetDeletionTimestamp() == nil { // TODO: consider filter events
		return ctrl.Result{}, nil
	}

	if controllerutil.ContainsFinalizer(&pp, ppFinalizer) {
		logger.Info("deleting instance", "InstanceID", pp.Spec.InstanceID, "CloudProvider", pp.Spec.CloudProvider)
		if err := r.Provider.DeleteInstance(ctx, pp.Spec.InstanceID); err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(&pp, ppFinalizer)
		if err := r.Update(ctx, &pp); err != nil {
			if !apierrors.IsNotFound(err) { // object exist but fail to update, try again
				return ctrl.Result{}, err
			}
		}

		logger.Info("instance deleted", "InstanceID", pp.Spec.InstanceID, "CloudProvider", pp.Spec.CloudProvider)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&confidentialcontainersorgv1alpha1.PeerPod{}).
		Complete(r)
}

func (r *PeerPodReconciler) cloudConfigsGetter() error {
	peerpodscm := corev1.ConfigMap{}
	peerpodssecret := corev1.Secret{}
	ns := os.Getenv("PEERPODS_NAMESPACE")
	if ns == "" {
		return fmt.Errorf("PEERPODS_NAMESPACE is not set")
	}

	var cmErr error
	if cmErr = r.Client.Get(context.TODO(), types.NamespacedName{Name: ppConfigMap, Namespace: ns}, &peerpodscm); cmErr == nil {
		// set all configs as env vars to make sure all the required vars for auth are set
		for k, v := range peerpodscm.Data {
			os.Setenv(k, v)
		}
	}

	var secretErr error
	if secretErr = r.Client.Get(context.TODO(), types.NamespacedName{Name: ppSecret, Namespace: ns}, &peerpodssecret); secretErr == nil {
		for k, v := range peerpodssecret.Data {
			os.Setenv(k, string(v))
		}
	}

	if peerpodscm.Data == nil && peerpodssecret.Data == nil {
		return fmt.Errorf("ConfigMap Error: %v, Secret Error: %v", cmErr, secretErr)
	}

	return nil
}

func SetProvider() (cloud.Provider, error) {
	cloudName := os.Getenv("CLOUD_PROVIDER")
	if cloud := cloudmgr.Get(cloudName); cloud != nil {
		cloud.LoadEnv() // we assume LoadEnv knows to load all necessary configs
		provider, err := cloud.NewProvider()
		if err != nil {
			return nil, err
		}
		return provider, nil
	}

	return nil, fmt.Errorf("cloudmgr: %s cloud provider not supported", cloudName)
}
