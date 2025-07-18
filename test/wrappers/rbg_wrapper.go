package wrappers

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workloadsv1alpha "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

type RoleBasedGroupWrapper struct {
	workloadsv1alpha.RoleBasedGroup
}

func (rbgWrapper *RoleBasedGroupWrapper) Obj() *workloadsv1alpha.RoleBasedGroup {
	return &rbgWrapper.RoleBasedGroup
}

func (rbgWrapper *RoleBasedGroupWrapper) WithName(name string) *RoleBasedGroupWrapper {
	rbgWrapper.ObjectMeta.Name = name
	return rbgWrapper
}

func (rbgWrapper *RoleBasedGroupWrapper) WithRoles(roles []workloadsv1alpha.RoleSpec) *RoleBasedGroupWrapper {
	rbgWrapper.Spec.Roles = roles
	return rbgWrapper
}

func (rbgWrapper *RoleBasedGroupWrapper) AddRole(role workloadsv1alpha.RoleSpec) *RoleBasedGroupWrapper {
	rbgWrapper.Spec.Roles = append(rbgWrapper.Spec.Roles, role)
	return rbgWrapper
}

func (rbgWrapper *RoleBasedGroupWrapper) WithGangScheduling(gangScheduling bool) *RoleBasedGroupWrapper {
	if gangScheduling {
		rbgWrapper.Spec.PodGroupPolicy = &workloadsv1alpha.PodGroupPolicy{
			PodGroupPolicySource: workloadsv1alpha.PodGroupPolicySource{
				KubeScheduling: &workloadsv1alpha.KubeSchedulingPodGroupPolicySource{},
			},
		}
	} else {
		rbgWrapper.Spec.PodGroupPolicy = nil
	}

	return rbgWrapper
}

func BuildBasicRoleBasedGroup(name, ns string) *RoleBasedGroupWrapper {
	return &RoleBasedGroupWrapper{
		workloadsv1alpha.RoleBasedGroup{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: workloadsv1alpha.RoleBasedGroupSpec{
				Roles: []workloadsv1alpha.RoleSpec{
					BuildBasicRole("test-role").Obj(),
				},
			},
		},
	}
}
