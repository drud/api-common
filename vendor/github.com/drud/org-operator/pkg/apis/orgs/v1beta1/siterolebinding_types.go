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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SiteRoleBindingSpec defines the desired state of SiteRoleBinding
type SiteRoleBindingSpec struct {

	//TODO we should support both specifiying individual sites and label based selectors
	Sites []SiteRef `json:"sites,omitempty"`

	SiteAdmins     []string `json:"siteAdmins,omitempty"`
	SiteDevelopers []string `json:"siteDevelopers,omitempty"`
	SiteReaders    []string `json:"siteReaders,omitempty"`
}

// SiteRoleBindingStatus defines the observed state of SiteRoleBinding
type SiteRoleBindingStatus struct {
	ActiveAdmins     int `json:"activeAdmins"`
	ActiveDevelopers int `json:"activeDevelopers"`
	ActiveReaders    int `json:"activeReaders"`
}

// SiteRef defines the sites defining access
type SiteRef struct {
	Kind string `json:"status"`
	Name string `json:"name"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteRoleBinding is the Schema for the siterolebindings API
// +k8s:openapi-gen=true
type SiteRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SiteRoleBindingSpec   `json:"spec,omitempty"`
	Status SiteRoleBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteRoleBindingList contains a list of SiteRoleBinding
type SiteRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SiteRoleBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SiteRoleBinding{}, &SiteRoleBindingList{})
}
