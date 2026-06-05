/*
Copyright 2025 Hauke Mettendorf.

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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=irrsched
// +kubebuilder:printcolumn:name="Valve",type="string",JSONPath=".spec.valveRef.name"
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.cronExpression"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Last Run",type="date",JSONPath=".status.lastExecutionTime"
// +kubebuilder:printcolumn:name="Next Run",type="date",JSONPath=".status.nextExecutionTime"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type IrrigationSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IrrigationScheduleSpec   `json:"spec"`
	Status IrrigationScheduleStatus `json:"status,omitempty"`
}

type IrrigationScheduleSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	CronExpression string `json:"cronExpression"`

	// +kubebuilder:validation:Required
	ValveRef ValveRef `json:"valveRef"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=86400
	DurationSeconds int32 `json:"durationSeconds"`

	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +kubebuilder:default=false
	DryRun bool `json:"dryRun"`

	// +kubebuilder:default="Europe/Berlin"
	TimeZone string `json:"timeZone,omitempty"`
}

type ValveRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type IrrigationScheduleStatus struct {
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`
	NextExecutionTime *metav1.Time `json:"nextExecutionTime,omitempty"`

	// +kubebuilder:validation:Enum=success;failed;dry-run
	LastExecutionResult string `json:"lastExecutionResult,omitempty"`

	// +kubebuilder:validation:Minimum=0
	ExecutionCount int64 `json:"executionCount,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
type IrrigationScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IrrigationSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IrrigationSchedule{}, &IrrigationScheduleList{})
}
