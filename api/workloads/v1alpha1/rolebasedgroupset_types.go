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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO 参考deployment优化rbgs
// ref: https://github.com/kubernetes/kubernetes/blob/83bb5d570580a3f477737fec5c24ba8fc3554264/staging/src/k8s.io/api/apps/v1/types.go

// RoleBasedGroupSetSpec defines the desired state of RoleBasedGroupSet.
type RoleBasedGroupSetSpec struct {
	// Replicas is the number of RoleBasedGroup that will be created.
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Template describes the RoleBasedGroup that will be created.
	Template RoleBasedGroupSpec `json:"template"`
}

// RoleBasedGroupSetStatus defines the observed state of RoleBasedGroupSet.
type RoleBasedGroupSetStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// +optional
	Replicas int32 `json:"replicas,omitempty" protobuf:"varint,2,opt,name=replicas"`

	// Conditions track the condition of the rbgs
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName={rbgs}

// RoleBasedGroupSet is the Schema for the rolebasedgroupsets API.
type RoleBasedGroupSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleBasedGroupSetSpec   `json:"spec,omitempty"`
	Status RoleBasedGroupSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RoleBasedGroupSetList contains a list of RoleBasedGroupSet.
type RoleBasedGroupSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleBasedGroupSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RoleBasedGroupSet{}, &RoleBasedGroupSetList{})
}
