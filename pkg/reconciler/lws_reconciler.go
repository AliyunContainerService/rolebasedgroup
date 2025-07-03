package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	utilpointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	lwsapplyv1 "sigs.k8s.io/lws/client-go/applyconfiguration/leaderworkerset/v1"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
	"time"
)

type LeaderWorkerSetReconciler struct {
	scheme *runtime.Scheme
	client client.Client
}

var _ WorkloadReconciler = &LeaderWorkerSetReconciler{}

func NewLeaderWorkerSetReconciler(scheme *runtime.Scheme, client client.Client) *LeaderWorkerSetReconciler {
	return &LeaderWorkerSetReconciler{scheme: scheme, client: client}
}

func (r *LeaderWorkerSetReconciler) Reconciler(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("start to reconciling lws workload")
	lwsApplyConfig, err := r.constructLWSApplyConfiguration(ctx, rbg, role)
	if err != nil {
		return err
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(lwsApplyConfig)
	if err != nil {
		logger.Error(err, "Converting obj apply configuration to json")
		return err
	}
	newLWS := &lwsv1.LeaderWorkerSet{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj, newLWS); err != nil {
		logger.Error(err, "convert lwsApplyConfig to lws")
		return err
	}
	oldLWS := &lwsv1.LeaderWorkerSet{}
	err = r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, oldLWS)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "get lws failed")
		return err
	}
	equal, err := utils.ObjectsEqual(oldLWS, newLWS)
	if err != nil {
		logger.Error(err, "compare lws error")
		return err
	}
	if equal {
		logger.V(1).Info("lws workload equal")
		return nil
	}
	logger.V(1).Info(fmt.Sprintf("lws not equal, old: %s, new: %s",
		utils.PrettyJson(oldLWS), utils.PrettyJson(newLWS)))

	if err = utils.PatchObjectApplyConfiguration(ctx, r.client, lwsApplyConfig, utils.PatchSpec); err != nil {
		logger.Error(err, "Failed to patch lws apply configuration", "rbg", keyOfRbg(rbg))
		return err
	}
	return nil
}

func (r *LeaderWorkerSetReconciler) ConstructRoleStatus(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) (workloadsv1alpha1.RoleStatus, bool, error) {
	updateStatus := false
	lws := &lwsv1.LeaderWorkerSet{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, lws); err != nil {
		return workloadsv1alpha1.RoleStatus{}, updateStatus, err
	}

	currentReplicas := lws.Status.Replicas
	currentReady := lws.Status.ReadyReplicas
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

func (r *LeaderWorkerSetReconciler) CheckWorkloadReady(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) (bool, error) {
	lws := &lwsv1.LeaderWorkerSet{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, lws); err != nil {
		return false, err
	}
	return lws.Status.ReadyReplicas == lws.Status.Replicas, nil
}

func (r *LeaderWorkerSetReconciler) CleanupOrphanedWorkloads(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup) error {
	logger := log.FromContext(ctx)
	// list lws managed by rbg
	lwsList := &lwsv1.LeaderWorkerSetList{}
	if err := r.client.List(ctx, lwsList, client.InNamespace(rbg.Namespace),
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/managed-by": workloadsv1alpha1.ControllerName,
			"app.kubernetes.io/name":       rbg.Name,
		}),
	); err != nil {
		return err
	}

	for _, lws := range lwsList.Items {
		found := false
		for _, role := range rbg.Spec.Roles {
			if role.Workload.Kind == "LeaderWorkerSet" && rbg.GetWorkloadName(&role) == lws.Name {
				found = true
				break
			}
		}
		if !found {
			logger.Info("delete lws", "lws", lws.Name)
			if err := r.client.Delete(ctx, &lws); err != nil {
				return fmt.Errorf("delete lws %s error: %s", lws.Name, err.Error())
			}
		}
	}
	return nil
}

func (r *LeaderWorkerSetReconciler) constructLWSApplyConfiguration(ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
) (*lwsapplyv1.LeaderWorkerSetApplyConfiguration, error) {
	logger := log.FromContext(ctx)
	// leaderTemplate
	podReconciler := NewPodReconciler(r.scheme, r.client)
	leaderTemp, err := patchPodTemplate(role.Template, role.LeaderWorkerSet.PatchLeaderTemplate)
	if err != nil {
		logger.Error(err, "patch leader podTemplate failed", "rbg", keyOfRbg(rbg))
		return nil, err
	}
	leaderTemplateApplyCfg, err := podReconciler.ConstructPodTemplateSpecApplyConfiguration(ctx, rbg, role, leaderTemp)
	if err != nil {
		logger.Error(err, "patch Construct PodTemplateSpecApplyConfiguration failed", "rbg", keyOfRbg(rbg))
		return nil, err
	}

	// workerTemplate
	workerTemp, err := patchPodTemplate(role.Template, role.LeaderWorkerSet.PatchWorkerTemplate)
	if err != nil {
		logger.Error(err, "patch worker podTemplate failed", "rbg", keyOfRbg(rbg))
		return nil, err
	}
	workerPodReconciler := NewPodReconciler(r.scheme, r.client)
	// workerTemplate do not need to inject sidecar
	workerPodReconciler.SetInjectors([]string{"config", "env"})
	workerTemplateApplyCfg, err := workerPodReconciler.ConstructPodTemplateSpecApplyConfiguration(ctx, rbg, role, workerTemp)
	if err != nil {
		logger.Error(err, "patch Construct PodTemplateSpecApplyConfiguration failed", "rbg", keyOfRbg(rbg))
		return nil, err
	}
	// TODO support SubGroupPolicy
	if role.Replicas == nil {
		role.Replicas = utilpointer.Int32(1)
	}

	//RestartPolicy
	var restartPolicy lwsv1.RestartPolicyType
	if role.RestartPolicy == "None" {
		restartPolicy = lwsv1.NoneRestartPolicy
	} else {
		// if role has RecreateRBGOnPodRestart or RecreateRoleInstanceOnPodRestart policy, set RecreateGroupOnPodRestart for lws
		// it's safe to do so since
		// 1. RecreateGroupOnPodRestart is the default restart policy for lws
		// 2. RecreateRBGOnPodRestart will delete lws if pod recreated or containers restarted
		restartPolicy = lwsv1.RecreateGroupOnPodRestart
	}

	lwsSpecConfig := lwsapplyv1.LeaderWorkerSetSpec().WithReplicas(*role.Replicas).
		WithLeaderWorkerTemplate(
			lwsapplyv1.LeaderWorkerTemplate().
				WithLeaderTemplate(leaderTemplateApplyCfg).
				WithWorkerTemplate(workerTemplateApplyCfg).
				WithSize(*role.LeaderWorkerSet.Size).
				WithRestartPolicy(restartPolicy),
		)

	// RollingUpdate
	if role.RolloutStrategy.RollingUpdate != nil {
		lwsSpecConfig = lwsSpecConfig.WithRolloutStrategy(lwsapplyv1.RolloutStrategy().WithRollingUpdateConfiguration(
			lwsapplyv1.RollingUpdateConfiguration().
				WithMaxSurge(role.RolloutStrategy.RollingUpdate.MaxSurge).
				WithMaxUnavailable(role.RolloutStrategy.RollingUpdate.MaxUnavailable),
		))
	}

	// construct lws apply configuration
	lwsConfig := lwsapplyv1.LeaderWorkerSet(rbg.GetWorkloadName(role), rbg.Namespace).
		WithSpec(lwsSpecConfig).
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
	return lwsConfig, nil

}

func (r *LeaderWorkerSetReconciler) RecreateWorkload(ctx context.Context, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	logger := log.FromContext(ctx)
	if rbg == nil || role == nil {
		return nil
	}

	lwsName := rbg.GetWorkloadName(role)
	var lws lwsv1.LeaderWorkerSet
	err := r.client.Get(ctx, types.NamespacedName{Name: lwsName, Namespace: rbg.Namespace}, &lws)
	// if lws is not found, skip delete lws
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if lws.UID == "" {
		return nil
	}

	logger.Info(fmt.Sprintf("Recreate lws workload, delete lws %s", lws.Name))
	if err := r.client.Delete(ctx, &lws); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// wait new lws create
	var retErr error
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		var newLws lwsv1.LeaderWorkerSet
		retErr = r.client.Get(ctx, types.NamespacedName{Name: lwsName, Namespace: rbg.Namespace}, &newLws)
		if retErr != nil {
			if apierrors.IsNotFound(retErr) {
				return false, nil
			}
			return false, retErr
		}
		return true, nil
	})

	if err != nil {
		logger.Error(retErr, "wait new lws creating error")
		return retErr
	}

	return nil
}

func patchPodTemplate(template corev1.PodTemplateSpec, patch runtime.RawExtension) (corev1.PodTemplateSpec, error) {
	if patch.Raw == nil {
		return template, nil
	}
	tempBytes, _ := json.Marshal(template)
	modified, err := strategicpatch.StrategicMergePatch(tempBytes, patch.Raw, &corev1.PodTemplateSpec{})
	if err != nil {
		return template, err
	}
	newTemp := &corev1.PodTemplateSpec{}
	if err = json.Unmarshal(modified, newTemp); err != nil {
		return template, err
	}
	return *newTemp, nil
}

func keyOfRbg(rbg *workloadsv1alpha1.RoleBasedGroup) string {
	return fmt.Sprintf("%s/%s", rbg.Namespace, rbg.Name)
}
