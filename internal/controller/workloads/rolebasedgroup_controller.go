/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workloads

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/errors"
	"reflect"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/dependency"
	"sigs.k8s.io/rbgs/pkg/reconciler"
	"sigs.k8s.io/rbgs/pkg/utils"
)

// RoleBasedGroupReconciler reconciles a RoleBasedGroup object
type RoleBasedGroupReconciler struct {
	client    client.Client
	apiReader client.Reader
	scheme    *runtime.Scheme
	recorder  record.EventRecorder
}

func NewRoleBasedGroupReconciler(mgr ctrl.Manager) *RoleBasedGroupReconciler {
	return &RoleBasedGroupReconciler{
		client:    mgr.GetClient(),
		apiReader: mgr.GetAPIReader(),
		scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor("RoleBasedGroup"),
	}
}

// +kubebuilder:rbac:groups=workloads.x-k8s.io,resources=rolebasedgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.x-k8s.io,resources=rolebasedgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.x-k8s.io,resources=rolebasedgroups/finalizers,verbs=update
func (r *RoleBasedGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the RoleBasedGroup instance
	rbg := &workloadsv1alpha1.RoleBasedGroup{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, rbg); err != nil {
		r.recorder.Eventf(rbg, corev1.EventTypeWarning, FailedGetRBG,
			"Failed to get rbg, err: %s", err.Error())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if rbg.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx).WithValues("rbg", klog.KObj(rbg))
	ctx = ctrl.LoggerInto(ctx, logger)
	logger.Info("Start reconciling")

	// Process roles in dependency order
	dependencyManager := dependency.NewDefaultDependencyManager(r.scheme, r.client)
	sortedRoles, err := dependencyManager.SortRoles(ctx, rbg)
	if err != nil {
		r.recorder.Event(rbg, corev1.EventTypeWarning, InvalidRoleDependency, err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile role, add & update
	var roleStatuses []workloadsv1alpha1.RoleStatus
	var updateStatus bool
	for _, role := range sortedRoles {
		// Check dependencies first
		ready, err := dependencyManager.CheckDependencyReady(ctx, rbg, role)
		if err != nil {
			r.recorder.Event(rbg, corev1.EventTypeWarning, FailedCheckRoleDependency, err.Error())
			return ctrl.Result{}, err
		}
		if !ready {
			logger.Info("Dependencies not met, requeuing", "role", role.Name)
			return ctrl.Result{RequeueAfter: 5}, nil
		}

		reconciler, err := reconciler.NewWorkloadReconciler(role.Workload, r.scheme, r.client)
		if err != nil {
			logger.Error(err, "Failed to create workload reconciler")
			r.recorder.Eventf(rbg, corev1.EventTypeWarning, FailedReconcileWorkload,
				"Failed to reconcile role %s: %v", role.Name, err)
			return ctrl.Result{}, err
		}

		if err := reconciler.Reconciler(ctx, rbg, role); err != nil {
			r.recorder.Eventf(rbg, corev1.EventTypeWarning, FailedReconcileWorkload,
				"Failed to reconcile role %s: %v", role.Name, err)
			return ctrl.Result{}, err
		}

		roleStatus, updateRoleStatus, err := reconciler.ConstructRoleStatus(ctx, rbg, role)
		if err != nil {
			// if workload is creating, skip patch status
			if apierrors.IsNotFound(err) {
				continue
			}
			r.recorder.Eventf(rbg, corev1.EventTypeWarning, FailedReconcileWorkload,
				"Failed to construct role %s status: %v", role.Name, err)
			return ctrl.Result{}, err
		}
		updateStatus = updateStatus || updateRoleStatus
		roleStatuses = append(roleStatuses, roleStatus)
	}

	if updateStatus {
		if err := r.updateRBGStatus(ctx, rbg, roleStatuses); err != nil {
			r.recorder.Eventf(rbg, corev1.EventTypeWarning, FailedUpdateStatus,
				"Failed to update status for %s: %v", rbg.Name, err)
			return ctrl.Result{}, err
		}
	}

	// delete role
	if err := r.deleteRoles(ctx, rbg); err != nil {
		r.recorder.Eventf(rbg, corev1.EventTypeWarning, "delete role error",
			"Failed to delete roles for %s: %v", rbg.Name, err)
		return ctrl.Result{}, err
	}

	r.recorder.Event(rbg, corev1.EventTypeNormal, Succeed, "ReconcileSucceed")
	return ctrl.Result{}, nil
}

func (r *RoleBasedGroupReconciler) deleteRoles(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup) error {
	errs := make([]error, 0)
	deployRecon := reconciler.NewDeploymentReconciler(r.scheme, r.client)
	if err := deployRecon.CleanupOrphanedWorkloads(ctx, rbg); err != nil {
		errs = append(errs, err)
	}

	stsRecon := reconciler.NewDeploymentReconciler(r.scheme, r.client)
	if err := stsRecon.CleanupOrphanedWorkloads(ctx, rbg); err != nil {
		errs = append(errs, err)
	}

	lwsRecon := reconciler.NewLeaderWorkerSetReconciler(r.scheme, r.client)
	if err := lwsRecon.CleanupOrphanedWorkloads(ctx, rbg); err != nil {
		errs = append(errs, err)
	}

	return errors.NewAggregate(errs)
}

func (r *RoleBasedGroupReconciler) updateRBGStatus(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, roleStatus []workloadsv1alpha1.RoleStatus) error {
	// update ready condition
	var readyCondition metav1.Condition
	for _, role := range roleStatus {
		if role.ReadyReplicas != role.Replicas {
			readyCondition = metav1.Condition{
				Type:               string(workloadsv1alpha1.RoleBasedGroupReady),
				Status:             metav1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "RoleNotReady",
				Message:            fmt.Sprintf("role %s is not ready", role.Name),
			}
		}
	}

	readyCondition = metav1.Condition{
		Type:               string(workloadsv1alpha1.RoleBasedGroupReady),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "AllRolesReady",
		Message:            "All roles are ready",
	}

	setCondition(rbg, readyCondition)

	// update role status
	for i := range roleStatus {
		found := false
		for j, oldStatus := range rbg.Status.RoleStatuses {
			// if found, update
			if roleStatus[i].Name == oldStatus.Name {
				found = true
				if roleStatus[i].Replicas != oldStatus.Replicas || roleStatus[i].ReadyReplicas != oldStatus.ReadyReplicas {
					rbg.Status.RoleStatuses[j] = roleStatus[i]
				}
				break
			}
		}
		if !found {
			rbg.Status.RoleStatuses = append(rbg.Status.RoleStatuses, roleStatus[i])
		}
	}

	// update rbg status
	rbgApplyConfig := utils.RoleBasedGroup(rbg.Name, rbg.Namespace, rbg.Kind, rbg.APIVersion).
		WithStatus(utils.RbgStatus().WithRoleStatuses(rbg.Status.RoleStatuses).WithConditions(rbg.Status.Conditions))

	return utils.PatchObjectApplyConfiguration(ctx, r.client, rbgApplyConfig, utils.PatchStatus)

}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleBasedGroupReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	controller := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&workloadsv1alpha1.RoleBasedGroup{}, builder.WithPredicates(RBGPredicate())).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(WorkloadPredicate())).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(WorkloadPredicate())).
		Owns(&corev1.Service{}).
		Named("workloads-rolebasedgroup")

	err := utils.CheckCrdExists(r.apiReader, "leaderworkerset.x-k8s.io")
	if err == nil {
		controller.Owns(&lwsv1.LeaderWorkerSet{}, builder.WithPredicates(WorkloadPredicate()))
	}

	return controller.Complete(r)
}

// CheckCrdExists checks if the specified Custom Resource Definition (CRD) exists in the Kubernetes cluster.
func (r *RoleBasedGroupReconciler) CheckCrdExists() error {
	crds := []string{
		"rolebasedgroups.workloads.x-k8s.io",
		"clusterengineruntimeprofiles.workloads.x-k8s.io",
	}

	for _, crd := range crds {
		if err := utils.CheckCrdExists(r.apiReader, crd); err != nil {
			return err
		}
	}
	return nil
}

func RBGPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.(*workloadsv1alpha1.RoleBasedGroup)
			if ok {
				ctrl.Log.Info("enqueue: rbg create event", "rbg", klog.KObj(e.Object))
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRbg, ok1 := e.ObjectOld.(*workloadsv1alpha1.RoleBasedGroup)
			newRbg, ok2 := e.ObjectNew.(*workloadsv1alpha1.RoleBasedGroup)
			if ok1 && ok2 {
				if !reflect.DeepEqual(oldRbg.Spec, newRbg.Spec) {
					ctrl.Log.Info("enqueue: rbg update event", "rbg", klog.KObj(e.ObjectOld))
					return true
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, ok := e.Object.(*workloadsv1alpha1.RoleBasedGroup)
			if ok {
				ctrl.Log.Info("enqueue: rbg delete event", "rbg", klog.KObj(e.Object))
				return true
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func WorkloadPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// ignore workload create event
			return false
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			ctrl.Log.V(1).Info("enter workload.onUpdateFunc", "name", e.ObjectNew.GetName(),
				"namespace", e.ObjectNew.GetNamespace(),
				"type", e.ObjectNew.GetObjectKind().GroupVersionKind().String())
			// Defensive check for nil objects
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}

			// Check validity of OwnerReferences for both old and new objects
			targetGVK := getRbgGVK()
			if !hasValidOwnerRef(e.ObjectOld, targetGVK) ||
				!hasValidOwnerRef(e.ObjectNew, targetGVK) {
				return false
			}

			equal, err := reconciler.WorkloadEqual(e.ObjectOld, e.ObjectNew)
			if !equal {
				if err != nil {
					ctrl.Log.V(1).Info("enqueue: workload update event",
						"rbg", klog.KObj(e.ObjectOld), "diff", err.Error())
				}
				return true
			}

			return false
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			// Ignore objects without valid OwnerReferences
			if e.Object == nil || !hasValidOwnerRef(e.Object, getRbgGVK()) {
				return false
			}

			ctrl.Log.V(1).Info("enqueue: workload delete event", "rbg", klog.KObj(e.Object))
			return true
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return false
		},
	}
}

// hasValidOwnerRef checks if the object has valid OwnerReferences matching target GVK
// Returns true only when:
// 1. Object has non-empty OwnerReferences
// 2. At least one OwnerReference matches target GroupVersionKind
func hasValidOwnerRef(obj client.Object, targetGVK schema.GroupVersionKind) bool {
	refs := obj.GetOwnerReferences()
	if len(refs) == 0 {
		return false
	}
	return utils.CheckOwnerReference(refs, targetGVK)
}

func getRbgGVK() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(workloadsv1alpha1.GroupVersion.String(), "RoleBasedGroup")
}
