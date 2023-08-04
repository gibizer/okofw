package reconcile

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a single logical step during the reconciliation of the T CRD
// type with the R reconcile request type
type Step[T client.Object, R Req[T]] interface {
	GetName() string
	Do(r R, log logr.Logger) Result
}

type SaveInstance[T client.Object, R Req[T]] struct {
}

func (s SaveInstance[T, R]) GetName() string {
	return "PersistInstance"
}

func (s SaveInstance[T, R]) Do(r R, log logr.Logger) Result {
	err := r.GetClient().Status().Update(r.GetCtx(), r.GetInstance())
	if err != nil {
		return r.Error(err, log)
	}
	return r.OK()
}
