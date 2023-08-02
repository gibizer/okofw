package reconcile

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReadInstance[T client.Object, R Req[T]] struct {
}

func (s ReadInstance[T, R]) GetName() string {
	return "Read instance state"
}

func (s ReadInstance[T, R]) Do(r *Req[T]) Result {
	err := r.Client.Get(r.Ctx, r.Request.NamespacedName, r.Instance)
	if err != nil {
		r.Log.Info("Failed to read instance, probably deleted. Nothing to do.", "client error", err)
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
}

type HandleDeleted[T client.Object, R Req[T]] struct {
}

func (s HandleDeleted[T, R]) GetName() string {
	return "Handle instance delete"
}

func (s HandleDeleted[T, R]) Do(r *Req[T]) Result {
	if !r.Instance.GetDeletionTimestamp().IsZero() {
		return r.Error(fmt.Errorf("not and error, instance deleted and cleaned. Refactor to handle stop iterating steps without error"))
	}
	return r.OK()
}

type SaveInstance[T client.Object, R Req[T]] struct {
}

func (s SaveInstance[T, R]) GetName() string {
	return "Persist instance state"
}

func (s SaveInstance[T, R]) Do(r *Req[T]) Result {
	err := r.Client.Status().Update(r.Ctx, r.Instance)
	if err != nil {
		return r.Error(err)
	}
	return r.OK()
}
