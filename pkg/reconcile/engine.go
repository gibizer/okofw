package reconcile

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler func() (ctrl.Result, error)

// NewReqHandler builds up a function that can handle the current reconcile
// request for CRD type T with reconcile request type R
func NewReqHandler[T client.Object, R Req[T]](
	r R, steps []Step[T, R],
) Handler {
	//TODO(gibi): transform this to a builder

	return func() (ctrl.Result, error) {
		r.GetLog().Info("Reconciling")
		result := handleReq[T, R](r, steps)
		r.GetLog().Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

// handleReq implements a single Reconcile run by going throught each
// reconciliation steps provided.
func handleReq[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {

	// NOTE(gibi): this is a bit wasteful as steps are static between reconcile
	// runs so this setup could be done only once at manager setup
	lateStepSetup(steps, r)

	result := r.OK()

	// Read the instance
	readResult, found := readInstance[T, R](r)
	if !readResult.IsOK() {
		return readResult
	}
	if !found {
		return r.OK()
	}

	// Create a snapshot of the instance to have a base for a diff patch at the
	// end of the reconciliation
	r.SnapshotInstance()

	if !r.GetInstance().GetDeletionTimestamp().IsZero() {
		reconcileDelete[T, R](r)
	} else {
		result = reconcileNormal(r, steps)
	}

	// TODO(gibi): implement Ready condition calculation before save

	saveResult := saveInstance[T, R](r)
	// intentionally only overwriting the normal steps' result only if save
	// failed so a requeue request or a step error is propagated if save
	// succeeded
	if !saveResult.IsOK() {
		return saveResult
	}

	return result
}

// lateStepSetup do late initalization of all steps based on every requested
// step
func lateStepSetup[T client.Object, R Req[T]](steps []Step[T, R], r R) {
	for _, step := range steps {
		step.SetupFromSteps(steps, r.GetLog())
	}
}

func runStep[T client.Object, R Req[T]](step Step[T, R], r R) Result {
	stepLog := r.GetLog().WithName(step.GetName())
	result := step.Do(r, stepLog)
	if result.IsError() {
		stepLog.Error(result.Err(), result.String())
	} else {
		stepLog.Info(result.String())
	}
	return result
}

func reconcileNormal[T client.Object, R Req[T]](r R, steps []Step[T, R]) Result {
	result := r.OK()
	for _, step := range steps {
		result = runStep(step, r)
		if !result.IsOK() {
			// stop progressing
			return result
		}
	}
	return result
}

func reconcileDelete[T client.Object, R Req[T]](r R) {
	// TODO(gibi): create delete customizations
	r.GetLog().Info("Deleting instance")
}

func readInstance[T client.Object, R Req[T]](r R) (result Result, found bool) {
	// TODO(gibi): hande NotFound and error differently
	err := r.GetClient().Get(r.GetCtx(), r.GetRequest().NamespacedName, r.GetInstance())
	if err != nil {
		r.GetLog().Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.OK(), false
	}
	return r.OK(), true
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
		err := fmt.Errorf("failed to persist instance: %w", err)
		return r.Error(err, r.GetLog())
	}

	err = r.GetClient().Status().Patch(r.GetCtx(), r.GetInstance(), patch)
	if err != nil {
		err := fmt.Errorf("failed to persist instance status: %w", err)
		return r.Error(err, r.GetLog())
	}
	return r.OK()
}

// func SetupFromSteps[T client.Object, R Req[T]](
// 	steps []Step[T, R], builder *builder.Builder,
// ) *builder.Builder {

// 	return builder
// }
