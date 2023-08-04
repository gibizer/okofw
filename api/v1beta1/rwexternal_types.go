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

const (
	OutputReadyCondition    condition.Type = "OutputReady"
	OutputReadyInitMessage  string         = "Output not ready"
	OutputReadyErrorMessage string         = "Output generation failed: %s"
	OutputReadyReadyMessage string         = "Output ready"
)

// RWExternalSpec defines the desired state of RWExternal
type RWExternalSpec struct {
	// +kubebuilder:validation:Required
	// InputSecret defines the name of the Secret to process
	InputSecret string `json:"inputSecret"`
}

// RWExternalStatus defines the observed state of RWExternal
type RWExternalStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// OutputSecret provides the name of the Secret where the result of the
	// processing stored
	OutputSecret *string `json:"outputSecret,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RWExternal is the Schema for the rwexternals API
type RWExternal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RWExternalSpec   `json:"spec,omitempty"`
	Status RWExternalStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RWExternalList contains a list of RWExternal
type RWExternalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RWExternal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RWExternal{}, &RWExternalList{})
}

func (i RWExternal) GetConditions() condition.Conditions {
	return i.Status.Conditions
}

func (i *RWExternal) SetConditions(conditions condition.Conditions) {
	i.Status.Conditions = conditions
}
