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
	"reflect"
	"strconv"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"

	v1beta1 "github.com/gibizer/okofw/api/v1beta1"
	"github.com/gibizer/okofw/pkg/reconcile"
)

// RWExternalReconciler reconciles a RWExternal object
type RWExternalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type RWExternalRReq struct {
	reconcile.Req[*v1beta1.RWExternal]
	Divident *int
	Divisor  *int
}

//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RWExternal object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *RWExternalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rReq := &RWExternalRReq{
		Req: &reconcile.DefaultReq[*v1beta1.RWExternal]{
			Ctx:            ctx,
			Request:        req,
			Log:            log.FromContext(ctx),
			Client:         r.Client,
			Instance:       &v1beta1.RWExternal{},
			RequeueTimeout: time.Duration(1) * time.Second,
		},
		Divident: nil,
		Divisor:  nil,
	}
	steps := []reconcile.Step[*v1beta1.RWExternal, *RWExternalRReq]{
		InitRWExternalStatus{},
		EnsureInput{},
	}
	return reconcile.NewReqHandler(rReq, steps)()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RWExternalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.RWExternal{}).
		Complete(r)
}

type InitRWExternalStatus struct{}

func (s InitRWExternalStatus) GetName() string {
	return "Init status"
}

func (s InitRWExternalStatus) Do(r *RWExternalRReq) reconcile.Result {
	// TODO(gibi): generalize this to collect condition types from Steps to
	// initialize
	cl := condition.CreateList(
		condition.UnknownCondition(
			condition.InputReadyCondition,
			condition.InitReason,
			condition.InputReadyInitMessage,
		),
	)
	r.GetInstance().Status.Conditions.Init(&cl)
	return r.OK()
}

type EnsureInput struct{}

func (s EnsureInput) GetName() string {
	return "Ensure input is available"
}

func (s EnsureInput) Do(r *RWExternalRReq) reconcile.Result {
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Namespace: r.GetInstance().Namespace,
		Name:      r.GetInstance().Spec.InputSecret,
	}
	err := r.GetClient().Get(r.GetCtx(), secretName, secret)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.GetInstance().Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				"Missing input: secret/"+secretName.Name))
			return r.RequeueAfter("Waiting for input secret/"+secretName.Name, nil)
		}
		r.GetInstance().Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return r.Error(err)
	}

	expectedFields := []string{
		"divident",
		"divisor",
	}
	for _, field := range expectedFields {
		v, ok := secret.Data[field]
		if !ok {
			err := fmt.Errorf("field '%s' not found in secret/%s", field, secretName.Name)
			r.GetInstance().Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.InputReadyErrorMessage,
				err.Error()))
			return r.Error(err)
		}
		d, err := strconv.Atoi(string(v))
		if err != nil {
			err := fmt.Errorf("'%s' in secret/%s cannot be converted to int: %w", field, secretName.Name, err)
			r.GetInstance().Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.InputReadyErrorMessage,
				err.Error()))
			return r.Error(err)
		}
		f := reflect.ValueOf(r).Elem().FieldByName(cases.Title(language.English, cases.Compact).String(field))
		f.Set(reflect.ValueOf(&d))
	}

	r.GetInstance().Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	// TODO(gibi): Ensure that watch is added for Secrets

	return r.OK()
}
