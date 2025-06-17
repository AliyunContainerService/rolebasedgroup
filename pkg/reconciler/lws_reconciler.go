package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	utilpointer "k8s.io/utils/pointer"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	lwsapplyv1 "sigs.k8s.io/lws/client-go/applyconfiguration/leaderworkerset/v1"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
)

const (
	defaultRestartPolicyType = lwsv1.RecreateGroupOnPodRestart
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
	logger.V(1).Info("start to reconciling lws workload", "rbg", keyOfRbg(rbg))
	lwsApplyConfig, err := r.constructLWSApplyConfiguration(ctx, rbg, role)
	if err != nil {
		return err
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(lwsApplyConfig)
	if err != nil {
		logger.Error(err, "Converting obj apply configuration to json", "rbg", keyOfRbg(rbg))
		return err
	}
	newLWS := &lwsv1.LeaderWorkerSet{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj, newLWS); err != nil {
		logger.Error(err, "convert lwsApplyConfig to lws", "rbg", keyOfRbg(rbg))
		return err
	}
	oldLWS := &lwsv1.LeaderWorkerSet{}
	err = r.client.Get(ctx, types.NamespacedName{Name: rbg.GetWorkloadName(role), Namespace: rbg.Namespace}, oldLWS)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "get lws failed", "rbg", keyOfRbg(rbg))
		return err
	}
	equal, err := semanticallyEqualLeaderWorkerSet(oldLWS, newLWS)
	if equal {
		logger.V(1).Info("lws workload equal", "rbg", keyOfRbg(rbg))
		return nil
	}
	logger.Info(fmt.Sprintf("lws not equal, diff: %s", err.Error()), "rbg", keyOfRbg(rbg))
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

func (r *LeaderWorkerSetReconciler) GetWorkloadType() string {
	return "leaderworkerset.x-k8s.io/v1/LeaderWorkerSet"
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
	lwsSpecConfig := lwsapplyv1.LeaderWorkerSetSpec().WithReplicas(*role.Replicas).WithLeaderWorkerTemplate(
		lwsapplyv1.LeaderWorkerTemplate().
			WithLeaderTemplate(leaderTemplateApplyCfg).
			WithWorkerTemplate(workerTemplateApplyCfg).
			WithSize(*role.LeaderWorkerSet.Size).
			WithRestartPolicy(defaultRestartPolicyType),
	)

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

func semanticallyEqualLeaderWorkerSet(lws1, lws2 *lwsv1.LeaderWorkerSet) (bool, error) {
	if lws1 == nil || lws2 == nil {
		if lws1 != lws2 {
			return false, fmt.Errorf("object is nil")
		} else {
			return true, nil
		}
	}

	if equal, err := objectMetaEqual(lws1.ObjectMeta, lws2.ObjectMeta); !equal {
		return false, fmt.Errorf("objectMeta not equal: %s", err.Error())
	}

	if equal, err := lwsSpecEqual(lws1.Spec, lws2.Spec); !equal {
		return false, fmt.Errorf("spec not equal: %s", err.Error())
	}

	return true, nil
}

func lwsSpecEqual(lws1, lws2 lwsv1.LeaderWorkerSetSpec) (bool, error) {
	if equal, err := leaderWorkerTemplateEqual(lws1.LeaderWorkerTemplate, lws2.LeaderWorkerTemplate); !equal {
		return false, fmt.Errorf("leaderWorkerTemplate not equal: %s", err.Error())
	}
	if lws1.Replicas == nil {
		lws1.Replicas = utilpointer.Int32(1)
	}
	if lws2.Replicas == nil {
		lws2.Replicas = utilpointer.Int32(1)
	}
	if *lws1.Replicas != *lws2.Replicas {
		return false, fmt.Errorf("LeaderWorkerSetSpec replicas not equal")
	}
	return true, nil
}

func leaderWorkerTemplateEqual(lwt1, lwt2 lwsv1.LeaderWorkerTemplate) (bool, error) {
	if equal, err := podTemplateSpecEqual(*lwt1.LeaderTemplate, *lwt2.LeaderTemplate); !equal {
		return false, fmt.Errorf("leaderTemplate not equal: %s", err.Error())
	}

	if equal, err := podTemplateSpecEqual(lwt1.WorkerTemplate, lwt2.WorkerTemplate); !equal {
		return false, fmt.Errorf("workerTemplate not equal: %s", err.Error())
	}

	if *lwt1.Size != *lwt2.Size {
		return false, fmt.Errorf("lws size not equal")
	}

	if !reflect.DeepEqual(lwt1.RestartPolicy, lwt2.RestartPolicy) {
		return false, fmt.Errorf("restart policy not equal")
	}
	return true, nil
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
