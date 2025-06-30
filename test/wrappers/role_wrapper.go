package wrappers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	workloadsv1alpha "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"strings"
)

type RoleWrapper struct {
	workloadsv1alpha.RoleSpec
}

func (roleWrapper *RoleWrapper) Obj() workloadsv1alpha.RoleSpec {
	return roleWrapper.RoleSpec
}

func (roleWrapper *RoleWrapper) WithName(name string) *RoleWrapper {
	roleWrapper.Name = name
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithReplicas(size int32) *RoleWrapper {
	roleWrapper.Replicas = ptr.To(size)
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithMaxUnavailable(value int32) *RoleWrapper {
	if roleWrapper.RolloutStrategy.RollingUpdate == nil {
		roleWrapper.RolloutStrategy.RollingUpdate = &workloadsv1alpha.RollingUpdate{}
	}
	roleWrapper.RolloutStrategy.RollingUpdate.MaxUnavailable = intstr.FromInt32(value)
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithMaxSurge(value int32) *RoleWrapper {
	if roleWrapper.RolloutStrategy.RollingUpdate == nil {
		roleWrapper.RolloutStrategy.RollingUpdate = &workloadsv1alpha.RollingUpdate{}
	}
	roleWrapper.RolloutStrategy.RollingUpdate.MaxSurge = intstr.FromInt32(value)
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithTemplate(template corev1.PodTemplateSpec) *RoleWrapper {
	roleWrapper.Template = template
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithDependencies(dependencies []string) *RoleWrapper {
	roleWrapper.Dependencies = dependencies
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithRestartPolicy(restartPolicy workloadsv1alpha.RestartPolicyType) *RoleWrapper {
	roleWrapper.RestartPolicy = restartPolicy
	return roleWrapper
}

func (roleWrapper *RoleWrapper) WithWorkload(workloadType string) *RoleWrapper {
	switch strings.ToLower(workloadType) {
	case "deployment", "deploy":
		roleWrapper.Workload = workloadsv1alpha.WorkloadSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		}
	case "statefulset", "sts":
		roleWrapper.Workload = workloadsv1alpha.WorkloadSpec{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		}
	case "leaderworkerset", "lws":
		roleWrapper.Workload = workloadsv1alpha.WorkloadSpec{
			APIVersion: "leaderworkerset.x-k8s.io/v1",
			Kind:       "LeaderWorkerSet",
		}
	default:
		panic("workload type not supported")
	}
	return roleWrapper
}

func BuildBasicRole(name string) *RoleWrapper {
	return &RoleWrapper{
		workloadsv1alpha.RoleSpec{
			Name:     name,
			Replicas: ptr.To(int32(1)),
			Template: BuildPodSpec(),
		},
	}
}
