package reconcile

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step defines a logical step during the reconciliation of the T CRD
// type with the R reconcile request type.
type Step[T client.Object, R Req[T]] interface {
	// GetName returns the name of the step
	GetName() string
	// Setup allow late initialization of the step based on all the
	// other steps added to the RequestHandler. It runs before any Step
	// execution.
	Setup(steps []Step[T, R], log logr.Logger)
	// Do actual reconciliation step on the request.
	// The passed in logger is already set up to have the step name as a
	// context.
	// If Do returns error or requests a requeue then no other Step's Do()
	// function run and the engine moves to execute the Post calls
	// of each Step and then saves the CR.
	Do(r R, log logr.Logger) Result
	// Cleanup resources and finalizers during the deletion of the CR.
	// If Cleanup returns an error or requests a requeue then no other Step's
	// Cleanup run and the engine moves to execute the Post calls
	// of each Step and then saves the CR.
	Cleanup(r R, log logr.Logger) Result
	// Post is called after each step's Do or Cleanup to do late actions
	// just before persisting the CR and returning a result to the
	// controller-runtime.
	// If Post returns an error or requests a requeue then no other Step's
	// Post runs and the engine just saves the CR.
	Post(r R, log logr.Logger) Result
}

// BaseStep is an empty struct that gives default implementation for some of
// the not mandatory Step functions like Setup.
type BaseStep[T client.Object, R Req[T]] struct {
}

func (s BaseStep[T, R]) Setup(steps []Step[T, R], log logr.Logger) {}

func (s BaseStep[T, R]) Cleanup(r R, log logr.Logger) Result {
	return r.OK()
}

func (s BaseStep[T, R]) Post(r R, log logr.Logger) Result {
	return r.OK()
}
