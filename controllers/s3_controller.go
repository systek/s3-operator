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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	systeknov1 "github.com/systek/s3-operator/api/v1"
	"github.com/systek/s3-operator/s3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// S3Reconciler reconciles a S3 object
type S3Reconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	S3Client s3.Client
}

// +kubebuilder:rbac:groups=systek.no,resources=s3s,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=systek.no,resources=s3s/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=systek.no,resources=s3s/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the S3 object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *S3Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("s3", req.NamespacedName)

	s3Object := &systeknov1.S3{}

	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, s3Object)

	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("S3 resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		//Requeue the request
		return ctrl.Result{}, err
	}

	resp, err := r.S3Client.CreateBucket(s3Object.Spec.BucketName)

	if err != nil {
		return ctrl.Result{}, err
	}

	s3Object.Status.Accepted = "OK"
	err = r.Client.Status().Update(ctx, s3Object)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.Log.Info("Got s3 event ", "Name", req.Name, "Namespace", req.Name)
	r.Log.Info("Created new Bucket", "Value", resp)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *S3Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&systeknov1.S3{}).
		Complete(r)
}
