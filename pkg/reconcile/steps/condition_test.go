package steps

import (
	"testing"

	"github.com/gibizer/okofw/pkg/reconcile"
	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

type Instance struct {
	client.Object
	Conditions condition.Conditions
}

func (i Instance) GetConditions() condition.Conditions {
	return i.Conditions
}

func (i *Instance) SetConditions(conditions condition.Conditions) {
	i.Conditions = conditions
}

type Req struct {
	reconcile.DefaultReq[*Instance]
}

type Step = reconcile.Step[*Instance, *Req]

var step = Conditions[*Instance, *Req]{}
var log = ctrl.Log

type EmptyStep struct {
	reconcile.BaseStep[*Instance, *Req]
}

func (s EmptyStep) GetName() string {
	return "EmptyStep"
}

func (s EmptyStep) Do(r *Req, log logr.Logger) reconcile.Result {
	return r.OK()
}

type NonConditionManagerStep struct {
	EmptyStep
}

type ConditionManagerStep struct {
	EmptyStep
}

func (s ConditionManagerStep) GetName() string {
	return "ConditionManagerStep"
}

func (s ConditionManagerStep) GetManagedConditions() condition.Conditions {
	return []condition.Condition{
		*condition.UnknownCondition(
			condition.InputReadyCondition,
			condition.InitReason,
			condition.InputReadyInitMessage,
		),
	}
}

func TestStepGetName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(step.GetName()).To(Equal("Conditions"))
}

func TestSetupCollectsConditions(t *testing.T) {
	g := NewWithT(t)
	step.Setup([]Step{
		NonConditionManagerStep{},
		&step,
		ConditionManagerStep{},
	}, log)

	g.Expect(step.conditions).To(HaveLen(1))
	g.Expect(step.conditions[0].Type).To(Equal(condition.InputReadyCondition))
}

func TestSetupCollectsConditionsDedup(t *testing.T) {
	g := NewWithT(t)
	step.Setup([]Step{
		&step,
		ConditionManagerStep{},
		ConditionManagerStep{},
	}, log)

	g.Expect(step.conditions).To(HaveLen(1))
}

func TestSetupOrderingCheckWrongOrder(t *testing.T) {
	g := NewWithT(t)
	setup := func() {
		step.Setup([]Step{
			NonConditionManagerStep{},
			ConditionManagerStep{},
			&step,
		}, log)
	}

	g.Expect(setup).To(
		PanicWith(
			"Step order error. Cannot add step ConditionManagerStep which " +
				"is a ConditionManager before step steps.Conditions"))
}

func TestDoInitializeConditions(t *testing.T) {
	g := NewWithT(t)
	step.Setup([]Step{
		NonConditionManagerStep{},
		&step,
		ConditionManagerStep{},
	}, log)

	req := &Req{}
	req.Instance = &Instance{}

	step.Do(req, log)

	conds := req.Instance.GetConditions()
	g.Expect(conds).To(HaveLen(2))
	g.Expect(conds[0].Type).To(Equal(condition.ReadyCondition))
	g.Expect(conds[0].Status).To(Equal(corev1.ConditionUnknown))
	g.Expect(conds[1].Type).To(Equal(condition.InputReadyCondition))
	g.Expect(conds[1].Status).To(Equal(corev1.ConditionUnknown))
}

func TestPostCalculatesReadyCondition(t *testing.T) {
	g := NewWithT(t)
	step.Setup([]Step{
		NonConditionManagerStep{},
		&step,
		ConditionManagerStep{},
	}, log)

	req := &Req{}
	req.Instance = &Instance{}

	step.Do(req, log)

	req.Instance.Conditions.MarkTrue(condition.InputReadyCondition, "failed")

	step.Post(req, log)

	conds := req.Instance.GetConditions()
	g.Expect(conds).To(HaveLen(2))
	g.Expect(conds[0].Type).To(Equal(condition.ReadyCondition))
	g.Expect(conds[0].Status).To(Equal(corev1.ConditionTrue))
	g.Expect(conds[1].Type).To(Equal(condition.InputReadyCondition))
	g.Expect(conds[1].Status).To(Equal(corev1.ConditionTrue))
}
