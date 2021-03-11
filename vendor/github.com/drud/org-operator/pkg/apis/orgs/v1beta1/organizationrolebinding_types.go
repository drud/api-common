/*
Copyright 2019 DRUD Technologies.

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

package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OrganizationRoleBindingConditionType string

const (
	// OrganizationRoleBindingBucketReady reflects status of RBAC policies
	RBACSynced OrganizationRoleBindingConditionType = "RBACSynced"

	// OrganizationRoleBindingBucketReady reflects status of RBAC policies
	CustRBACSynced OrganizationRoleBindingConditionType = "CustRBACSynced"
)

// OrganizationRoleBindingSpec defines the desired state of OrganizationRoleBinding
type OrganizationRoleBindingSpec struct {
	OrgAdmins []string `json:"orgAdmins,omitempty"`

	// OrgDevelopers can CRUD any Site or subresource of a site
	OrgDevelopers []string `json:"orgDevelopers,omitempty"`
}

// OrganizationRoleBindingCondition describes status conditons
type OrganizationRoleBindingCondition struct {
	// Type of OrganizationRoleBinding condition.
	Type OrganizationRoleBindingConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// OrganizationRoleBindingStatus defines the observed state of OrganizationRoleBinding
type OrganizationRoleBindingStatus struct {
	ActiveOrgAdmins     int `json:"activeOrgAdmins"`
	ActiveOrgDevelopers int `json:"activeOrgDevelopers"`

	// Conditions
	Conditions []OrganizationRoleBindingCondition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OrganizationRoleBinding is the Schema for the organizationrolebindings API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="ActiveAdmins",type="integer",JSONPath=".status.activeOrgAdmins",description="ActiveOrgAdmins"
// +kubebuilder:printcolumn:name="ActiveDevelopers",type="integer",JSONPath=".status.activeOrgDevelopers",description="ActiveOrgDevelopers"
// +kubebuilder:subresource:status
// +kubebuilder:resource:path="organizationrolebindings",shortName="orgrb"
type OrganizationRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationRoleBindingSpec   `json:"spec,omitempty"`
	Status OrganizationRoleBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OrganizationRoleBindingList contains a list of OrganizationRoleBinding
type OrganizationRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrganizationRoleBinding `json:"items"`
}

func (r *OrganizationRoleBinding) GetCondition(t OrganizationRoleBindingConditionType) *OrganizationRoleBindingCondition {
	for i, c := range r.Status.Conditions {
		if c.Type == t {
			return &r.Status.Conditions[i]
		}
	}
	return nil
}

func (r *OrganizationRoleBinding) SetCondition(t OrganizationRoleBindingConditionType, s v1.ConditionStatus, reason, message string) {
	condition := r.GetCondition(t)
	if condition == nil {
		r.Status.Conditions = append(r.Status.Conditions, OrganizationRoleBindingCondition{
			Type:               t,
			Status:             s,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		})
		return
	}
	if condition.Status != s || condition.Reason != reason || condition.Message != message {
		for _, c := range r.Status.Conditions {
			if c.Type == t {
				condition.Status = s
				condition.LastTransitionTime = metav1.Now()
				condition.Reason = reason
				condition.Message = message
			}
		}
	}
}

func init() {
	SchemeBuilder.Register(&OrganizationRoleBinding{}, &OrganizationRoleBindingList{})
}
