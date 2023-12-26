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
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1 "github.com/gibizer/okofw/api/v1beta1"
	"github.com/gibizer/okofw/pkg/reconcile"
	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// SimpleReconciler reconciles a Simple object
type SimpleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type SimpleRReq struct {
	reconcile.DefaultReq[*v1beta1.Simple]
}

//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=simples/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SimpleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {

	rReq := &SimpleRReq{
		DefaultReq: reconcile.DefaultReq[*v1beta1.Simple]{
			Ctx:      ctx,
			Request:  req,
			Log:      log.FromContext(ctx),
			Client:   r.Client,
			Instance: &v1beta1.Simple{},
		},
	}

	return reconcile.NewReqHandler[*v1beta1.Simple, *SimpleRReq]().
		WithSteps(
			// TODO(gibi): check why InitCondition needs an extra pointer
			// but RecalculateReadyCondition does not.
			&reconcile.InitConditions[*v1beta1.Simple, *SimpleRReq]{},
			EnsureNonZeroDivisor{},
			Divide{},
		).
		WithPostSteps(
			reconcile.RecalculateReadyCondition[*v1beta1.Simple, *SimpleRReq]{},
		).
		Handle(rReq)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Simple{}).
		Complete(r)
}

type EnsureNonZeroDivisor struct {
	reconcile.BaseStep[*v1beta1.Simple, *SimpleRReq]
}

func (s EnsureNonZeroDivisor) GetName() string {
	return "EnsureNonZeroDivisor"
}

func (s EnsureNonZeroDivisor) GetManagedConditions() condition.Conditions {
	return []condition.Condition{
		*condition.UnknownCondition(
			condition.InputReadyCondition,
			condition.InitReason,
			condition.InputReadyInitMessage,
		),
	}
}

func (s EnsureNonZeroDivisor) Do(r *SimpleRReq, log logr.Logger) reconcile.Result {
	if r.GetInstance().Spec.Divisor == 0 {
		err := fmt.Errorf("division by zero")
		r.GetInstance().Status.Conditions.MarkFalse(
			condition.InputReadyCondition, condition.ErrorReason, condition.SeverityError, err.Error())
		return r.Error(err, log)
	}
	return r.OK()
}

type Divide struct {
	reconcile.BaseStep[*v1beta1.Simple, *SimpleRReq]
}

func (s Divide) GetName() string {
	return "Divide"
}

func (s Divide) GetManagedConditions() condition.Conditions {
	return []condition.Condition{
		*condition.UnknownCondition(
			condition.InputReadyCondition,
			condition.InitReason,
			condition.InputReadyInitMessage,
		),
	}
}

func (s Divide) Do(r *SimpleRReq, log logr.Logger) reconcile.Result {
	instance := r.GetInstance()
	quotient := instance.Spec.Dividend / instance.Spec.Divisor
	remainder := instance.Spec.Dividend % instance.Spec.Divisor
	instance.Status.Quotient = &quotient
	instance.Status.Remainder = &remainder
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, "calculation done")
	return r.OK()
}
