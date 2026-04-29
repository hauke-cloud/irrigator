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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	// BridgeRef references the MQTTBridge this device belongs to
	// This field is set by the operator and should not be modified by users
	// +kubebuilder:validation:Required
	BridgeRef BridgeReference `json:"bridgeRef"`

	// IEEEAddr is the unique IEEE address of the device (set by operator)
	// This is the immutable identifier from Zigbee/MQTT discovery
	// +kubebuilder:validation:Required
	IEEEAddr string `json:"ieeeAddr"`

	// SensorType specifies what type of measurements this device provides
	// This determines which database and GORM handler to use for storing data
	// Supported types: "valve", "moisture", "power", "solar", "temperature", "humidity", "pressure"
	// +kubebuilder:validation:Enum=valve;moisture;power;solar;temperature;humidity;pressure
	// +optional
	SensorType string `json:"sensorType,omitempty"`

	// FriendlyName is a user-configurable name for the device
	// +optional
	FriendlyName string `json:"friendlyName,omitempty"`

	// Location describes where the device is physically located
	// +optional
	Location string `json:"location,omitempty"`

	// Room for grouping devices by room
	// +optional
	Room string `json:"room,omitempty"`

	// Disabled indicates whether measurements from this device should be ignored
	// +kubebuilder:default=false
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// MetadataLabels are custom key-value pairs for user-defined metadata
	// +optional
	MetadataLabels map[string]string `json:"metadataLabels,omitempty"`
}

// BridgeReference contains information to reference an MQTTBridge
type BridgeReference struct {
	// Name of the MQTTBridge
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the MQTTBridge. If not specified, uses the same namespace as the Device
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// DeviceStatus defines the observed state of Device.
type DeviceStatus struct {
	// ShortAddr is the short Zigbee address (e.g., "0x4F2E")
	// This is used to map MQTT messages to devices
	// +optional
	ShortAddr string `json:"shortAddr,omitempty"`

	// ModelID is the device model identifier discovered from MQTT
	// +optional
	ModelID string `json:"modelId,omitempty"`

	// Manufacturer is the device manufacturer
	// +optional
	Manufacturer string `json:"manufacturer,omitempty"`

	// DeviceType indicates the type of device (e.g., sensor, actuator, switch)
	// +optional
	DeviceType string `json:"deviceType,omitempty"`

	// Capabilities lists what the device can measure or control
	// Examples: temperature, humidity, pressure, battery, water_leak, occupancy
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// LastSeen is the timestamp when the device last sent data
	// +optional
	LastSeen *metav1.Time `json:"lastSeen,omitempty"`

	// Reachable indicates whether the device is currently reachable
	// +optional
	Reachable *bool `json:"reachable,omitempty"`

	// Available indicates whether the device is currently reachable (deprecated, use Reachable)
	// +optional
	Available bool `json:"available,omitempty"`

	// LastMeasurement contains the most recent measurement data as JSON string
	// +optional
	LastMeasurement string `json:"lastMeasurement,omitempty"`

	// LastPowerState indicates the last known power state for valve/switch devices
	// 0 = OFF, 1 = ON, nil = unknown
	// +optional
	LastPowerState *int `json:"lastPowerState,omitempty"`

	// BatteryPercentage indicates the battery percentage (0-100) if applicable
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	BatteryPercentage *int32 `json:"batteryPercentage,omitempty"`

	// BatteryLevel indicates the battery percentage (0-100) if applicable (deprecated, use BatteryPercentage)
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	BatteryLevel *int32 `json:"batteryLevel,omitempty"`

	// BatteryLastSeenEpoch is the Unix epoch timestamp when battery level was last updated
	// +optional
	BatteryLastSeenEpoch *int64 `json:"batteryLastSeenEpoch,omitempty"`

	// LastSeenSeconds is the number of seconds since the device was last seen
	// +optional
	LastSeenSeconds *int32 `json:"lastSeenSeconds,omitempty"`

	// LastSeenEpoch is the Unix epoch timestamp when the device was last seen
	// +optional
	LastSeenEpoch *int64 `json:"lastSeenEpoch,omitempty"`

	// LinkQuality indicates the signal quality (0-255)
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=255
	LinkQuality *int32 `json:"linkQuality,omitempty"`

	// Conditions represent the current state of the Device resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Short Addr",type=string,JSONPath=`.status.shortAddr`
// +kubebuilder:printcolumn:name="Friendly Name",type=string,JSONPath=`.spec.friendlyName`
// +kubebuilder:printcolumn:name="Sensor Type",type=string,JSONPath=`.spec.sensorType`
// +kubebuilder:printcolumn:name="Battery",type=integer,JSONPath=`.status.batteryPercentage`
// +kubebuilder:printcolumn:name="Link Quality",type=integer,JSONPath=`.status.linkQuality`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.status.deviceType`
// +kubebuilder:printcolumn:name="Available",type=boolean,JSONPath=`.status.available`
// +kubebuilder:printcolumn:name="Battery",type=integer,JSONPath=`.status.batteryLevel`
// +kubebuilder:printcolumn:name="Last Seen",type=date,JSONPath=`.status.lastSeen`

// Device is the Schema for the devices API
type Device struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Device
	// +required
	Spec DeviceSpec `json:"spec"`

	// status defines the observed state of Device
	// +optional
	Status DeviceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
