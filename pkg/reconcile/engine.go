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
			// TODO(gibi): create a step specific logger for the step
			result = step.Do(r)
			if result.IsError() {
				r.GetLog().Error(result.Err(), fmt.Sprintf("Step: %s: %s", step.GetName(), result))
				// jump to final steps
				break
			}
			if result.IsRequeue() {
				r.GetLog().Info(fmt.Sprintf("Step: %s: %s ", step.GetName(), result))
				// jump to final steps
				break
			}
			r.GetLog().Info(fmt.Sprintf("Step: %s: %s", step.GetName(), result))
		}
	}

	for _, step := range postSteps {
		// We don't want to override the steps result unless the post step
		// result is signalling a negative result
		postResult := step.Do(r)
		if postResult.IsError() {
			r.GetLog().Error(result.Err(), fmt.Sprintf("PostStep: %s: %s", step.GetName(), postResult))
			result = postResult
			// run the rest of the post steps
		}
		if postResult.IsRequeue() {
			r.GetLog().Info(fmt.Sprintf("PostStep: %s: %s", step.GetName(), postResult))
			result = postResult
			// run the rest of the post steps
		}
		r.GetLog().Info(fmt.Sprintf("PostStep: %s: %s", step.GetName(), postResult))
	}

	return result
}
