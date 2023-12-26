package reconcile

import (
	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a single logical step during the reconciliation of the T CRD
// type with the R reconcile request type
type Step[T client.Object, R Req[T]] interface {
	GetName() string
	GetManagedConditions() condition.Conditions
	Do(r R, log logr.Logger) Result

	SetupFromSteps(steps []Step[T, R], log logr.Logger)
}

type BaseStep[T client.Object, R Req[T]] struct {
}

func (s BaseStep[T, R]) GetManagedConditions() condition.Conditions {
	return []condition.Condition{}
}

func (s BaseStep[T, R]) SetupFromSteps(steps []Step[T, R], log logr.Logger) {}

type SaveInstance[T client.Object, R Req[T]] struct {
	BaseStep[T, R]
}

func (s SaveInstance[T, R]) GetName() string {
	return "PersistInstance"
}

// InitConditions is a generic step that automatically initialize the
// conditions list of the instance Status.
// It collects the conditions managed by other steps to make it so.
type InitConditions[T InstanceWithConditions, R Req[T]] struct {
	BaseStep[T, R]
	conditions condition.Conditions
}

func (s InitConditions[T, R]) GetName() string {
	return "InitConditions"
}

func (s *InitConditions[T, R]) SetupFromSteps(steps []Step[T, R], log logr.Logger) {
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

func (s InitConditions[T, R]) Do(r R, log logr.Logger) Result {
	if r.GetInstance().GetConditions() == nil {
		c := condition.Conditions{}
		c.Init(&s.conditions)
		r.GetInstance().SetConditions(c)
	}
	return r.OK()
}

// RecalculateReadyCondition set the status of the Ready condition based on
// the status of the other conditions in the instance and mirrors the latest
// error to the Ready condition if any.
type RecalculateReadyCondition[T InstanceWithConditions, R Req[T]] struct {
	BaseStep[T, R]
}

func (s RecalculateReadyCondition[T, R]) GetName() string {
	return "RecalculateReadyCondition"
}

func (s RecalculateReadyCondition[T, R]) Do(r R, log logr.Logger) Result {
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
