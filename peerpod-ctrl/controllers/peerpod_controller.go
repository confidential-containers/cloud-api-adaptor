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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	confidentialcontainersorgv1alpha1 "github.com/confidential-containers/cloud-api-adaptor/peerpod-ctrl/api/v1alpha1"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// PeerPodReconciler reconciles a PeerPod object
type PeerPodReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Provider cloud.Provider
}

var ppFinalizer string = "peer.pod/finalizer"

//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=confidentialcontainers.org,resources=peerpods/finalizers,verbs=update

func (r *PeerPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	pp := confidentialcontainersorgv1alpha1.PeerPod{}

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
