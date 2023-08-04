package reconcile

import (
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

	// do the late setup of all steps based on every requested step
	// NOTE(gibi): this is a bit wasteful as steps are static between reconcile
	// runs so this setup could be done only once at manager setup
	for _, step := range steps {
		step.SetupFromSteps(steps, r.GetLog())
	}

	var result Result

	// Read the instance
	err := r.GetClient().Get(r.GetCtx(), r.GetRequest().NamespacedName, r.GetInstance())
	if err != nil {
		r.GetLog().Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.OK()
	}

	if !r.GetInstance().GetDeletionTimestamp().IsZero() {
		// Cleanup reconciliation
		// TODO(gibi): create a delete path for cleanup
		r.GetLog().Info("Deleting instance")
	} else {
		// Normal reconciliation
		for _, step := range steps {
			result = runStep(step, r)
			if !result.IsOK() {
				// jump to final steps
				break
			}
		}
	}

	for _, step := range postSteps {
		// We don't want to override the steps result unless the post step
		// result is signalling a negative result
		postResult := runStep(step, r)
		if !postResult.IsOK() {
			// override the result and run the rest of the post steps
			result = postResult
		}
	}

	return result
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

// func SetupFromSteps[T client.Object, R Req[T]](
// 	steps []Step[T, R], builder *builder.Builder,
// ) *builder.Builder {

// 	return builder
// }
