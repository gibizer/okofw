package reconcile

import (
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Handler func() (ctrl.Result, error)

// NewReqHandler builds up a function that can handle the current reconcile
// request for CRD type T with reconcile request type R
func NewReqHandler[T client.Object, R Req[T]](
	r R, steps []Step[T, R], cleanups []Step[T, R],
) Handler {
	//TODO(gibi): transform this to a builder

	return func() (ctrl.Result, error) {
		r.GetLog().Info("Reconciling")
		result := handleReq[T, R](r, steps, cleanups)
		r.GetLog().Info("Reconciled", "result", result)
		return result.Unwrap()
	}
}

// handleReq implements a single Reconcile run by going throught each
// reconciliation steps provided.
func handleReq[T client.Object, R Req[T]](
	r R,
	steps []Step[T, R],
	cleanups []Step[T, R],
) Result {

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
		result = reconcileDelete(r, cleanups)
	} else {
		result = reconcileNormal(r, steps)
	}

	saveResult := saveInstance[T, R](r)
	// intentionally overwrite the normal steps' result only if save
	// failed so a requeue request or a step error is propagated
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
	// before we change anything esure that we have our own finalizer set so
	// we can catch Instance delete and do a proper cleanup
	updated := controllerutil.AddFinalizer(r.GetInstance(), r.GetFinalizer())
	if updated {
		r.GetLog().Info("Added finalizer to ourselves")
		// we intentionally force a requeue imediately here to persist the
		// Instance with the finalizer. We need to have our own
		// finalizer persisted before we try to create any external resources
		return r.RequeueAfter(
			"Requeue to get our finalizer persisted before continue",
			pointer.Duration(r.GetDefaultRequeueTimeout()),
		)
	}

	for _, step := range steps {
		result := runStep(step, r)
		if !result.IsOK() {
			// stop progressing as something failed
			return result
		}
	}
	return r.OK()
}

func reconcileDelete[T client.Object, R Req[T]](r R, cleanups []Step[T, R]) Result {
	r.GetLog().Info("Deleting instance")

	for _, step := range cleanups {
		result := runStep(step, r)
		if !result.IsOK() {
			// skip the rest of the cleanups it will be done in a later
			// reconcile
			return result
		}
	}

	// all cleaups are done successfully so we can remove the finalizer
	// from ourselves
	updated := controllerutil.RemoveFinalizer(r.GetInstance(), r.GetFinalizer())
	if updated {
		r.GetLog().Info("Removed finalizer from ourselves")
	}

	return r.OK()
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

func allSubConditionIsTrue(conditions condition.Conditions) bool {
	// It assumes that all of our conditions report success via the True status
	for _, c := range conditions {
		if c.Type == condition.ReadyCondition {
			continue
		}
		if c.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}

func recalculateReadyCondition(instance InstanceWithConditions) {
	conditions := instance.GetConditions()
	if conditions == nil {
		return
	}

	// update the Ready condition based on the sub conditions
	if allSubConditionIsTrue(conditions) {
		conditions.MarkTrue(
			condition.ReadyCondition, condition.ReadyMessage)
	} else {
		// something is not ready so reset the Ready condition
		conditions.MarkUnknown(
			condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
		// and recalculate it based on the state of the rest of the conditions
		conditions.Set(conditions.Mirror(condition.ReadyCondition))
	}
	instance.SetConditions(conditions)
}

func saveInstance[T client.Object, R Req[T]](r R) Result {

	instanceWithConditions, ok := any(r.GetInstance()).(InstanceWithConditions)
	if ok {
		recalculateReadyCondition(instanceWithConditions)
	}

	patch := client.MergeFrom(r.GetInstanceSnapshot())

	// We need to patch the Instance to allow metadata (finalizer) update and
	// need a separate patch call for the Status. We need to pass a copy to
	// Patch as it will reset the Status fields by reading back the object
	// after Patching the non status part.
	instance := r.GetInstance().DeepCopyObject().(T)
	err := r.GetClient().Patch(r.GetCtx(), instance, patch)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.GetLog().Info("Cannot perist instance as it is deleted")
			return r.OK()
		}

		err := fmt.Errorf("failed to persist instance: %w", err)
		return r.Error(err, r.GetLog())
	}

	err = r.GetClient().Status().Patch(r.GetCtx(), r.GetInstance(), patch)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.GetLog().Info("Cannot perist instance status as it is deleted")
			return r.OK()
		}

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
