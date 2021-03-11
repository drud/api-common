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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OrganizationSpec defines the desired state of Organization
type OrganizationSpec struct {
	Owner                Owner           `json:"owner"`
	CompanyName          string          `json:"companyName"`
	GithubOrg            string          `json:"githubOrg"`
	LetsEncrypt          LetsEncryptSpec `json:"letsEncrypt,omitempty"`
	ProvisionMySQLServer bool            `json:"provisionMySQLServer,omitempty"`
}

// LetsEncryptSpec defines the desired state of LetsEncryptAccount
type LetsEncryptSpec struct {
	AccountDisabled bool   `json:"accountDisabled,omitempty"`
	AccountEmail    string `json:"email,omitempty"`
}

// Owner refers to the super admin of the organization. This user will always be a OrgAdmin
type Owner struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Email     string `json:"email"`
}

// OrganizationStatus defines the observed state of Organization
type OrganizationStatus struct {
	NamespaceRef  string                  `json:"namespaceRef"`
	OrgRolesRef   string                  `json:"orgRolesRef"`
	CertIssuerRef string                  `json:"certIssuerRef"`
	Conditions    []OrganizationCondition `json:"conditions,omitempty"`
}

// OrgConditionType tracks the observed status of Organization & the it's dependant resources
type OrgConditionType string

const (
	// OrgNamespaceProv tracks if the Organization's NS is Provisioned
	OrgNamespaceProv OrgConditionType = "NamespaceProvisioned"

	// OrgRolesProv tracks if the Organization's RoleBindings are Provisioned
	OrgRolesProv OrgConditionType = "OrgRolesProvisioned"

	// OrgCertIssuerProv tracks if the Organization's LetsEncrypt Issuer is Provisioned
	OrgCertIssuerProv OrgConditionType = "OrgCertIssuerProvisioned"
)

// OrganizationCondition describes status conditions for resources handled by an Organization
type OrganizationCondition struct {
	// Type of Org condition.
	Type OrgConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:

// Organization is the Schema for the organizations API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Owner",type="string",JSONPath=".spec.owner.email",description="The username of the organization owner"
// +kubebuilder:printcolumn:name="Company",type="string",JSONPath=".spec.companyName",description="The Company Name"
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".status.namespaceRef",description="The namespace that all resources for this org belong in"
// +kubebuilder:subresource:status
// +kubebuilder:resource:path="organizations",shortName="org"
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec,omitempty"`
	Status OrganizationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
