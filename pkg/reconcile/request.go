package reconcile

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Req holds a single reconcile request
// T is the CRD type the reconcile request running on
type Req[T client.Object] interface {
	GetCtx() context.Context
	GetLog() logr.Logger
	GetRequest() ctrl.Request
	GetClient() client.Client
	GetInstance() T

	OK() Result
	Error(error) Result
	Requeue(after *time.Duration) Result
}

// ReqBase provides the minimal implementation of a reconcile request
type ReqBase[T client.Object] struct {
	Ctx      context.Context
	Log      logr.Logger
	Request  ctrl.Request
	Client   client.Client
	Instance T
}

func (r *ReqBase[T]) GetCtx() context.Context {
	return r.Ctx
}

func (r *ReqBase[T]) GetLog() logr.Logger {
	return r.Log
}

func (r *ReqBase[T]) GetRequest() ctrl.Request {
	return r.Request
}

func (r *ReqBase[T]) GetClient() client.Client {
	return r.Client
}

func (r *ReqBase[T]) GetInstance() T {
	return r.Instance
}

type Handler func() (ctrl.Result, error)

// NewReqHandler builds up a function that can handle the current reconcile
// request for CRD type T with reconcile request type R
func NewReqHandler[T client.Object, R Req[T]](
	r R, steps []Step[T, R],
) Handler {
	// steps that run before any real reconciliation step and stop reconciling
	// if they fail.
	preSteps := []Step[T, R]{
		ReadInstance[T, R]{},
		HandleDeleted[T, R]{},
	}
	// steps to do always regardles of why we exit the reconciliation
	postSteps := []Step[T, R]{
		SaveInstance[T, R]{},
	}

	return func() (ctrl.Result, error) {
		r.GetLog().Info("Reconciling")
		result := handleReq[T, R](r, preSteps, steps, postSteps)
		r.GetLog().Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

// handleReq implements a single Reconcile run by going throught each
// reconciliation steps provided.
//   - preSteps are special steps that if fail or requeue then postSteps are
//     skipped
//   - postSteps are always run even if a step fails or requeues. If a postStep
//     fails all the remaining post step still runs.
func handleReq[T client.Object, R Req[T]](
	r R,
	preSteps []Step[T, R],
	steps []Step[T, R],
	postSteps []Step[T, R],
) Result {
	var result Result

	for _, step := range preSteps {
		result = step.Do(r)
		if result.err != nil {
			r.GetLog().Error(result.err, fmt.Sprintf("PreStep: %s: failed. Return immediately", step.GetName()))
			// return, skip final steps
			return result
		}
		if result.Requeue {
			r.GetLog().Info(fmt.Sprintf("PreStep: %s: requested requeue. Return immediately", step.GetName()))
			// return, skip final steps
			return result
		}
		r.GetLog().Info(fmt.Sprintf("PreStep: %s: OK", step.GetName()))
	}

	for _, step := range steps {
		result = step.Do(r)
		if result.err != nil {
			r.GetLog().Error(result.err, fmt.Sprintf("Step: %s: failed.", step.GetName()))
			// jump to final steps
			break
		}
		if result.Requeue {
			r.GetLog().Info(fmt.Sprintf("Step: %s: requested requeue.", step.GetName()))
			// jump to final steps
			break
		}
		r.GetLog().Info(fmt.Sprintf("Step: %s: OK", step.GetName()))
	}

	for _, step := range postSteps {
		result = step.Do(r)
		if result.err != nil {
			r.GetLog().Error(result.err, fmt.Sprintf("PostStep: %s: failed.", step.GetName()))
			// run the rest of the post steps
		}
		if result.Requeue {
			r.GetLog().Info(fmt.Sprintf("PostStep: %s: requested requeue. This should not happen. Ignored", step.GetName()))
			// run the rest of the post steps
		}
		r.GetLog().Info(fmt.Sprintf("PostStep: %s: OK", step.GetName()))
	}

	return result
}
