package steps

import (
	"fmt"

	"github.com/gibizer/okofw/pkg/reconcile"
	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstanceWithConditions interface {
	client.Object

	GetConditions() condition.Conditions
	SetConditions(condition.Conditions)
}

type ConditionManager interface {
	// GetManagedConditions return a list of condition the step might update
	GetManagedConditions() condition.Conditions
}

// Conditions is a generic step that automatically initialize the
// conditions list of the instance Status and ensures that Ready condition
// is updated before the CR is saved. It requires that the CRD type T
// implements the InstanceWithConditions interface.
// It collects the conditions managed by other steps to make it so.
type Conditions[T InstanceWithConditions, R reconcile.Req[T]] struct {
	reconcile.BaseStep[T, R]
	conditions condition.Conditions
}

func (s Conditions[T, R]) GetName() string {
	return "Conditions"
}

func (s *Conditions[T, R]) Setup(
	steps []reconcile.Step[T, R],
	log logr.Logger,
) {
	// collect all the conditions other steps are managing but ignore
	// duplicates
	conditions := map[condition.Type]condition.Condition{}
	// look for ourselves in the step list. If there are other
	// ConditionManagers in the list before us that is a programmer error.
	foundOurselves := false
	for _, step := range steps {
		if step == s {
			foundOurselves = true
		}
		condMgr, ok := step.(ConditionManager)
		if ok {
			if !foundOurselves {
				panic(
					fmt.Sprintf(
						"Step order error. Cannot add step %s which is a "+
							"ConditionManager before step steps.Conditions",
						step.GetName()))
			}
			for _, cond := range condMgr.GetManagedConditions() {
				conditions[cond.Type] = cond
			}
		}
	}
	// ignore ReadyCondition as that always initialized automatically
	delete(conditions, condition.ReadyCondition)

	s.conditions = maps.Values(conditions)
}

func (s Conditions[T, R]) Do(r R, log logr.Logger) reconcile.Result {
	if r.GetInstance().GetConditions() == nil {
		c := condition.Conditions{}
		c.Init(&s.conditions)
		r.GetInstance().SetConditions(c)
	}
	return r.OK()
}

func (s Conditions[T, R]) Post(r R, log logr.Logger) reconcile.Result {
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
