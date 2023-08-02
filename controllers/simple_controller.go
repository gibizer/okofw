/*
Copyright 2022.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/gibizer/okofw/api/v1beta1"
	"github.com/gibizer/okofw/pkg/reconcile"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// SimpleReconciler reconciles a Simple object
type SimpleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SimpleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	return reconcile.NewReqHandler(
		ctx, req, r.Client, &v1beta1.Simple{},
		[]reconcile.Step[*v1beta1.Simple, reconcile.Req[*v1beta1.Simple]]{
			InitStatus{},
			EnsureNonZeroDivisor{},
			Divide{},
		},
	)()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Simple{}).
		Complete(r)
}

type InitStatus struct{}

func (s InitStatus) GetName() string {
	return "Init status"
}

func (s InitStatus) Do(r *reconcile.Req[*v1beta1.Simple]) reconcile.Result {
	r.Instance.Status.Conditions.Init(&condition.Conditions{})
	return r.OK()
}

type EnsureNonZeroDivisor struct{}

func (s EnsureNonZeroDivisor) GetName() string {
	return "Ensure non-zereo divisor"
}
func (s EnsureNonZeroDivisor) Do(r *reconcile.Req[*v1beta1.Simple]) reconcile.Result {
	if r.Instance.Spec.Divisor == 0 {
		r.Instance.Status.Conditions.MarkFalse(condition.ReadyCondition, condition.ErrorReason, condition.SeverityError, "division by zero")
		return r.Error(fmt.Errorf("division by zero"))
	}
	return r.OK()
}

type Divide struct{}

func (s Divide) GetName() string {
	return "Divide"
}
func (s Divide) Do(r *reconcile.Req[*v1beta1.Simple]) reconcile.Result {
	quotient := r.Instance.Spec.Divident / r.Instance.Spec.Divisor
	remainder := r.Instance.Spec.Divident % r.Instance.Spec.Divisor
	r.Instance.Status.Quotient = &quotient
	r.Instance.Status.Remainder = &remainder
	r.Instance.Status.Conditions.MarkTrue(condition.ReadyCondition, "calculation done")
	return r.OK()
}
