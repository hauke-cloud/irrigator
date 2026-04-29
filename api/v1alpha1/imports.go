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
	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// Re-export types from kubernetes-iot-api for backwards compatibility
// These types are now managed in the shared kubernetes-iot-api repository

type (
	// Device types
	Device          = iotv1alpha1.Device
	DeviceList      = iotv1alpha1.DeviceList
	DeviceSpec      = iotv1alpha1.DeviceSpec
	DeviceStatus    = iotv1alpha1.DeviceStatus
	BridgeReference = iotv1alpha1.BridgeReference
	AlertCondition  = iotv1alpha1.AlertCondition

	// MQTTBridge types
	MQTTBridge        = iotv1alpha1.MQTTBridge
	MQTTBridgeList    = iotv1alpha1.MQTTBridgeList
	MQTTBridgeSpec    = iotv1alpha1.MQTTBridgeSpec
	MQTTBridgeStatus  = iotv1alpha1.MQTTBridgeStatus
	SecretReference   = iotv1alpha1.SecretReference
	TopicSubscription = iotv1alpha1.TopicSubscription
	TLSConfig         = iotv1alpha1.TLSConfig

	// Schedule types
	Schedule           = iotv1alpha1.Schedule
	ScheduleList       = iotv1alpha1.ScheduleList
	ScheduleSpec       = iotv1alpha1.ScheduleSpec
	ScheduleStatus     = iotv1alpha1.ScheduleStatus
	ExecutionCondition = iotv1alpha1.ExecutionCondition
	ComparisonOperator = iotv1alpha1.ComparisonOperator

	// Database types
	Database       = iotv1alpha1.Database
	DatabaseList   = iotv1alpha1.DatabaseList
	DatabaseSpec   = iotv1alpha1.DatabaseSpec
	DatabaseStatus = iotv1alpha1.DatabaseStatus
)

// Re-export constants
const (
	OperatorEqual              = iotv1alpha1.OperatorEqual
	OperatorNotEqual           = iotv1alpha1.OperatorNotEqual
	OperatorGreaterThan        = iotv1alpha1.OperatorGreaterThan
	OperatorGreaterThanOrEqual = iotv1alpha1.OperatorGreaterThanOrEqual
	OperatorLessThan           = iotv1alpha1.OperatorLessThan
	OperatorLessThanOrEqual    = iotv1alpha1.OperatorLessThanOrEqual
)
