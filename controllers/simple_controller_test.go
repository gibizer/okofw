package controllers

import (
	v1beta1 "github.com/gibizer/okofw/api/v1beta1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateSimple(namespace string, spec v1beta1.SimpleSpec) types.NamespacedName {
	name := uuid.New().String()
	instance := &v1beta1.Simple{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "okofw-example.openstack.org/v1beta1",
			Kind:       "Simple",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	Expect(k8sClient.Create(ctx, instance)).Should(Succeed())

	logger.Info("Created")
	return types.NamespacedName{Name: name, Namespace: namespace}
}

func GetSimple(name types.NamespacedName) *v1beta1.Simple {
	instance := &v1beta1.Simple{}
	Eventually(func(g Gomega) {
		logger.Info("Try get", "simple", name)
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	logger.Info("Got", "simple", instance)
	return instance
}

func ExpectSimpleStatusReady(simpleName types.NamespacedName) {
	Eventually(func(g Gomega) {
		simple := GetSimple(simpleName)
		g.Expect(simple.Status.Conditions).NotTo(BeNil())
		g.Expect(simple.Status.Conditions[0].Type).To(Equal(condition.ReadyCondition))
		g.Expect(simple.Status.Conditions[0].Status).To(Equal(corev1.ConditionTrue))
	}, timeout, interval).Should(Succeed())

}

func ExpectSimpleStatusDivisionByZero(simpleName types.NamespacedName) {
	Eventually(func(g Gomega) {
		simple := GetSimple(simpleName)
		g.Expect(simple.Status.Conditions).NotTo(BeNil())
		g.Expect(simple.Status.Conditions[0].Type).To(Equal(condition.ReadyCondition))
		g.Expect(simple.Status.Conditions[0].Status).To(Equal(corev1.ConditionFalse))
		g.Expect(simple.Status.Conditions[0].Severity).To(Equal(condition.SeverityError))
		g.Expect(simple.Status.Conditions[0].Reason).To(BeEquivalentTo(condition.ErrorReason))
		g.Expect(simple.Status.Conditions[0].Message).To(ContainSubstring("division by zero"))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("Simple controller", func() {
	var namespace string

	BeforeEach(func() {
		namespace = uuid.New().String()
		CreateNamespace(namespace)
		DeferCleanup(DeleteNamespace, namespace)
	})

	It("Divides", func() {
		simpleName := CreateSimple(namespace, v1beta1.SimpleSpec{Dividend: 10, Divisor: 5})
		DeferCleanup(DeleteInstance, simpleName)

		ExpectSimpleStatusReady(simpleName)

		simple := GetSimple(simpleName)
		Expect(*simple.Status.Quotient).To(Equal(2))
		Expect(*simple.Status.Remainder).To(Equal(0))
	})
	It("Fails to divide with zero", func() {
		simpleName := CreateSimple(namespace, v1beta1.SimpleSpec{Dividend: 10, Divisor: 0})
		DeferCleanup(DeleteInstance, simpleName)

		ExpectSimpleStatusDivisionByZero(simpleName)
	})
})
