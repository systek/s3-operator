package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/systek/s3-operator/iam"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

/*
Copyright 2021.

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


// S3Reconciler reconciles a S3 object
type SecretsReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	IAMClient iam.Client
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete;update;get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the S3 object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *SecretsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("secrets", req.NamespacedName)

	secretObject := &v1.Secret{}

	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, secretObject)

	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Secret not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		//Requeue the request
		return ctrl.Result{}, err
	}
	// add finalizer
	if len(secretObject.GetFinalizers()) < 1 {
		secretObject.SetFinalizers([]string{"systek.no/finalizer"})
		err = r.Client.Update(ctx, secretObject)
		if err != nil {
			return ctrl.Result{
				Requeue: true,
			}, err
		}
	}
	// handle delete
	if !secretObject.DeletionTimestamp.IsZero() {
		if secretObject.Annotations == nil {
			r.Log.Info("Annotations not found")
			return ctrl.Result{}, nil
		}
		policyArn := secretObject.Annotations["systek.no/arnPolicy"]
		iamUser := secretObject.Annotations["systek.no/iamUser"]

		derr := r.IAMClient.DeletePolicy(policyArn,iamUser)

		if derr != nil {
			//TODO: Change
			return ctrl.Result{}, derr
		}
		// delete accesskey
		accessKeyId := secretObject.Data["AWS_ACCESS_KEY_ID"]

		derr = r.IAMClient.DeleteAccessKey(iamUser, string(accessKeyId))

		if derr != nil {
			//TODO: Change
			fmt.Println("Could not delete accesskey", derr)
			return ctrl.Result{}, derr
		}

		// delete user
		derr = r.IAMClient.DeleteUser(iamUser)

		if derr != nil {
			//TODO: Change
			return ctrl.Result{}, derr
		}

		secretObject.SetFinalizers(remove(secretObject.GetFinalizers(), "systek.no/finalizer"))

		uerr := r.Client.Update(ctx, secretObject)
		if uerr != nil {
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}


// SetupWithManager sets up the controller with the Manager.
func (r *SecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			labels := event.Object.GetLabels()
			if labels != nil {
				if _, ok := labels["s3operator"]; ok {
					return true
				}
			}
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true
		},
		UpdateFunc:  func(event event.UpdateEvent) bool {
			labels := event.ObjectNew.GetLabels()
			if labels != nil {
				if _, ok := labels["s3operator"]; ok {
					return true
				}
			}
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}).
		WithEventFilter(pred).
		Complete(r)
}


