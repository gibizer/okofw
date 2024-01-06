# openstack-k8s-operators-framework

The goal of okofw is to implement a framework of a generic reconcile engine
that helps implementing openstack service operators without duplicating logic
like:
* loading and saving the state of the reconciled instance
* initializing status conditions and automatically calculating the overall
  Ready condition
* ...

## Concepts
Top of the concepts introduced by the controller-runtime the framework
introduces additional building blocks

**Reconcile request (Req)**

A type encapsulating the data related to a single request to Reconcile a CR
instance. A new request is created for each `Reconcile()` call and individual
reconcile Steps can use it to store data and pass them to other Steps in the
same Reconcile run.

**Reconcile request handler (Handler)**

The engine that executes the reconcile Steps according to the request, handle
step results, loads and saves the CR instance, etc.

**Step**

A single piece of work to be done to reconcile a CR instance run by the
Reconcile request handler. It has access to the CR instance state, and the
Reconcile request and it can manipulate both.

There are different phases of a step execution implemented by different
functions:
* `Do()`: Normal reconciliation
* `Cleanup()`: implement any cleanup action needed during CR deletion
* `Post()`: implement tasks that always needs to be run right before the CR is
  persisted even if a previous step failed.


## Reconcile flow

```
        ┌───────────┐
        │Reconcile()│
        └─────┬─────┘
          ┌───▽───┐
          │Load CR│
          └───┬───┘
  ____________▽_____________
 ╱                          ╲    ┌─────────────────────┐
╱ DeletionTimestamp.isZero() ╲___│Ensure self finalizer│
╲                            ╱yes└──────────┬──────────┘
 ╲__________________________╱     ┌─────────▽─────────┐
              │no                 │For each Step: Do()│
 ┌────────────▽───────────┐       └─────────┬─────────┘
 │For each Step in reverse│                 │
 │order: Cleanup()        │                 │
 └────────────┬───────────┘                 │
   ┌──────────▽──────────┐                  │
   │Remove self finalizer│                  │
   └──────────┬──────────┘                  │
              └───────┬─────────────────────┘
           ┌──────────▽──────────┐
           │For each step: Post()│
           └──────────┬──────────┘
                  ┌───▽───┐
                  │Save CR│
                  └───────┘
```
<!---
Drawn with https://arthursonzogni.com/Diagon/#Flowchart

"Reconcile()"
"Load CR"
if ("DeletionTimestamp.isZero()") {

  "Ensure self finalizer"
  "For each Step: Do()"

}
else {
  "For each Step in reverse order: Cleanup()"
  "Remove self finalizer"
}

"For each step: Post()"
"Save CR"

-->

For each `Reconcile()` call a new `Req` and `Handler` is created based on the
request from the controller-runtime (i.e which CR to reconcile) and based on
the programmer defined Steps (i.e. how to reconcile).


## Implementation

To keep the engine and some common steps (i.e. condition handling) generic the
`Req` is a generic type where the type parameter `T` represents the type of the
CR instance (i.e `Req[T]` means a reconcile request for the T CR type).
The `Handler` takes an `Req` instance so it is also generic with type parameter
`T` and `Req[T]`.

This allows to create both generic and specific Steps. For example
`Conditions` is a generic step where the `T` CR type is only restricted
to support `GetConditions` and `SetCondition` calls.
An example for a type specific step is `EnsureNonZeroDivisor` from the
`simple_controller` where `T` is replaced with the specific CR type
`v1beta1.Simple`. So this step can directly access all the CR specific Spec and
Status fields.

The implementation strategy is to use interfaces to specify the need of the
generic steps towards the CR type. This way the generic step will be reusable
for multiple CRs. But also allow implementing steps that are only useful for
a single CR type without the need to write boilerplate interfaces (or do type
casting) to access all the CR specific fields.

### Available generic steps

* `Conditions`: This step ensure that the every condition is initialized
  and the Ready condition is always recalculated before the CR is saved. It
  requires that the CRD type implements the `InstanceWithConditions` interface

### Examples
* `v1beta1.Simple` + `simple_controller`: Shows the basic Reconcile setup
   without any external dependencies but with Condition handling.
   The reconciler only reads its Spec and writes its Status.
* `v1beta1.RWExternal` + `rwexternal_controller`: The reconciler reads an
   external input (a Secret) and create external output (another Secret) with
   additional cleanup logic for the output during CR deletion.
