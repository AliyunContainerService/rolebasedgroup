package reconciler

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	appsapplyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
)

type DeploymentReconciler struct {
	scheme *runtime.Scheme
	client client.Client
}

var _ WorkloadReconciler = &DeploymentReconciler{}

func NewDeploymentReconciler(scheme *runtime.Scheme, client client.Client) *DeploymentReconciler {
	return &DeploymentReconciler{scheme: scheme, client: client}
}

func (r *DeploymentReconciler) Reconciler(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("start to reconciling deploy workload")

	deployApplyConfig, err := r.constructDeployApplyConfiguration(ctx, rbg, role)
	if err != nil {
		logger.Error(err, "Failed to construct deploy apply configuration")
		return err
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployApplyConfig)
	if err != nil {
		logger.Error(err, "Converting obj apply configuration to json.")
		return err
	}
	newDeploy := &appsv1.Deployment{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj, newDeploy); err != nil {
		return fmt.Errorf("convert deployApplyConfig to deploy error: %s", err.Error())
	}

	oldDeploy := &appsv1.Deployment{}
	err = r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, oldDeploy)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	equal, err := SemanticallyEqualDeployment(oldDeploy, newDeploy)
	if equal {
		logger.V(1).Info("deploy equal, skip reconcile")
		return nil
	}

	logger.V(1).Info(fmt.Sprintf("deploy not equal, diff: %s", err.Error()))

	if err := utils.PatchObjectApplyConfiguration(ctx, r.client, deployApplyConfig, utils.PatchSpec); err != nil {
		logger.Error(err, "Failed to patch deploy apply configuration")
		return err
	}
	return nil
}

func (r *DeploymentReconciler) constructDeployApplyConfiguration(
	ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
) (*appsapplyv1.DeploymentApplyConfiguration, error) {
	podReconciler := NewPodReconciler(r.scheme, r.client)
	podTemplateApplyConfiguration, err := podReconciler.ConstructPodTemplateSpecApplyConfiguration(ctx, rbg, role)
	if err != nil {
		return nil, err
	}

	// construct deployment apply configuration
	deployConfig := appsapplyv1.Deployment(rbg.GetWorkloadName(role), rbg.Namespace).
		WithSpec(appsapplyv1.DeploymentSpec().
			WithReplicas(*role.Replicas).
			WithTemplate(podTemplateApplyConfiguration).
			WithSelector(metaapplyv1.LabelSelector().
				WithMatchLabels(rbg.GetCommonLabelsFromRole(role)))).
		WithAnnotations(rbg.GetCommonAnnotationsFromRole(role)).
		WithLabels(rbg.GetCommonLabelsFromRole(role)).
		WithOwnerReferences(metaapplyv1.OwnerReference().
			WithAPIVersion(rbg.APIVersion).
			WithKind(rbg.Kind).
			WithName(rbg.Name).
			WithUID(rbg.GetUID()).
			WithBlockOwnerDeletion(true).
			WithController(true),
		)
	if role.RolloutStrategy.RollingUpdate != nil {
		deployConfig = deployConfig.WithSpec(
			deployConfig.Spec.WithStrategy(appsapplyv1.DeploymentStrategy().
				WithType(appsv1.DeploymentStrategyType(role.RolloutStrategy.Type)).
				WithRollingUpdate(appsapplyv1.RollingUpdateDeployment().
					WithMaxSurge(role.RolloutStrategy.RollingUpdate.MaxSurge).
					WithMaxUnavailable(role.RolloutStrategy.RollingUpdate.MaxUnavailable),
				),
			))
	}
	return deployConfig, nil

}

func (r *DeploymentReconciler) ConstructRoleStatus(
	ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
) (workloadsv1alpha1.RoleStatus, bool, error) {
	updateStatus := false
	deploy := &appsv1.Deployment{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, deploy); err != nil {
		return workloadsv1alpha1.RoleStatus{}, false, err
	}

	currentReplicas := *deploy.Spec.Replicas
	currentReady := deploy.Status.ReadyReplicas
	status, found := rbg.GetRoleStatus(role.Name)
	if !found || status.Replicas != currentReplicas || status.ReadyReplicas != currentReady {
		status = workloadsv1alpha1.RoleStatus{
			Name:          role.Name,
			Replicas:      currentReplicas,
			ReadyReplicas: currentReady,
		}
		updateStatus = true
	}

	return status, updateStatus, nil
}

func (r *DeploymentReconciler) CheckWorkloadReady(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) (bool, error) {
	deploy := &appsv1.Deployment{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, deploy); err != nil {
		return false, err
	}
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas, nil
}

func (r *DeploymentReconciler) CleanupOrphanedWorkloads(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup) error {
	logger := log.FromContext(ctx)
	// list deploy managed by rbg
	deployList := &appsv1.DeploymentList{}
	if err := r.client.List(context.Background(), deployList, client.InNamespace(rbg.Namespace),
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/managed-by": workloadsv1alpha1.ControllerName,
			"app.kubernetes.io/name":       rbg.Name,
		}),
	); err != nil {
		return err
	}

	for _, deploy := range deployList.Items {
		found := false
		for _, role := range rbg.Spec.Roles {
			if role.Workload.Kind == "Deployment" && rbg.GetWorkloadName(&role) == deploy.Name {
				found = true
				break
			}
		}
		if !found {
			logger.Info("delete deploy", "deploy", deploy.Name)
			if err := r.client.Delete(ctx, &deploy); err != nil {
				return fmt.Errorf("delete sts %s error: %s", deploy.Name, err.Error())
			}
		}
	}
	return nil
}

func (r *DeploymentReconciler) RecreateWorkload(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	logger := log.FromContext(ctx)
	if rbg == nil || role == nil {
		return nil
	}

	deployName := rbg.GetWorkloadName(role)
	var deploy appsv1.Deployment
	err := r.client.Get(ctx, types.NamespacedName{Name: deployName, Namespace: rbg.Namespace}, &deploy)
	// if deploy is not found, skip delete deploy
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if deploy.UID == "" {
		return nil
	}

	logger.Info(fmt.Sprintf("Recreate deploy workload, delete deploy %s", deployName))
	if err := r.client.Delete(ctx, &deploy); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// wait new deploy create
	var retErr error
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		var newDeploy appsv1.Deployment
		retErr = r.client.Get(ctx, types.NamespacedName{Name: deployName, Namespace: rbg.Namespace}, &newDeploy)
		if retErr != nil {
			if apierrors.IsNotFound(retErr) {
				return false, nil
			}
			return false, retErr
		}
		return true, nil
	})

	if err != nil {
		logger.Error(retErr, "wait new deploy creating error")
		return retErr
	}

	return nil
}

func SemanticallyEqualDeployment(dep1, dep2 *appsv1.Deployment) (bool, error) {
	if dep1 == nil || dep2 == nil {
		if dep1 != dep2 {
			return false, fmt.Errorf("object is nil")
		} else {
			return true, nil
		}
	}

	if equal, err := objectMetaEqual(dep1.ObjectMeta, dep2.ObjectMeta); !equal {
		return false, fmt.Errorf("objectMeta not equal: %s", err.Error())
	}

	if equal, err := deploymentSpecEqual(dep1.Spec, dep2.Spec); !equal {
		return false, fmt.Errorf("spec not equal: %s", err.Error())
	}

	return true, nil
}

func deploymentSpecEqual(spec1, spec2 appsv1.DeploymentSpec) (bool, error) {
	if spec1.Replicas != nil && spec2.Replicas != nil {
		if *spec1.Replicas != *spec2.Replicas {
			return false, fmt.Errorf("replicas not equal, old: %d, new: %d", *spec1.Replicas, *spec2.Replicas)
		}
	}

	if !reflect.DeepEqual(spec1.Selector, spec2.Selector) {
		return false, fmt.Errorf("selector not equal, old: %v, new: %v", spec1.Selector, spec2.Selector)
	}

	if equal, err := podTemplateSpecEqual(spec1.Template, spec2.Template); !equal {
		return false, fmt.Errorf("podTemplateSpec not equal, %s", err.Error())
	}

	return true, nil
}
