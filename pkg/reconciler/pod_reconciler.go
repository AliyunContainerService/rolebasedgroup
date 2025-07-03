package reconciler

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/discovery"
	"sigs.k8s.io/rbgs/pkg/utils"
)

type PodReconciler struct {
	scheme        *runtime.Scheme
	client        client.Client
	injectObjects []string
}

func NewPodReconciler(scheme *runtime.Scheme, client client.Client) *PodReconciler {
	return &PodReconciler{
		scheme: scheme,
		client: client,
	}
}

func (r *PodReconciler) SetInjectors(injectObjects []string) {
	r.injectObjects = injectObjects
}

func (r *PodReconciler) ConstructPodTemplateSpecApplyConfiguration(
	ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
	podTmpls ...corev1.PodTemplateSpec,
) (*coreapplyv1.PodTemplateSpecApplyConfiguration, error) {
	var podTemplateSpec corev1.PodTemplateSpec
	if len(podTmpls) > 0 {
		podTemplateSpec = podTmpls[0]
	} else {
		podTemplateSpec = *role.Template.DeepCopy()
	}

	// inject objects
	injector := discovery.NewDefaultInjector(r.scheme, r.client)
	if r.injectObjects == nil {
		r.injectObjects = []string{"config", "sidecar", "env"}
	}
	if utils.ContainsString(r.injectObjects, "config") {
		if err := injector.InjectConfig(ctx, &podTemplateSpec, rbg, role); err != nil {
			return nil, fmt.Errorf("failed to inject config: %w", err)
		}
	}
	if utils.ContainsString(r.injectObjects, "sidecar") {
		// sidecar也需要rbg相关的env，先注入sidecar
		if err := injector.InjectSidecar(ctx, &podTemplateSpec, rbg, role); err != nil {
			return nil, fmt.Errorf("failed to inject sidecar: %w", err)
		}
	}
	if utils.ContainsString(r.injectObjects, "env") {
		if err := injector.InjectEnv(ctx, &podTemplateSpec, rbg, role); err != nil {
			return nil, fmt.Errorf("failed to inject env vars: %w", err)
		}
	}

	// construct pod template spec configuration
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&podTemplateSpec)
	if err != nil {
		return nil, err
	}
	var podTemplateApplyConfiguration *coreapplyv1.PodTemplateSpecApplyConfiguration
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj, &podTemplateApplyConfiguration)
	if err != nil {
		return nil, err
	}
	podTemplateApplyConfiguration.WithLabels(rbg.GetCommonLabelsFromRole(role))
	podTemplateApplyConfiguration.WithAnnotations(rbg.GetCommonAnnotationsFromRole(role))

	return podTemplateApplyConfiguration, nil
}
