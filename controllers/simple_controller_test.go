package controllers

import (
	testv1 "github.com/gibizer/test-operator/api/v1beta1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Simple controller", func() {
	var namespace string

	BeforeEach(func() {
		namespace = uuid.New().String()
		CreateNamespace(namespace)
		DeferCleanup(DeleteNamespace, namespace)
	})

	It("Divides", func() {
		simpleName := CreateSimple(namespace, testv1.SimpleSpec{Divident: 10, Divisor: 5})
		DeferCleanup(DeleteSimple, simpleName)

		ExpectSimpleStatusReady(simpleName)

		simple := GetSimple(simpleName)
		Expect(*simple.Status.Quotient).To(Equal(2))
		Expect(*simple.Status.Remainder).To(Equal(0))
	})
	It("Failes to divide with zero", func() {
		simpleName := CreateSimple(namespace, testv1.SimpleSpec{Divident: 10, Divisor: 0})
		DeferCleanup(DeleteSimple, simpleName)

		ExpectSimpleStatusDivisonByZero(simpleName)
	})
})
