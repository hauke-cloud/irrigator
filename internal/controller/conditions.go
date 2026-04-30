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
	"fmt"
	"strings"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hauke-cloud/irrigator/internal/alerts"
	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// ConditionEvaluator evaluates execution conditions for schedules
type ConditionEvaluator struct {
	client       client.Client
	alertsClient *alerts.Client
	log          *zap.Logger
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(c client.Client, alertsClient *alerts.Client, log *zap.Logger) *ConditionEvaluator {
	return &ConditionEvaluator{
		client:       c,
		alertsClient: alertsClient,
		log:          log,
	}
}

// EvaluateConditions checks if all execution conditions are met
// Returns (passed bool, message string, error)
func (ce *ConditionEvaluator) EvaluateConditions(ctx context.Context, schedule *iotv1alpha1.Schedule) (bool, string, error) {
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
func (ce *ConditionEvaluator) evaluateCondition(ctx context.Context, schedule *iotv1alpha1.Schedule, condition *iotv1alpha1.ExecutionCondition, index int) (bool, string, error) {
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

	// Evaluate based on condition type
	if condition.Alert != nil {
		return ce.evaluateAlertCondition(ctx, condition)
	}

	// Measurement-based conditions not yet supported without Device.Status.LastMeasurement
	// Would need to query database or other data source
	return false, "Measurement-based conditions not yet supported", fmt.Errorf("measurement conditions require additional implementation")
}

// evaluateAlertCondition checks alert status via the alerts API
func (ce *ConditionEvaluator) evaluateAlertCondition(ctx context.Context, condition *iotv1alpha1.ExecutionCondition) (bool, string, error) {
	checkForAlert := *condition.Alert

	// Query alerts API for this sensor type
	alertDevices, err := ce.alertsClient.GetAlertsBySensorType(ctx, condition.SensorType)
	if err != nil {
		ce.log.Warn("Failed to query alerts API",
			zap.Error(err),
			zap.String("sensorType", condition.SensorType))
		// If API is unavailable, log but don't fail the condition
		// This allows irrigation to continue if alerts service is down
		return true, fmt.Sprintf("Alerts API unavailable: %v", err), nil
	}

	hasAlerts := len(alertDevices) > 0

	if checkForAlert && hasAlerts {
		// Looking for alerts, found some - condition passes
		msg := fmt.Sprintf("%d device(s) with sensorType=%s have alert status", len(alertDevices), condition.SensorType)
		if condition.Message != "" {
			msg = condition.Message
		}
		ce.log.Debug("Alert condition passed",
			zap.String("sensorType", condition.SensorType),
			zap.Int("alertCount", len(alertDevices)))
		return true, msg, nil
	} else if !checkForAlert && hasAlerts {
		// Looking for no alerts, found some - condition fails
		deviceNames := make([]string, 0, len(alertDevices))
		for _, dev := range alertDevices {
			deviceNames = append(deviceNames, dev.DeviceID)
		}
		msg := fmt.Sprintf("Device(s) with sensorType=%s have alert status: %s", condition.SensorType, strings.Join(deviceNames, ", "))
		if condition.Message != "" {
			msg = condition.Message
		}
		ce.log.Debug("Alert condition failed",
			zap.String("sensorType", condition.SensorType),
			zap.Int("alertCount", len(alertDevices)))
		return false, msg, nil
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
