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
	// steps to do always regardles of why we exit the reconciliation
	postSteps := []Step[T, R]{
		SaveInstance[T, R]{},
	}

	return func() (ctrl.Result, error) {
		r.GetLog().Info("Reconciling")
		result := handleReq[T, R](r, steps, postSteps)
		r.GetLog().Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

// handleReq implements a single Reconcile run by going throught each
// reconciliation steps provided.
func handleReq[T client.Object, R Req[T]](
	r R,
	steps []Step[T, R],
	postSteps []Step[T, R],
) Result {
	var result Result

	err := r.GetClient().Get(r.GetCtx(), r.GetRequest().NamespacedName, r.GetInstance())
	if err != nil {
		r.GetLog().Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.OK()
	}

	if !r.GetInstance().GetDeletionTimestamp().IsZero() {
		// TODO(gibi): create a delete path for cleanup
		r.GetLog().Info("Deleting instance")
	} else {
		// Normal reconciliation
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
	}

	for _, step := range postSteps {
		// We don't want to override the steps result unless the post step
		// result is signalling a negative result
		postResult := step.Do(r)
		if postResult.err != nil {
			r.GetLog().Error(result.err, fmt.Sprintf("PostStep: %s: failed.", step.GetName()))
			result = postResult
			// run the rest of the post steps
		}
		if postResult.Requeue {
			r.GetLog().Info(fmt.Sprintf("PostStep: %s: requested requeue. This should not happen. Ignored", step.GetName()))
			result = postResult
			// run the rest of the post steps
		}
		r.GetLog().Info(fmt.Sprintf("PostStep: %s: OK", step.GetName()))
	}

	return result
}
