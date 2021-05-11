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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/api/errors"

	awss3 "github.com/aws/aws-sdk-go/service/s3"
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
	// add finalizer
	if len(s3Object.GetFinalizers()) < 1 {
		s3Object.SetFinalizers([]string{"systek.no/finalizer"})
		err = r.Client.Update(ctx, s3Object)
		if err != nil {
			return ctrl.Result{
				Requeue: true,
			}, err
		}
	}
	// handle delete
	if !s3Object.DeletionTimestamp.IsZero() {
		_, derr := r.S3Client.DeleteBucket(s3Object.Spec.BucketName)
		if derr != nil {
			//TODO: Change
			return ctrl.Result{}, derr
		}
		s3Object.SetFinalizers(remove(s3Object.GetFinalizers(), "systek.no/finalizer"))

		uerr := r.Client.Update(ctx, s3Object)
		if uerr != nil {
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{}, nil
	}

	//Check for updates
	if hasChanged(s3Object) {
		_, derr := r.S3Client.DeleteBucket(s3Object.Status.BucketName)
		if derr != nil {
			return ctrl.Result{}, derr
		}
		//Update status and requeue
		s3Object.Status = systeknov1.S3Status{}
		uerr := r.Client.Status().Update(ctx, s3Object)
		if uerr != nil {
			return ctrl.Result{}, uerr
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// Check if status is Success
	if s3Object.Status.Status == "Success" {
		return ctrl.Result{}, nil
	}

	//Handle common errors
	resp, err := r.S3Client.CreateBucket(s3Object.Spec.BucketName)
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case awss3.ErrCodeBucketAlreadyExists:
			return ctrl.Result{}, err
		case awss3.ErrCodeBucketAlreadyOwnedByYou:
			return ctrl.Result{}, nil
		default:
			r.Log.Error(aerr, "CreateBucket failed")
		}
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	s3Object.Status.Status = "Success"
	s3Object.Status.Location = *resp.Location
	err = r.Client.Status().Update(ctx, s3Object)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.Log.Info("Got s3 event ", "Name", req.Name, "Namespace", req.Name)

	r.Log.Info("BucketName new Bucket", "Value", resp)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *S3Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&systeknov1.S3{}).
		//Prevent reconcile on status changes
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func hasChanged(s3Object *systeknov1.S3) bool {
	return s3Object.Status.BucketName != s3Object.Spec.BucketName
}

func remove(strings []string, removeString string) []string {
	ret := make([]string, 0, len(strings))
	for _, s := range strings {
		if s != removeString {
			ret = append(ret, s)
		}
	}
	return ret
}
