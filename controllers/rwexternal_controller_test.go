package controllers

import (
	"github.com/gibizer/okofw/api/v1beta1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateRWExternal(namespace string, spec v1beta1.RWExternalSpec) types.NamespacedName {
	name := uuid.New().String()
	instance := &v1beta1.RWExternal{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "okofw-example.openstack.org/v1beta1",
			Kind:       "RWExternal",
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

func GetRWExternal(name types.NamespacedName) *v1beta1.RWExternal {
	instance := &v1beta1.RWExternal{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

var _ = Describe("RWExternal controller", func() {
	var namespace string

	BeforeEach(func() {
		namespace = uuid.New().String()
		CreateNamespace(namespace)
		DeferCleanup(DeleteNamespace, namespace)
	})
	It("Reports if input is missing", func() {
		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "foo"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			inputCondition := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), inputCondition))
			g.Expect(inputCondition.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(inputCondition.Message).To(ContainSubstring("Missing input: secret/foo"))
		}, timeout, interval).Should(Succeed())
	})
})
