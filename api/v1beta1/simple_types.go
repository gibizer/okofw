/*
Copyright 2022.

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
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SimpleSpec defines the desired state of Simple
type SimpleSpec struct {
	// +kubebuilder:validation:Required
	// Dividend
	Dividend int `json:"dividend"`

	// +kubebuilder:validation:Required
	// Divisor
	Divisor int `json:"divisor"`
}

// SimpleStatus defines the observed state of Simple
type SimpleStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Quotient
	Quotient *int `json:"quotient,omitempty" optional:"true"`

	// Remainder
	Remainder *int `json:"remainder,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Simple is the Schema for the simples API
type Simple struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimpleSpec   `json:"spec,omitempty"`
	Status SimpleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SimpleList contains a list of Simple
type SimpleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Simple `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Simple{}, &SimpleList{})
}

func (i Simple) GetConditions() condition.Conditions {
	return i.Status.Conditions
}

func (i *Simple) SetConditions(conditions condition.Conditions) {
	i.Status.Conditions = conditions
}
