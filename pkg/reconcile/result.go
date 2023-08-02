package reconcile

import (
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

type Result struct {
	ctrl.Result
	err error
}

func (r Result) String() string {
	if r.err != nil {
		return fmt.Sprintf("Reconciliation failed: %v", r.err)
	}
	if r.Requeue {
		return fmt.Sprintf("Reconciliation requeued after %v", r.RequeueAfter)
	}
	return "Reconciliation succeded"
}

func (r Result) Unwrap() (ctrl.Result, error) {
	return r.Result, r.err
}

func (r *Req[T]) OK() Result {
	return Result{Result: ctrl.Result{}, err: nil}
}

func (r *Req[T]) Error(err error) Result {
	return Result{Result: ctrl.Result{}, err: err}
}

func (r *Req[T]) Requeue(after *time.Duration) Result {
	if after != nil {
		return Result{Result: ctrl.Result{RequeueAfter: *after}, err: nil}
	}
	return Result{Result: ctrl.Result{Requeue: true}, err: nil}
}
