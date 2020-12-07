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
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
const iamRoleFinalizer = "iam.iclinic.com.br/finalizer"

// +kubebuilder:rbac:groups=iam.iclinic.com.br,resources=iamroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.iclinic.com.br,resources=iamroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update

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
			if err := r.DeleteRole(iamRole); err != nil {
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
	if err := r.CreateRole(ctx, iamRole); err != nil {
		log.Error(err, "Failed to create IamRole")
		return ctrl.Result{}, err
	}

	// Create or update service account
	if err := r.CreateOrUpdateServiceAccount(ctx, log, iamRole); err != nil {
		log.Error(err, "Error to update ServiceAccount")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// CreateOrUpdateServiceAccount creates or updates service account
func (r *IamRoleReconciler) CreateOrUpdateServiceAccount(ctx context.Context, log logr.Logger, iamRole *iamv1alpha1.IamRole) error {

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      iamRole.Spec.ServiceAccount,
			Namespace: iamRole.Namespace,
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": iamRole.Status.Arn,
			},
		},
	}

	found := &corev1.ServiceAccount{}
	findMe := types.NamespacedName{
		Name:      iamRole.Spec.ServiceAccount,
		Namespace: iamRole.Namespace,
	}

	if err := r.Get(ctx, findMe, found); err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, sa); err != nil {
			log.Error(err, "Failed to create ServiceAccount")
			return err
		}
	}

	if !reflect.DeepEqual(sa.Annotations, found.Annotations) {
		found.Annotations = sa.Annotations
		log.Info("Updating service account")
		if err := r.Update(ctx, found); err != nil {
			return err
		}
	}

	return nil
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
