package reconcile

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a single logical step during the reconciliation of the T CRD
// type with the R reconcile request type
type Step[T client.Object, R any] interface {
	GetName() string
	Do(r R) Result
}

type ReadInstance[T client.Object, R Req[T]] struct {
}

func (s ReadInstance[T, R]) GetName() string {
	return "Read instance state"
}

func (s ReadInstance[T, R]) Do(r R) Result {
	err := r.GetClient().Get(r.GetCtx(), r.GetRequest().NamespacedName, r.GetInstance())
	if err != nil {
		r.GetLog().Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
}

type HandleDeleted[T client.Object, R Req[T]] struct {
}

func (s HandleDeleted[T, R]) GetName() string {
	return "Handle instance delete"
}

func (s HandleDeleted[T, R]) Do(r R) Result {
	if !r.GetInstance().GetDeletionTimestamp().IsZero() {
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
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
