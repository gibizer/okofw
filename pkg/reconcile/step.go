package reconcile

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a single logical step during the reconciliation of the T CRD
// type with the R reconcile request type
type Step[T client.Object, R any] interface {
	GetName() string
	Do(r R) Result
}

type SaveInstance[T client.Object, R Req[T]] struct {
}

func (s SaveInstance[T, R]) GetName() string {
	return "Persist instance state"
}

func (s SaveInstance[T, R]) Do(r R) Result {
	err := r.GetClient().Status().Update(r.GetCtx(), r.GetInstance())
	if err != nil {
		return r.Error(err)
	}
	return r.OK()
}
