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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
)

// ConditionEvaluator evaluates execution conditions for schedules
type ConditionEvaluator struct {
	client client.Client
	log    *zap.Logger
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(c client.Client, log *zap.Logger) *ConditionEvaluator {
	return &ConditionEvaluator{
		client: c,
		log:    log,
	}
}

// EvaluateConditions checks if all execution conditions are met
// Returns (passed bool, message string, error)
func (ce *ConditionEvaluator) EvaluateConditions(ctx context.Context, schedule *mqttv1alpha1.Schedule) (bool, string, error) {
	if len(schedule.Spec.ExecutionConditions) == 0 {
		return true, "No conditions configured", nil
	}

	ce.log.Debug("Evaluating execution conditions",
		zap.String("schedule", schedule.Name),
		zap.Int("conditionCount", len(schedule.Spec.ExecutionConditions)))

	var failedConditions []string

	for i, condition := range schedule.Spec.ExecutionConditions {
		passed, msg, err := ce.evaluateCondition(ctx, schedule, &condition, i)
		if err != nil {
			return false, fmt.Sprintf("Error evaluating condition %d: %s", i+1, err.Error()), err
		}
		if !passed {
			failedConditions = append(failedConditions, msg)
		}
	}

	if len(failedConditions) > 0 {
		message := fmt.Sprintf("Conditions failed: %s", strings.Join(failedConditions, "; "))
		ce.log.Info("Execution conditions not met",
			zap.String("schedule", schedule.Name),
			zap.String("message", message))
		return false, message, nil
	}

	ce.log.Info("All execution conditions passed", zap.String("schedule", schedule.Name))
	return true, "All conditions passed", nil
}

// evaluateCondition evaluates a single execution condition
func (ce *ConditionEvaluator) evaluateCondition(ctx context.Context, schedule *mqttv1alpha1.Schedule, condition *mqttv1alpha1.ExecutionCondition, index int) (bool, string, error) {
	// Validate condition configuration
	if condition.Alert != nil && (condition.Measurement != "" || condition.Operator != "" || condition.Value != "") {
		return false, "Alert and Measurement/Operator/Value cannot be used together", fmt.Errorf("invalid condition configuration at index %d", index)
	}

	if condition.Alert == nil && condition.Measurement == "" {
		return false, "Either Alert or Measurement must be specified", fmt.Errorf("invalid condition configuration at index %d", index)
	}

	if condition.Measurement != "" && (condition.Operator == "" || condition.Value == "") {
		return false, "Operator and Value are required when Measurement is specified", fmt.Errorf("invalid condition configuration at index %d", index)
	}

	// Find all devices matching the sensor type
	devices, err := ce.findDevicesBySensorType(ctx, schedule.Namespace, condition.SensorType)
	if err != nil {
		return false, fmt.Sprintf("Failed to find devices with sensorType=%s", condition.SensorType), err
	}

	if len(devices) == 0 {
		msg := fmt.Sprintf("No devices found with sensorType=%s", condition.SensorType)
		if condition.Message != "" {
			msg = condition.Message
		}
		ce.log.Warn("No devices found for condition",
			zap.String("sensorType", condition.SensorType),
			zap.Int("index", index))
		return false, msg, nil
	}

	ce.log.Debug("Found devices for condition",
		zap.String("sensorType", condition.SensorType),
		zap.Int("deviceCount", len(devices)))

	// Evaluate based on condition type
	if condition.Alert != nil {
		return ce.evaluateAlertCondition(devices, condition)
	}

	return ce.evaluateMeasurementCondition(devices, condition)
}

// evaluateAlertCondition checks alert status across devices
func (ce *ConditionEvaluator) evaluateAlertCondition(devices []mqttv1alpha1.Device, condition *mqttv1alpha1.ExecutionCondition) (bool, string, error) {
	checkForAlert := *condition.Alert

	for _, device := range devices {
		hasAlert := ce.deviceHasAlert(&device)

		if checkForAlert && hasAlert {
			// Looking for alerts, found one - condition passes
			msg := fmt.Sprintf("Device %s has alert status", device.Name)
			if condition.Message != "" {
				msg = condition.Message
			}
			ce.log.Debug("Alert condition passed",
				zap.String("device", device.Name),
				zap.Bool("hasAlert", hasAlert))
			return true, msg, nil
		} else if !checkForAlert && hasAlert {
			// Looking for no alerts, found one - condition fails
			msg := fmt.Sprintf("Device %s has alert status", device.Name)
			if condition.Message != "" {
				msg = condition.Message
			}
			ce.log.Debug("Alert condition failed",
				zap.String("device", device.Name),
				zap.Bool("hasAlert", hasAlert))
			return false, msg, nil
		}
	}

	// If checking for alerts and none found, condition fails
	if checkForAlert {
		msg := fmt.Sprintf("No devices with sensorType=%s have alert status", condition.SensorType)
		if condition.Message != "" {
			msg = condition.Message
		}
		return false, msg, nil
	}

	// If checking for no alerts and none found, condition passes
	msg := fmt.Sprintf("No alerts for sensorType=%s", condition.SensorType)
	return true, msg, nil
}

// evaluateMeasurementCondition checks measurement values across devices
func (ce *ConditionEvaluator) evaluateMeasurementCondition(devices []mqttv1alpha1.Device, condition *mqttv1alpha1.ExecutionCondition) (bool, string, error) {
	allPassed := true
	var failedDevices []string

	for _, device := range devices {
		passed, err := ce.checkDeviceMeasurement(&device, condition)
		if err != nil {
			ce.log.Warn("Failed to check device measurement",
				zap.String("device", device.Name),
				zap.Error(err))
			continue
		}

		if !passed {
			allPassed = false
			failedDevices = append(failedDevices, device.Name)
		}
	}

	if !allPassed {
		msg := fmt.Sprintf("Measurement condition failed for devices: %s", strings.Join(failedDevices, ", "))
		if condition.Message != "" {
			msg = condition.Message
		}
		return false, msg, nil
	}

	msg := fmt.Sprintf("Measurement condition passed for all %d devices", len(devices))
	return true, msg, nil
}

// deviceHasAlert checks if a device has an alert status
func (ce *ConditionEvaluator) deviceHasAlert(device *mqttv1alpha1.Device) bool {
	// Check if device has any conditions with status "True" that indicate an alert
	for _, cond := range device.Status.Conditions {
		if cond.Status == "True" && (cond.Type == "Alert" || cond.Type == "Warning" || cond.Type == "Error") {
			return true
		}
	}

	// Check LastMeasurement for alert indicators
	if device.Status.LastMeasurement != "" {
		var measurement map[string]any
		if err := json.Unmarshal([]byte(device.Status.LastMeasurement), &measurement); err == nil {
			if alert, ok := measurement["alert"].(bool); ok && alert {
				return true
			}
			if waterLeak, ok := measurement["water_leak"].(bool); ok && waterLeak {
				return true
			}
		}
	}

	return false
}

// checkDeviceMeasurement checks if a device's measurement meets the condition
func (ce *ConditionEvaluator) checkDeviceMeasurement(device *mqttv1alpha1.Device, condition *mqttv1alpha1.ExecutionCondition) (bool, error) {
	if device.Status.LastMeasurement == "" {
		return false, fmt.Errorf("no measurement data available")
	}

	var measurement map[string]any
	if err := json.Unmarshal([]byte(device.Status.LastMeasurement), &measurement); err != nil {
		return false, fmt.Errorf("failed to parse measurement: %w", err)
	}

	value, ok := measurement[condition.Measurement]
	if !ok {
		return false, fmt.Errorf("measurement field '%s' not found", condition.Measurement)
	}

	return ce.compareValues(value, condition.Operator, condition.Value)
}

// compareValues compares a measurement value against a condition value using the specified operator
func (ce *ConditionEvaluator) compareValues(actualValue any, operator mqttv1alpha1.ComparisonOperator, expectedValueStr string) (bool, error) {
	// Try to compare as boolean
	if boolVal, ok := actualValue.(bool); ok {
		expectedBool, err := strconv.ParseBool(expectedValueStr)
		if err != nil {
			return false, fmt.Errorf("cannot compare boolean with non-boolean value")
		}
		return ce.compareBool(boolVal, operator, expectedBool)
	}

	// Try to compare as number
	var actualFloat float64
	switch v := actualValue.(type) {
	case float64:
		actualFloat = v
	case float32:
		actualFloat = float64(v)
	case int:
		actualFloat = float64(v)
	case int32:
		actualFloat = float64(v)
	case int64:
		actualFloat = float64(v)
	default:
		// Try string comparison as fallback
		actualStr := fmt.Sprintf("%v", actualValue)
		return ce.compareString(actualStr, operator, expectedValueStr)
	}

	expectedFloat, err := strconv.ParseFloat(expectedValueStr, 64)
	if err != nil {
		return false, fmt.Errorf("cannot parse expected value as number: %w", err)
	}

	return ce.compareFloat(actualFloat, operator, expectedFloat)
}

// compareBool compares boolean values
func (ce *ConditionEvaluator) compareBool(actual bool, operator mqttv1alpha1.ComparisonOperator, expected bool) (bool, error) {
	switch operator {
	case mqttv1alpha1.OperatorEqual:
		return actual == expected, nil
	case mqttv1alpha1.OperatorNotEqual:
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for boolean comparison", operator)
	}
}

// compareFloat compares numeric values
func (ce *ConditionEvaluator) compareFloat(actual float64, operator mqttv1alpha1.ComparisonOperator, expected float64) (bool, error) {
	switch operator {
	case mqttv1alpha1.OperatorEqual:
		return actual == expected, nil
	case mqttv1alpha1.OperatorNotEqual:
		return actual != expected, nil
	case mqttv1alpha1.OperatorGreaterThan:
		return actual > expected, nil
	case mqttv1alpha1.OperatorGreaterThanOrEqual:
		return actual >= expected, nil
	case mqttv1alpha1.OperatorLessThan:
		return actual < expected, nil
	case mqttv1alpha1.OperatorLessThanOrEqual:
		return actual <= expected, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

// compareString compares string values
func (ce *ConditionEvaluator) compareString(actual string, operator mqttv1alpha1.ComparisonOperator, expected string) (bool, error) {
	switch operator {
	case mqttv1alpha1.OperatorEqual:
		return actual == expected, nil
	case mqttv1alpha1.OperatorNotEqual:
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for string comparison", operator)
	}
}

// findDevicesBySensorType finds all devices with a specific sensor type in the namespace
func (ce *ConditionEvaluator) findDevicesBySensorType(ctx context.Context, namespace string, sensorType string) ([]mqttv1alpha1.Device, error) {
	deviceList := &mqttv1alpha1.DeviceList{}
	err := ce.client.List(ctx, deviceList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var matchingDevices []mqttv1alpha1.Device
	for _, device := range deviceList.Items {
		if device.Spec.SensorType == sensorType {
			matchingDevices = append(matchingDevices, device)
		}
	}

	return matchingDevices, nil
}
