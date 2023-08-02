package reconcile

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Req represents a single Reconcile request
type Req[T client.Object] struct {
	Ctx      context.Context
	Log      logr.Logger
	Request  ctrl.Request
	Client   client.Client
	Instance T
}

type Step[T client.Object, R Req[T]] interface {
	GetName() string
	Do(r *R) Result
}

type Handler func() (ctrl.Result, error)

func NewReqHandler[T client.Object](
	ctx context.Context, req ctrl.Request, client client.Client, prototype T,
	steps []Step[T, Req[T]],
) Handler {
	r := &Req[T]{
		Ctx:      ctx,
		Log:      log.FromContext(ctx),
		Request:  req,
		Client:   client,
		Instance: prototype,
	}
	// steps that run before any real reconciliation step and stop reconciling
	// if they fail.
	preSteps := []Step[T, Req[T]]{
		ReadInstance[T, Req[T]]{},
		HandleDeleted[T, Req[T]]{},
	}
	// steps to do always regardles of why we exit the reconciliation
	finallySteps := []Step[T, Req[T]]{
		SaveInstance[T, Req[T]]{},
	}

	return func() (ctrl.Result, error) {
		r.Log.Info("Reconciling")
		result := r.handle(preSteps, steps, finallySteps)
		r.Log.Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

func (r *Req[T]) handle(preSteps []Step[T, Req[T]], steps []Step[T, Req[T]], postSteps []Step[T, Req[T]]) Result {
	var result Result

	for _, step := range preSteps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("PreStep: %s: failed. Return immediately", step.GetName()))
			// return, skip final steps
			return result
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("PreStep: %s: requested requeue. Return immediately", step.GetName()))
			// return, skip final steps
			return result
		}
		r.Log.Info(fmt.Sprintf("PreStep: %s: OK", step.GetName()))
	}

	for _, step := range steps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("Step: %s: failed.", step.GetName()))
			// jump to final steps
			break
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("Step: %s: requested requeue.", step.GetName()))
			// jump to final steps
			break
		}
		r.Log.Info(fmt.Sprintf("Step: %s: OK", step.GetName()))
	}

	for _, step := range postSteps {
		result = step.Do(r)
		if result.err != nil {
			r.Log.Error(result.err, fmt.Sprintf("PostStep: %s: failed.", step.GetName()))
			// run the rest of the post steps
		}
		if result.Requeue {
			r.Log.Info(fmt.Sprintf("PostStep: %s: requested requeue. This should not happen. Ignored", step.GetName()))
			// run the rest of the post steps
		}
		r.Log.Info(fmt.Sprintf("PostStep: %s: OK", step.GetName()))
	}

	return result
}
