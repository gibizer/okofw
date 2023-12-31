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

	It("Reports if input secret is missing", func() {
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

	It("Initialize every conditions", func() {
		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "foo"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).To(HaveLen(3))

			cond := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(cond.Message).To(ContainSubstring("Missing input: secret/foo"))

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", v1beta1.OutputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionUnknown))

			// Ready condition mirrors the latest error
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.ReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(cond.Message).To(ContainSubstring("Missing input: secret/foo"))

		}, timeout, interval).Should(Succeed())
	})

	It("Reports if input field is missing", func() {
		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{})
		DeferCleanup(DeleteInstance, secretName)

		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			inputCondition := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), inputCondition))
			g.Expect(inputCondition.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(inputCondition.Message).To(
				ContainSubstring(
					"Input data error occurred field 'dividend' not found " +
						"in secret/input"))
		}, timeout, interval).Should(Succeed())
	})

	It("Reports if input field is wrongly formatted", func() {
		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{
			"dividend": []byte("10"),
			"divisor":  []byte("not-an-int"),
		})
		DeferCleanup(DeleteInstance, secretName)

		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			inputCondition := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), inputCondition))
			g.Expect(inputCondition.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(inputCondition.Message).To(
				ContainSubstring(
					"Input data error occurred 'divisor' in secret/input " +
						"cannot be converted to int: strconv.Atoi: parsing " +
						"\"not-an-int\": invalid syntax"))
		}, timeout, interval).Should(Succeed())
	})

	It("Requeue until input secret is available", func() {
		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)
		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			inputCondition := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), inputCondition))
			g.Expect(inputCondition.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(inputCondition.Message).To(ContainSubstring("Missing input: secret/input"))
		}, timeout, interval).Should(Succeed())

		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{
			"dividend": []byte("10"),
			"divisor":  []byte("5"),
		})
		DeferCleanup(DeleteInstance, secretName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			inputCondition := &condition.Condition{}
			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), inputCondition))
			g.Expect(inputCondition.Status).To(Equal(corev1.ConditionTrue))
		}, timeout, interval).Should(Succeed())
	})

	It("Reports division by zero", func() {
		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{
			"dividend": []byte("10"),
			"divisor":  []byte("0"),
		})
		DeferCleanup(DeleteInstance, secretName)

		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			cond := &condition.Condition{}

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", v1beta1.OutputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionFalse))
			g.Expect(cond.Message).To(ContainSubstring("Output generation failed: division by zero"))

		}, timeout, interval).Should(Succeed())
	})

	It("Stores the result in an output Secret", func() {
		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{
			"dividend": []byte("10"),
			"divisor":  []byte("5"),
		})
		DeferCleanup(DeleteInstance, secretName)

		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			cond := &condition.Condition{}

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.InputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", v1beta1.OutputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", condition.ReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))

			g.Expect(rw.Status.OutputSecret).NotTo(BeNil())
			output := th.GetSecret(types.NamespacedName{Namespace: namespace, Name: *rw.Status.OutputSecret})
			g.Expect(output.Data).To(HaveKeyWithValue("quotient", []byte("2")))
			g.Expect(output.Data).To(HaveKeyWithValue("remainder", []byte("0")))

		}, timeout, interval).Should(Succeed())
	})

	It("Deletes the output secret when RWExternal is deleted", func() {
		secretName := types.NamespacedName{Namespace: namespace, Name: "input"}
		th.CreateSecret(secretName, map[string][]byte{
			"dividend": []byte("10"),
			"divisor":  []byte("5"),
		})
		DeferCleanup(DeleteInstance, secretName)

		rwName := CreateRWExternal(namespace, v1beta1.RWExternalSpec{InputSecret: "input"})
		DeferCleanup(DeleteInstance, rwName)

		Eventually(func(g Gomega) {
			rw := GetRWExternal(rwName)
			g.Expect(rw.Status.Conditions).NotTo(BeNil())
			cond := &condition.Condition{}

			g.Expect(rw.Status.Conditions).To(ContainElement(HaveField("Type", v1beta1.OutputReadyCondition), cond))
			g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))

			g.Expect(rw.Status.OutputSecret).NotTo(BeNil())
			th.GetSecret(types.NamespacedName{Namespace: namespace, Name: *rw.Status.OutputSecret})

			g.Expect(rw.Finalizers).To(ContainElement("RWExternal"))

		}, timeout, interval).Should(Succeed())

		rw := GetRWExternal(rwName)
		th.DeleteInstance(rw)

		Eventually(func(g Gomega) {
			th.AssertSecretDoesNotExist(types.NamespacedName{Namespace: namespace, Name: *rw.Status.OutputSecret})
		}, timeout, interval).Should(Succeed())

	})

})
