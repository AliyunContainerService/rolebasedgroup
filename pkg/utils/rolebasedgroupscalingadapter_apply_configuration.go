package utils

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

type RbgScalingAdapterApplyConfiguration struct {
	metaapplyv1.TypeMetaApplyConfiguration    `json:",inline"`
	*metaapplyv1.ObjectMetaApplyConfiguration `json:"metadata,omitempty"`

	Status *RbgScalingAdapterStatusApplyConfiguration `json:"status,omitempty"`
}

func RoleBasedGroupScalingAdapter(rbgScalingAdapter *v1alpha1.RoleBasedGroupScalingAdapter) *RbgScalingAdapterApplyConfiguration {
	b := &RbgScalingAdapterApplyConfiguration{}
	b.WithName(rbgScalingAdapter.Name)
	b.WithNamespace(rbgScalingAdapter.Namespace)
	b.WithKind(rbgScalingAdapter.Kind)
	b.WithAPIVersion(rbgScalingAdapter.APIVersion)
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) WithAPIVersion(value string) *RbgScalingAdapterApplyConfiguration {
	b.TypeMetaApplyConfiguration.APIVersion = &value
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) WithKind(value string) *RbgScalingAdapterApplyConfiguration {
	b.TypeMetaApplyConfiguration.Kind = &value
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) WithNamespace(value string) *RbgScalingAdapterApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.Namespace = &value
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) WithName(value string) *RbgScalingAdapterApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.Name = &value
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) WithStatus(value *RbgScalingAdapterStatusApplyConfiguration) *RbgScalingAdapterApplyConfiguration {
	b.Status = value
	return b
}

func (b *RbgScalingAdapterApplyConfiguration) ensureObjectMetaApplyConfigurationExists() {
	if b.ObjectMetaApplyConfiguration == nil {
		b.ObjectMetaApplyConfiguration = &metaapplyv1.ObjectMetaApplyConfiguration{}
	}
}

type RbgScalingAdapterStatusApplyConfiguration struct {
	Conditions    []v1.Condition        `json:"conditions,omitempty"`
	Replicas      *int32                `json:"replicas,omitempty"`
	Phase         v1alpha1.AdapterPhase `json:"phase,omitempty"`
	Selector      string                `json:"selector,omitempty"`
	LastScaleTime *v1.Time              `json:"lastScaleTime,omitempty"`
}

func RbgScalingAdapterStatus(status v1alpha1.RoleBasedGroupScalingAdapterStatus) *RbgScalingAdapterStatusApplyConfiguration {
	return &RbgScalingAdapterStatusApplyConfiguration{
		Replicas:      status.Replicas,
		Phase:         status.Phase,
		Selector:      status.Selector,
		LastScaleTime: status.LastScaleTime,
	}
}

func (b *RbgScalingAdapterStatusApplyConfiguration) WithConditions(conditions ...v1.Condition) *RbgScalingAdapterStatusApplyConfiguration {
	for i := range conditions {
		found := false
		for j, cond := range b.Conditions {
			if conditions[i].Type == cond.Type {
				found = true
				if conditions[i].Status != cond.Status {
					b.Conditions[j] = conditions[i]
				}
				break
			}
		}
		if !found {
			b.Conditions = append(b.Conditions, conditions[i])
		}
	}

	return b
}

func (b *RbgScalingAdapterStatusApplyConfiguration) WithPhase(phase v1alpha1.AdapterPhase) *RbgScalingAdapterStatusApplyConfiguration {
	b.Phase = phase
	return b
}

func (b *RbgScalingAdapterStatusApplyConfiguration) WithSelector(selector string) *RbgScalingAdapterStatusApplyConfiguration {
	b.Selector = selector
	return b
}

func (b *RbgScalingAdapterStatusApplyConfiguration) WithReplicas(replicas *int32, scale bool) *RbgScalingAdapterStatusApplyConfiguration {
	b.Replicas = replicas
	if scale {
		now := v1.NewTime(time.Now())
		b.LastScaleTime = &now
	}
	return b
}
