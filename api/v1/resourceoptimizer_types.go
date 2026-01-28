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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceOptimizerSpec defines the desired state of ResourceOptimizer
type ResourceOptimizerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	TargetRef TargetRef `json:"targetRef"`
	Policy    Policy    `json:"policy"`
}

type TargetRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Policy struct {
	Cpu    CPUPolicy    `json:"cpu"`
	Memory MemoryPolicy `json:"memory"`
}

type CPUPolicy struct {
	// +kubebuilder:validation:Pattern=`^([0-9]+m|[0-9]+)$`
	Min string `json:"min"`

	// +kubebuilder:validation:Pattern=`^([0-9]+m|[0-9]+)$`
	Max string `json:"max"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=70
	TargetUtilization int32 `json:"targetUtilization"`
}

type MemoryPolicy struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=20
	BufferPercent int32 `json:"bufferPercent"`
}

// ResourceOptimizerStatus defines the observed state of ResourceOptimizer.
type ResourceOptimizerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ResourceOptimizer resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// currentRecommendation holds the latest resource optimization recommendation
	// +optional
	CurrentRecommendation *ResourceRecommendation `json:"currentRecommendation,omitempty"`

	// lastOptimized indicates when the workload was last optimized
	// +optional
	LastOptimized *metav1.Time `json:"lastOptimized,omitempty"`
}

type ResourceRecommendation struct {
	// CPU resource recommendations
	CPU CPURecommendation `json:"cpu"`

	// Memory resource recommendations
	Memory MemoryRecommendation `json:"memory"`

	// Confidence level of the recommendation (0 to 100 percent)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Confidence int32 `json:"confidence"`

	// Reason for the recommendation
	Reason string `json:"reason"`

	// Timestamp when recommendation was generated
	GeneratedAt metav1.Time `json:"generatedAt"`
}

type CPURecommendation struct {
	// Recommended CPU request
	Request string `json:"request"`

	// Recommended CPU limit
	Limit string `json:"limit"`
}

type MemoryRecommendation struct {
	// Recommended memory request
	Request string `json:"request"`

	// Recommended memory limit
	Limit string `json:"limit"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ResourceOptimizer is the Schema for the resourceoptimizers API
type ResourceOptimizer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ResourceOptimizer
	// +required
	Spec ResourceOptimizerSpec `json:"spec"`

	// status defines the observed state of ResourceOptimizer
	// +optional
	Status ResourceOptimizerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ResourceOptimizerList contains a list of ResourceOptimizer
type ResourceOptimizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ResourceOptimizer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceOptimizer{}, &ResourceOptimizerList{})
}
