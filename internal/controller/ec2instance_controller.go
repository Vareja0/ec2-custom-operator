/*
Copyright 2026.

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

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "github.com/vareja0/operator-repo/api/v1"
)

// EC2instanceReconciler reconciles a EC2instance object
type EC2instanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EC2instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *EC2instanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// TODO(user): your logic here
	ec2Instance := &computev1.EC2instance{}

	if err := r.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("Instance not found, likely deleted")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !ec2Instance.DeletionTimestamp.IsZero() {
		l.Info("Instance marked for deletion, terminating EC2 instance", "instanceID", ec2Instance.Status.InstanceID)

		if ec2Instance.Status.InstanceID != "" {
			if err := deleteEc2Instance(ctx, ec2Instance.Spec.Region, ec2Instance.Status.InstanceID); err != nil {
				l.Error(err, "Failed to terminate EC2 instance", "instanceID", ec2Instance.Status.InstanceID)
				return ctrl.Result{Requeue: true}, err
			}
			l.Info("EC2 instance terminated", "instanceID", ec2Instance.Status.InstanceID)
		}

		controllerutil.RemoveFinalizer(ec2Instance, "ec2instance.compute.cloud.com")
		if err := r.Update(ctx, ec2Instance); err != nil {
			l.Error(err, "Failed to remove finalizer")
			return ctrl.Result{Requeue: true}, err
		}

		l.Info("Finalizer removed, object will be garbage collected")
		return ctrl.Result{}, nil
	}

	if ec2Instance.Status.InstanceID != "" {
		live, err := describeLiveInstance(ctx, ec2Instance.Spec.Region, ec2Instance.Status.InstanceID)
		if err != nil {
			l.Error(err, "Failed to describe live instance", "instanceID", ec2Instance.Status.InstanceID)
			return ctrl.Result{Requeue: true}, err
		}

		if specChanged(live, &ec2Instance.Spec) {
			l.Info("Spec change detected, replacing instance", "instanceID", ec2Instance.Status.InstanceID)

			if err := deleteEc2Instance(ctx, ec2Instance.Spec.Region, ec2Instance.Status.InstanceID); err != nil {
				l.Error(err, "Failed to terminate outdated instance", "instanceID", ec2Instance.Status.InstanceID)
				return ctrl.Result{Requeue: true}, err
			}
			l.Info("Outdated instance terminated", "instanceID", ec2Instance.Status.InstanceID)

			ec2Instance.Status.InstanceID = ""
			ec2Instance.Status.PublicIP = ""
			ec2Instance.Status.PrivateIP = ""
			ec2Instance.Status.PublicDNS = ""
			ec2Instance.Status.PrivateDNS = ""
			ec2Instance.Status.Status = "replacing"
			if err := r.Status().Update(ctx, ec2Instance); err != nil {
				l.Error(err, "Failed to clear instance status before replacement")
				return ctrl.Result{Requeue: true}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}

		if live.State != "running" {
			l.Info("Instance is not running, updating status", "instanceID", ec2Instance.Status.InstanceID, "state", live.State)
			ec2Instance.Status.Status = live.State
			if err := r.Status().Update(ctx, ec2Instance); err != nil {
				l.Error(err, "Failed to update EC2instance status")
				return ctrl.Result{Requeue: true}, err
			}
		} else {
			l.Info("Instance is running and spec is up to date", "instanceID", ec2Instance.Status.InstanceID)
		}

		return ctrl.Result{}, nil
	}

	l.Info("Creating new instance")

	if !controllerutil.ContainsFinalizer(ec2Instance, "ec2instance.compute.cloud.com") {
		l.Info("Adding finalizer")
		controllerutil.AddFinalizer(ec2Instance, "ec2instance.compute.cloud.com")
		if err := r.Update(ctx, ec2Instance); err != nil {
			l.Error(err, "Failed to add finalizer")
			return ctrl.Result{Requeue: true}, err
		}
	}

	instanceInfo, err := createEc2Instance(ctx, ec2Instance)
	if err != nil {
		l.Error(err, "Failed to create EC2 instance")
		return ctrl.Result{Requeue: true}, err
	}

	ec2Instance.Status.InstanceID = instanceInfo.InstanceID
	ec2Instance.Status.PublicIP = instanceInfo.PublicIP
	ec2Instance.Status.PrivateIP = instanceInfo.PrivateIP
	ec2Instance.Status.PublicDNS = instanceInfo.PublicDNS
	ec2Instance.Status.PrivateDNS = instanceInfo.PrivateDNS
	ec2Instance.Status.Status = instanceInfo.State

	if err := r.Status().Update(ctx, ec2Instance); err != nil {
		l.Error(err, "Failed to update EC2instance status")
		return ctrl.Result{Requeue: true}, err
	}

	l.Info("EC2 instance status updated", "instanceID", instanceInfo.InstanceID, "publicIP", instanceInfo.PublicIP)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2instanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.EC2instance{}).
		Named("ec2instance").
		Complete(r)
}
