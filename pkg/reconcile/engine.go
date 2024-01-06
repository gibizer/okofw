package reconcile

import (
	"fmt"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ReqHandlerBuilder helps building a ReqHandler.
// It is not intended for direct use. Use NewReqHandler() instead.
type ReqHandlerBuilder[T client.Object, R Req[T]] struct {
	steps []Step[T, R]
}

// NewReqHandler builds up a function that can handle the current reconcile
// request for CRD type T with reconcile request type R. It can be configured
// with the functions on the returned builder to add reconciliation steps.
//
// Step.Do() is called in the order of the Steps added to the handler when
// CR is reconciled normally.
// Step.Cleanup() called in the reverse order of the Steps added when the CR is
// being deleted.
// Step.Post() is called in order of the Steps added after all the Step's Do or
// Cleanup function is executed, or one of those functions returned error or
// requested requeue.
func NewReqHandler[T client.Object, R Req[T]]() *ReqHandlerBuilder[T, R] {
	return &ReqHandlerBuilder[T, R]{}
}

// WithSteps adds steps to handle the reconciliation of the instance T
func (builder *ReqHandlerBuilder[T, R]) WithSteps(steps ...Step[T, R]) *ReqHandlerBuilder[T, R] {
	builder.steps = append(builder.steps, steps...)
	return builder
}

// Handle builds the request handler for the request and executes defined steps
// to reconcile the request
func (builder *ReqHandlerBuilder[T, R]) Handle(request R) (ctrl.Result, error) {
	request.GetLog().Info("Reconciling")
	result := handleReq[T, R](request, builder.steps)
	request.GetLog().Info("Reconciled", "result", result)
	return result.Unwrap()
}

// handleReq implements a single Reconcile run by going through each
// reconciliation steps provided.
func handleReq[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {

	// NOTE(gibi): this is a bit wasteful as steps are static between reconcile
	// runs so this setup could be done only once at manager setup
	lateStepSetup(steps, r)

	// Read the instance
	readResult, found := readInstance[T, R](r)
	if !readResult.IsOK() {
		return readResult
	}
	if !found {
		// Instance not found nothing to reconcile so skip the rest
		return r.OK()
	}

	// Create a snapshot of the instance to have a base for a diff patch at the
	// end of the reconciliation
	r.SnapshotInstance()

	var result Result
	if !r.GetInstance().GetDeletionTimestamp().IsZero() {
		result = reconcileDelete(r, steps)
	} else {
		result = reconcileNormal(r, steps)
	}

	postResult := reconcilePost(r, steps)
	if !postResult.IsOK() {
		if !result.IsOK() {
			r.GetLog().Info(
				"Post step failure overrides existing negative result",
				"post step result", postResult, "dropped result", result,
			)
		}
		result = postResult
	}

	saveResult := saveInstance[T, R](r)
	// intentionally overwrite the normal steps' result only if save
	// failed so a requeue request or a step error is propagated
	if !saveResult.IsOK() {
		return saveResult
	}

	return result
}

// lateStepSetup do late initialization of all steps based on every requested
// step
func lateStepSetup[T client.Object, R Req[T]](steps []Step[T, R], r R) {
	for _, step := range steps {
		step.Setup(steps, r.GetLog())
	}
}

func runStep[T client.Object, R Req[T]](name string, stepF func(r R, log logr.Logger) Result, r R, log logr.Logger) Result {
	stepLog := log.WithName(name)
	result := stepF(r, stepLog)
	if result.IsError() {
		stepLog.Error(result.Err(), result.String())
	} else {
		stepLog.Info(result.String())
	}
	return result
}

func reconcileNormal[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {
	// before we change anything ensure that we have our own finalizer set so
	// we can catch Instance delete and do a proper cleanup
	updated := controllerutil.AddFinalizer(r.GetInstance(), r.GetFinalizer())
	if updated {
		r.GetLog().Info("Added finalizer to ourselves")
		// we intentionally force a requeue immediately here to persist the
		// Instance with the finalizer. We need to have our own
		// finalizer persisted before we try to create any external resources
		return r.RequeueAfter(
			"Requeue to get our finalizer persisted before continue",
			pointer.Duration(r.GetDefaultRequeueTimeout()),
		)
	}

	for _, step := range steps {
		result := runStep[T, R](step.GetName(), step.Do, r, r.GetLog())
		if !result.IsOK() {
			// stop progressing as something failed
			return result
		}
	}
	return r.OK()
}

func reconcileDelete[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {
	r.GetLog().Info("Deleting instance")
	l := r.GetLog().WithName("Cleanup")

	// Do the cleanup calls in reverse order so the last created resource
	// cleaned up first
	for _, step := range reverse(steps) {
		result := runStep[T, R](step.GetName(), step.Cleanup, r, l)
		if !result.IsOK() {
			// skip the rest of the cleanups it will be done in a later
			// reconcile
			return result
		}
	}

	// all cleanups are done successfully so we can remove the finalizer
	// from ourselves
	updated := controllerutil.RemoveFinalizer(r.GetInstance(), r.GetFinalizer())
	if updated {
		r.GetLog().Info("Removed finalizer from ourselves")
	}

	return r.OK()
}

// Reverse a slice. Replace this with slices.Reverse() from golang 1.21
func reverse[S ~[]E, E any](s S) S {
	r := []E{}
	for i := len(s) - 1; i >= 0; i-- {
		r = append(r, s[i])
	}
	return r
}

func readInstance[T client.Object, R Req[T]](r R) (result Result, found bool) {
	err := r.GetClient().Get(r.GetCtx(), r.GetRequest().NamespacedName, r.GetInstance())

	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			r.GetLog().Info("Instance not found, probably deleted before reconciled. Nothing to do.")
			return r.OK(), false
		}
		err := fmt.Errorf("failed to read instance: %w", err)
		return r.Error(err, r.GetLog()), false
	}

	return r.OK(), true
}

func reconcilePost[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {
	l := r.GetLog().WithName("Post")
	// FIXME(gibi): This is to much log with empty function. Either we
	// should check if the function is empty and not run / log it
	// or only log error in optional phases.
	// Also double check if cleanup logging happening properly
	for _, step := range steps {
		result := runStep[T, R](step.GetName(), step.Post, r, l)
		if !result.IsOK() {
			return result
		}
	}
	return r.OK()
}

func saveInstance[T client.Object, R Req[T]](r R) Result {
	patch := client.MergeFrom(r.GetInstanceSnapshot())

	// We need to patch the Instance to allow metadata (finalizer) update and
	// need a separate patch call for the Status. We need to pass a copy to
	// Patch as it will reset the Status fields by reading back the object
	// after Patching the non status part.
	instance := r.GetInstance().DeepCopyObject().(T)
	err := r.GetClient().Patch(r.GetCtx(), instance, patch)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.GetLog().Info("Cannot persist instance as it is deleted")
			return r.OK()
		}

		err := fmt.Errorf("failed to persist instance: %w", err)
		return r.Error(err, r.GetLog())
	}

	err = r.GetClient().Status().Patch(r.GetCtx(), r.GetInstance(), patch)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.GetLog().Info("Cannot persist instance status as it is deleted")
			return r.OK()
		}

		err := fmt.Errorf("failed to persist instance status: %w", err)
		return r.Error(err, r.GetLog())
	}
	return r.OK()
}
