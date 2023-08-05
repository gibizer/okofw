package reconcile

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstanceWithConditions interface {
	client.Object

	GetConditions() condition.Conditions
	SetConditions(condition.Conditions)
}

type ResultGenerator interface {
	OK() Result
	Error(error, logr.Logger) Result
	Requeue(msg string) Result
	RequeueAfter(msg string, after *time.Duration) Result
}

// Req holds a single reconcile request
// T is the CRD type the reconcile request running on
type Req[T client.Object] interface {
	GetCtx() context.Context
	GetLog() logr.Logger
	GetRequest() ctrl.Request
	GetClient() client.Client
	GetInstance() T
	SnapshotInstance()
	GetInstanceSnapshot() T
	GetDefaultRequeueTimeout() time.Duration
	GetFinalizer() string

	ResultGenerator
}

// DefaultReq provides the minimal implementation of a reconcile request. This
// can be used to embed into a CRD specific request type
type DefaultReq[T client.Object] struct {
	Ctx              context.Context
	Log              logr.Logger
	Request          ctrl.Request
	Client           client.Client
	Instance         T
	InstanceSnapshot T
	RequeueTimeout   time.Duration
}

// --- implement Req[T]

func (r *DefaultReq[T]) GetCtx() context.Context {
	return r.Ctx
}

func (r *DefaultReq[T]) GetLog() logr.Logger {
	return r.Log
}

func (r *DefaultReq[T]) GetRequest() ctrl.Request {
	return r.Request
}

func (r *DefaultReq[T]) GetClient() client.Client {
	return r.Client
}

func (r *DefaultReq[T]) GetInstance() T {
	return r.Instance
}

func (r *DefaultReq[T]) SnapshotInstance() {
	r.InstanceSnapshot = r.Instance.DeepCopyObject().(T)
}

func (r DefaultReq[T]) GetInstanceSnapshot() T {
	return r.InstanceSnapshot
}

func (r DefaultReq[T]) GetDefaultRequeueTimeout() time.Duration {
	return r.RequeueTimeout
}

func (r DefaultReq[T]) GetFinalizer() string {
	return r.GetInstance().GetObjectKind().GroupVersionKind().Kind
}

// --- implementing ResultGenerator

func (r DefaultReq[T]) OK() Result {
	return DefaultResult{Result: ctrl.Result{}, err: nil}
}

func (r DefaultReq[T]) Error(err error, log logr.Logger) Result {
	log.Error(err, "")
	return DefaultResult{Result: ctrl.Result{}, err: err}
}

func (r DefaultReq[T]) Requeue(msg string) Result {
	return DefaultResult{
		Result:     ctrl.Result{Requeue: true},
		err:        nil,
		requeueMsg: msg,
	}
}

func (r DefaultReq[T]) RequeueAfter(msg string, after *time.Duration) Result {
	a := r.GetDefaultRequeueTimeout()
	if after != nil {
		a = *after
	}
	return DefaultResult{
		Result:     ctrl.Result{Requeue: true, RequeueAfter: a},
		err:        nil,
		requeueMsg: msg,
	}
}
