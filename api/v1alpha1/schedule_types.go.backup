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

// ComparisonOperator defines operators for comparing values in conditions
// +kubebuilder:validation:Enum=eq;ne;gt;ge;lt;le
type ComparisonOperator string

const (
	OperatorEqual              ComparisonOperator = "eq" // Equal
	OperatorNotEqual           ComparisonOperator = "ne" // Not Equal
	OperatorGreaterThan        ComparisonOperator = "gt" // Greater Than
	OperatorGreaterThanOrEqual ComparisonOperator = "ge" // Greater Than or Equal
	OperatorLessThan           ComparisonOperator = "lt" // Less Than
	OperatorLessThanOrEqual    ComparisonOperator = "le" // Less Than or Equal
)

// ExecutionCondition defines a condition that must be met before executing a schedule
type ExecutionCondition struct {
	// SensorType is the type of sensor to check (e.g., "water_level", "moisture", "temperature")
	// This matches the Device.Spec.SensorType field
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SensorType string `json:"sensorType"`

	// Alert indicates whether to check for alert status on matching devices
	// If true, condition passes only if no matching devices have an alert
	// If false, condition passes only if all matching devices have an alert
	// Cannot be used together with Measurement
	// +optional
	Alert *bool `json:"alert,omitempty"`

	// Measurement is the name of the measurement field to check in device telemetry
	// Example fields: "temperature", "humidity", "battery", "moisture"
	// Cannot be used together with Alert
	// +optional
	Measurement string `json:"measurement,omitempty"`

	// Operator defines how to compare the measurement value
	// Required when Measurement is specified
	// +optional
	Operator ComparisonOperator `json:"operator,omitempty"`

	// Value is the value to compare against
	// Can be a number (e.g., "25.5") or boolean (e.g., "true")
	// Required when Measurement is specified
	// +optional
	Value string `json:"value,omitempty"`

	// Message is a custom message to include in status when this condition fails
	// +optional
	Message string `json:"message,omitempty"`
}

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

	// ExecutionConditions are conditions that must be met before executing the schedule
	// All conditions must pass for the schedule to execute
	// Example: Check that all water_level sensors are not alerting
	// +optional
	ExecutionConditions []ExecutionCondition `json:"executionConditions,omitempty"`
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

	// NextScheduledTimeFormatted is the next scheduled time formatted in the configured timezone
	// This is a human-readable string for display purposes
	// +optional
	NextScheduledTimeFormatted string `json:"nextScheduledTimeFormatted,omitempty"`

	// LastExecutionTimeFormatted is the last execution time formatted in the configured timezone
	// This is a human-readable string for display purposes
	// +optional
	LastExecutionTimeFormatted string `json:"lastExecutionTimeFormatted,omitempty"`

	// Active indicates if irrigation is currently running
	// +optional
	Active bool `json:"active,omitempty"`

	// LastStatus describes the result of the last execution
	// +optional
	LastStatus string `json:"lastStatus,omitempty"`

	// Message provides additional information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// ConditionsLastChecked is when execution conditions were last evaluated
	// +optional
	ConditionsLastChecked *metav1.Time `json:"conditionsLastChecked,omitempty"`

	// ConditionsPassed indicates whether all execution conditions passed on last check
	// +optional
	ConditionsPassed *bool `json:"conditionsPassed,omitempty"`

	// ConditionsMessage provides details about condition evaluation
	// +optional
	ConditionsMessage string `json:"conditionsMessage,omitempty"`

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
// +kubebuilder:printcolumn:name="Last Run",type=string,JSONPath=`.status.lastExecutionTimeFormatted`
// +kubebuilder:printcolumn:name="Next Run",type=string,JSONPath=`.status.nextScheduledTimeFormatted`
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
