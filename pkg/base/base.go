package base

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ReconcileReq represents a single Reconcile request
type ReconcileReq[T client.Object] struct {
	Ctx      context.Context
	Log      logr.Logger
	Request  ctrl.Request
	Client   client.Client
	Instance T
}

type StepFunc[T client.Object, R ReconcileReq[T]] func(r *R) Result

type Step[T client.Object, R ReconcileReq[T]] struct {
	Name string
	Do   StepFunc[T, R]
}

type Handler func() (ctrl.Result, error)

func NewReconcileReqHandler[T client.Object](
	ctx context.Context, req ctrl.Request, client client.Client, prototype T,
	steps []Step[T, ReconcileReq[T]],
) Handler {
	r := &ReconcileReq[T]{
		Ctx:      ctx,
		Log:      log.FromContext(ctx),
		Request:  req,
		Client:   client,
		Instance: prototype,
	}
	// steps that run before any real reconciliation step and stop reconciling
	// if they fail.
	preSteps := []Step[T, ReconcileReq[T]]{
		{Name: "Read instance state", Do: readInstance[T]},
		{Name: "Handle instance delete", Do: handleDeleted[T]},
	}
	// steps to do always regardles of why we exit the reconciliation
	finallySteps := []Step[T, ReconcileReq[T]]{
		{Name: "Persist instance state", Do: saveInstance[T]},
	}

	return func() (ctrl.Result, error) {
		r.Log.Info("Reconciling")
		result := r.handle(preSteps, steps, finallySteps)
		r.Log.Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

func (r *ReconcileReq[T]) handle(preSteps []Step[T, ReconcileReq[T]], steps []Step[T, ReconcileReq[T]], postSteps []Step[T, ReconcileReq[T]]) Result {
	var result Result

	for _, step := range preSteps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("PreStep: %s: failed. Return immediately", step.Name))
			// return, skip final steps
			return result
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("PreStep: %s: requested requeue. Return immediately", step.Name))
			// return, skip final steps
			return result
		}
		r.Log.Info(fmt.Sprintf("PreStep: %s: OK", step.Name))
	}

	for _, step := range steps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("Step: %s: failed.", step.Name))
			// jump to final steps
			break
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("Step: %s: requested requeue.", step.Name))
			// jump to final steps
			break
		}
		r.Log.Info(fmt.Sprintf("Step: %s: OK", step.Name))
	}

	for _, step := range postSteps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("PostStep: %s: failed.", step.Name))
			// run the rest of the post steps
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("PostStep: %s: requested requeue. This should not happen. Ignored", step.Name))
			// run the rest of the post steps
		}
		r.Log.Info(fmt.Sprintf("PostStep: %s: OK", step.Name))
	}

	return result
}

func readInstance[T client.Object](r *ReconcileReq[T]) Result {
	err := r.Client.Get(r.Ctx, r.Request.NamespacedName, r.Instance)
	if err != nil {
		r.Log.Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
}

func handleDeleted[T client.Object](r *ReconcileReq[T]) Result {
	if !r.Instance.GetDeletionTimestamp().IsZero() {
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
}

func saveInstance[T client.Object](r *ReconcileReq[T]) Result {
	err := r.Client.Status().Update(r.Ctx, r.Instance)
	if err != nil {
		return r.Error(err)
	}
	return r.OK()
}
