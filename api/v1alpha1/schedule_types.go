/*
Copyright 2026 hauke.cloud.

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

// ScheduleSpec defines the desired state of Schedule
type ScheduleSpec struct {
	// DeviceName is the name of the Device CR to control
	// The device must have sensorType "valve" to be used for irrigation
	// +optional
	DeviceName string `json:"deviceName,omitempty"`

	// DeviceFriendlyName is the friendly name of the device to control
	// Alternative to DeviceName - will look up device by spec.friendlyName
	// +optional
	DeviceFriendlyName string `json:"deviceFriendlyName,omitempty"`

	// DeviceIEEEAddr is the IEEE address of the device to control
	// Alternative to DeviceName - will look up device by spec.ieeeAddr
	// +optional
	DeviceIEEEAddr string `json:"deviceIEEEAddr,omitempty"`

	// DeviceShortAddr is the short Zigbee address of the device to control
	// Alternative to DeviceName - will look up device by status.shortAddr
	// +optional
	DeviceShortAddr string `json:"deviceShortAddr,omitempty"`

	// CronExpression defines when the irrigation should run
	// Standard cron format: "minute hour day month weekday"
	// Examples:
	//   "0 6 * * *" - Every day at 6:00 AM
	//   "0 18 * * 1,3,5" - Monday, Wednesday, Friday at 6:00 PM
	//   "*/30 * * * *" - Every 30 minutes
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	CronExpression string `json:"cronExpression"`

	// DurationSeconds defines how long the valve should remain open (in seconds)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=86400
	DurationSeconds int32 `json:"durationSeconds"`

	// Enabled indicates whether this schedule is active
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// TimeZone for the cron schedule (e.g., "Europe/Berlin", "UTC")
	// If not specified, uses UTC
	// +kubebuilder:default="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// DryRun enables dry-run mode where execution plans are logged but no MQTT commands are sent
	// Useful for testing schedules without actually controlling valves
	// +kubebuilder:default=false
	// +optional
	DryRun bool `json:"dryRun,omitempty"`
}

// ScheduleStatus defines the observed state of Schedule
type ScheduleStatus struct {
	// ResolvedDeviceName is the name of the Device CR that was found
	// This shows which device is actually being controlled
	// +optional
	ResolvedDeviceName string `json:"resolvedDeviceName,omitempty"`

	// ValvePowerState indicates the last known power state from telemetry
	// 0 = OFF, 1 = ON, nil = unknown
	// +optional
	ValvePowerState *int `json:"valvePowerState,omitempty"`

	// LastScheduledTime is when the irrigation was last scheduled to run
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// LastExecutionTime is when the irrigation actually started
	// +optional
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// LastCompletionTime is when the irrigation finished
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty"`

	// NextScheduledTime is when the irrigation is next scheduled to run
	// +optional
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`

	// Active indicates if irrigation is currently running
	// +optional
	Active bool `json:"active,omitempty"`

	// LastStatus describes the result of the last execution
	// +optional
	LastStatus string `json:"lastStatus,omitempty"`

	// Message provides additional information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the schedule's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sched
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.status.resolvedDeviceName`,priority=1
// +kubebuilder:printcolumn:name="Cron",type=string,JSONPath=`.spec.cronExpression`
// +kubebuilder:printcolumn:name="Duration",type=integer,JSONPath=`.spec.durationSeconds`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
// +kubebuilder:printcolumn:name="Last Run",type=string,JSONPath=`.status.lastExecutionTime`
// +kubebuilder:printcolumn:name="Next Run",type=string,JSONPath=`.status.nextScheduledTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Schedule is the Schema for the schedules API
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduleSpec   `json:"spec,omitempty"`
	Status ScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScheduleList contains a list of Schedule
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Schedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schedule{}, &ScheduleList{})
}
