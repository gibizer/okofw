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

// ServiceWithDBSpec defines the desired state of ServiceWithDB
type ServiceWithDBSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// DatabaseInstance is the name of the MariaDB CR to select the DB
	// Service instance used
	DatabaseInstance string `json:"databaseInstance"`

	// +kubebuilder:validation:Required
	// The service specific Container Image URL
	ContainerImage string `json:"containerImage"`

	// +kubebuilder:validation:Optional
	// CustomServiceConfig
	CustomServiceConfig string `json:"customServiceConfig"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas of the service to run
	Replicas int32 `json:"replicas"`
}

// ServiceWithDBStatus defines the observed state of ServiceWithDB
type ServiceWithDBStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServiceWithDB is the Schema for the servicewithdbs API
type ServiceWithDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceWithDBSpec   `json:"spec,omitempty"`
	Status ServiceWithDBStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceWithDBList contains a list of ServiceWithDB
type ServiceWithDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceWithDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceWithDB{}, &ServiceWithDBList{})
}
