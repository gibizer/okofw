package reconcile

import (
	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a logical step during the reconciliation of the T CRD
// type with the R reconcile request type.
type Step[T client.Object, R Req[T]] interface {
	// GetName returns the name of the step
	GetName() string
	// Setup allow late initialization of the step based on all the
	// other steps added to the RequestHandler. It runs before any Step
	// execution.
	Setup(steps []Step[T, R], log logr.Logger)
	// GetManagedConditions return a list of condition the step might update
	GetManagedConditions() condition.Conditions
	// Do actual reconciliation step on the request.
	// The passed in logger is already set up to have the step name as a
	// context.
	// If Do returns error or requests a requeue then no other Step's Do()
	// function run and the engine moves to execute the Post calls
	// of each Step and then saves the CR.
	Do(r R, log logr.Logger) Result
	// Cleanup resources and finalizers during the deletion of the CR.
	// If Cleanup returns an error or requests a requeue then no other Step's
	// Cleanup run and the engine moves to execute the Post calls
	// of each Step and then saves the CR.
	Cleanup(r R, log logr.Logger) Result
	// PostDo is called after each step's Do or Cleanup to do late actions
	// just before persisting the CR and returning a result to the
	// controller-runtime.
	// If Post returns an error or requests a requeue then no other Step's
	// Post runs and the engine just saves the CR.
	Post(r R, log logr.Logger) Result
}

// BaseStep is an empty struct that gives default implementation for some of
// the not mandatory Step functions like GetManagedConditions and
// Setup.
type BaseStep[T client.Object, R Req[T]] struct {
}

func (s BaseStep[T, R]) GetManagedConditions() condition.Conditions {
	return []condition.Condition{}
}

func (s BaseStep[T, R]) Setup(steps []Step[T, R], log logr.Logger) {}

func (s BaseStep[T, R]) Cleanup(r R, log logr.Logger) Result {
	return r.OK()
}

func (s BaseStep[T, R]) Post(r R, log logr.Logger) Result {
	return r.OK()
}

// Conditions is a generic step that automatically initialize the
// conditions list of the instance Status and ensures that Ready condition
// is updated before the CR is saved.
// It collects the conditions managed by other steps to make it so.
type Conditions[T InstanceWithConditions, R Req[T]] struct {
	BaseStep[T, R]
	conditions condition.Conditions
}

func (s Conditions[T, R]) GetName() string {
	return "Conditions"
}

func (s *Conditions[T, R]) Setup(steps []Step[T, R], log logr.Logger) {
	// collect all the conditions other steps are managing but ignore
	// duplicates
	conditions := map[condition.Type]condition.Condition{}
	for _, step := range steps {
		for _, cond := range step.GetManagedConditions() {
			conditions[cond.Type] = cond
		}
	}
	// ignore ReadyCondition as that always initialized automatically
	delete(conditions, condition.ReadyCondition)

	s.conditions = maps.Values(conditions)
}

func (s Conditions[T, R]) Do(r R, log logr.Logger) Result {
	if r.GetInstance().GetConditions() == nil {
		c := condition.Conditions{}
		c.Init(&s.conditions)
		r.GetInstance().SetConditions(c)
	}
	return r.OK()
}

func (s Conditions[T, R]) Post(r R, log logr.Logger) Result {
	recalculateReadyCondition(r.GetInstance())
	return r.OK()
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
