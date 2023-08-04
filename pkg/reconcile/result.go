package reconcile

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

// Result defines the outcome of a given reconciliation call
type Result interface {
	Unwrap() (ctrl.Result, error)
	String() string
	Err() error
	IsError() bool
	IsRequeue() bool
	IsOK() bool
}

type DefaultResult struct {
	ctrl.Result
	err        error
	requeueMsg string
}

func (r DefaultResult) String() string {
	if r.IsError() {
		return fmt.Sprintf("Failure: %v", r.err)
	}
	if r.IsRequeue() {
		return fmt.Sprintf("Requeue(%s): %s", r.RequeueAfter, r.requeueMsg)
	}
	return "Succeeded"
}

func (r DefaultResult) Unwrap() (ctrl.Result, error) {
	return r.Result, r.err
}

func (r DefaultResult) IsError() bool {
	return r.err != nil
}

func (r DefaultResult) IsRequeue() bool {
	return r.Result != ctrl.Result{}
}

func (r DefaultResult) Err() error {
	return r.err
}

func (r DefaultResult) IsOK() bool {
	return !r.IsError() && !r.IsRequeue()
}
