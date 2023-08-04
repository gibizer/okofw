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

	"github.com/go-logr/logr"
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

var rwExternalSteps = []reconcile.Step[*v1beta1.RWExternal, *RWExternalRReq]{
	&reconcile.InitConditions[*v1beta1.RWExternal, *RWExternalRReq]{},
	EnsureInput{},
	DivideAndStore{},
}

//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=okofw-example.openstack.org,resources=rwexternals/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
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
	return reconcile.NewReqHandler(rReq, rwExternalSteps)()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RWExternalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.RWExternal{}).
		Complete(r)
}

type EnsureInput struct {
	reconcile.BaseStep[*v1beta1.RWExternal, *RWExternalRReq]
}

func (s EnsureInput) GetName() string {
	return "EnsureInput"
}

func (s EnsureInput) GetManagedConditions() condition.Conditions {
	return []condition.Condition{
		*condition.UnknownCondition(
			condition.InputReadyCondition,
			condition.InitReason,
			condition.InputReadyInitMessage,
		),
	}
}

func (s EnsureInput) Do(r *RWExternalRReq, log logr.Logger) reconcile.Result {
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
		err = fmt.Errorf("failed to read/secret/%s:%w", secretName.Name, err)
		r.GetInstance().Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return r.Error(err, log)
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
			return r.Error(err, log)
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
			return r.Error(err, log)
		}
		f := reflect.ValueOf(r).Elem().FieldByName(cases.Title(language.English, cases.Compact).String(field))
		f.Set(reflect.ValueOf(&d))
	}

	r.GetInstance().Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	// TODO(gibi): Ensure that watch is added for Secrets

	return r.OK()
}

type DivideAndStore struct {
	reconcile.BaseStep[*v1beta1.RWExternal, *RWExternalRReq]
}

func (s DivideAndStore) GetName() string {
	return "DivideAndStore"
}

func (s DivideAndStore) GetManagedConditions() condition.Conditions {
	return []condition.Condition{
		*condition.UnknownCondition(
			v1beta1.OutputReadyCondition,
			condition.InitReason,
			v1beta1.OutputReadyInitMessage,
		),
	}
}

func (s DivideAndStore) Do(r *RWExternalRReq, log logr.Logger) reconcile.Result {
	// TODO(gibi): implement storing the output in a Secret
	r.GetInstance().Status.Conditions.MarkTrue(v1beta1.OutputReadyCondition, v1beta1.OutputReadyReadyMessage)
	return r.OK()
}
