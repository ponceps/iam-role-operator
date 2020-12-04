/*


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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	iamv1alpha1 "github.com/iclinic/iam-role-operator/api/v1alpha1"
)

// IamRoleReconciler reconciles a IamRole object
type IamRoleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Finalizer for our objects
const iamRoleFinalizer = "iam.ponceps.com/finalizer"

// +kubebuilder:rbac:groups=iam.iclinic.com.br,resources=iamroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.iclinic.com.br,resources=iamroles/status,verbs=get;update;patch

func (r *IamRoleReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("iamrole", req.NamespacedName)

	log.Info("Reconciling IamRole")

	// Fetch the IamRole instance
	iamRole := &iamv1alpha1.IamRole{}

	if err := r.Get(ctx, req.NamespacedName, iamRole); err != nil {
		if errors.IsNotFound(err) {
			log.Info("IamRole resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get IamRole resource.")
		return ctrl.Result{}, err
	}

	// Check if the CR is marked to be deleted
	if iamRole.GetDeletionTimestamp() != nil {
		log.Info("IamRole marked for deletion. Running finalizers.")
		if contains(iamRole.GetFinalizers(), iamRoleFinalizer) {
			// Firt delete role
			if err := DeleteRole(ctx, iamRole.ObjectMeta.Name); err != nil {
				return ctrl.Result{}, err
			}
			// Remove finalizer once the finalizer logic has run
			controllerutil.RemoveFinalizer(iamRole, iamRoleFinalizer)
			if err := r.Update(ctx, iamRole); err != nil {
				// If the object update fails, requeue
				return ctrl.Result{}, err
			}
		}
		log.Info("IamRole can be deleted now.")
		return ctrl.Result{}, nil
	}

	// Add Finalizers to the CR
	if !contains(iamRole.GetFinalizers(), iamRoleFinalizer) {
		controllerutil.AddFinalizer(iamRole, iamRoleFinalizer)
		if err := r.Update(ctx, iamRole); err != nil {
			log.Error(err, "Failed to update IamRole with finalizer")
			return ctrl.Result{}, err
		}
	}

	// Create role if not exists
	if err := CreateRole(ctx, iamRole); err != nil {
		log.Error(err, "Failed to create IamRole")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *IamRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.IamRole{}).
		Complete(r)
}

// contains returns true if a string is found on a slice
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
