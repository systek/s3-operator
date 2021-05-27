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
	"github.com/systek/s3-operator/iam"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"

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
	Log       logr.Logger
	Scheme    *runtime.Scheme
	S3Client  s3.Client
	IAMClient iam.Client
}

const (
	success = "Success"
	failed = "Failed"
)

// +kubebuilder:rbac:groups=systek.no,resources=s3s,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=systek.no,resources=s3s/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=systek.no,resources=s3s/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete;update;get

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
		s3Object.SetFinalizers([]string{"systek.no/finalizer","foregroundDeletion"})
		err = r.Client.Update(ctx, s3Object)
		if err != nil {
			return ctrl.Result{
				Requeue: true,
			}, err
		}
	}
	// handle delete
	if !s3Object.DeletionTimestamp.IsZero() {
		derr := r.S3Client.DeleteBucket(s3Object.Spec.BucketName)
		if derr != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awss3.ErrCodeNoSuchBucket == awsErr.Code() {
					r.Log.Error(awsErr,"No such bucket exists")
				}
			}else{
				r.Log.Error(derr, "Unexpected error")
			}
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
		derr := r.S3Client.DeleteBucket(s3Object.Status.BucketName)
		if derr != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awss3.ErrCodeNoSuchBucket == awsErr.Code() {
					r.Log.Error(awsErr,"No such bucket exists")
				}
			}else{
				r.Log.Error(derr, "Unexpected error")
			}
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
	if s3Object.Status.Status == success {
		return ctrl.Result{}, nil
	}

	//Handle common errors
	resp, err := r.S3Client.CreateBucket(s3Object.Spec.BucketName)
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case awss3.ErrCodeBucketAlreadyExists:
			// update s3 status to failed and end reconciler
			err := r.setFailedState(ctx, s3Object, "Bucket already exists globally")
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil

		case awss3.ErrCodeBucketAlreadyOwnedByYou:
			// update s3 status to failed and end reconciler
			err := r.setFailedState(ctx, s3Object, "Bucket already owned by account")
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil

		default:
			r.Log.Error(aerr, "CreateBucket failed")
			return ctrl.Result{}, nil
		}
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	//Create user.
	userName := strings.Join([]string{s3Object.Spec.BucketName, s3Object.Namespace},"-")
	user, err := r.IAMClient.CreateUser(userName)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Create accesskey with given user
	accessKeyOutput, err := r.IAMClient.CreateAccessKey(user)

	if err != nil {
		return ctrl.Result{}, err
	}

	//Create policy
	policyName := strings.Join([]string{s3Object.Name, s3Object.Namespace},"-")
	policy, err := r.IAMClient.CreateAndAttachPolicy(policyName, user, s3Object.Spec.BucketName)

	if err != nil {
		return ctrl.Result{}, err
	}
	blockOwnerDeletion := true
	err = r.Client.Create(ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{

			Name: s3Object.Name + "-aws-credentials",
			Namespace: s3Object.Namespace,
			Annotations: map[string]string{
				"systek.no/arnPolicy": *policy.Policy.Arn,
				"systek.no/iamUser": user,
			},
			Labels: map[string]string{
				"s3operator": "true",

			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "systek.no/v1",
					Kind: "S3",
					Name: s3Object.Name,
					UID: s3Object.UID,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		StringData:       map[string]string{"AWS_SECRET_KEY": *accessKeyOutput.AccessKey.SecretAccessKey,
											"AWS_ACCESS_KEY_ID": *accessKeyOutput.AccessKey.AccessKeyId},
	})

	if err != nil {
		return ctrl.Result{}, err
	}
	
	s3Object.Status.Status = success
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
	if s3Object.Status == (systeknov1.S3Status{}) {
		return false
	}
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

func (r *S3Reconciler) setFailedState(ctx context.Context, s3Object *systeknov1.S3, message string) error {
	r.Log.Info("Setting failed state")
	s3Object.Status.Message = message
	s3Object.Status.Status = failed
	err := r.Client.Status().Update(ctx, s3Object)
	if err != nil {
		return err
	}
	return nil
}